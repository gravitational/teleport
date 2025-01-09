/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web/terminal"
)

// podHandler connects Kube exec session and web-based terminal via a websocket.
type podHandler struct {
	teleportCluster     string
	configTLSServerName string
	configServerAddr    string
	req                 *PodExecRequest
	sess                session.Session
	sctx                *SessionContext
	ws                  *websocket.Conn
	keepAliveInterval   time.Duration
	logger              *slog.Logger
	userClient          authclient.ClientI
	localCA             types.CertAuthority

	// closedByClient indicates if the websocket connection was closed by the
	// user (closing the browser tab, exiting the session, etc).
	closedByClient atomic.Bool
}

// PodExecRequest describes a request to create a web-based terminal
// to exec into a pod.
type PodExecRequest struct {
	// KubeCluster specifies what Kubernetes cluster to connect to.
	KubeCluster string `json:"kubeCluster"`
	// Namespace is the namespace of the target pod
	Namespace string `json:"namespace"`
	// Pod is the target pod to connect to.
	Pod string `json:"pod"`
	// Container is a container within the target pod to connect to, optional.
	Container string `json:"container"`
	// Command is the command to run at the target pod.
	Command string `json:"command"`
	// IsInteractive specifies whether exec request should have interactive TTY.
	IsInteractive bool `json:"isInteractive"`
	// Term is the initial PTY size.
	Term session.TerminalParams `json:"term"`
}

func (r *PodExecRequest) Validate() error {
	if r.KubeCluster == "" {
		return trace.BadParameter("missing parameter KubeCluster")
	}
	if r.Namespace == "" {
		return trace.BadParameter("missing parameter Namespace")
	}
	if r.Pod == "" {
		return trace.BadParameter("missing parameter Pod")
	}
	if r.Command == "" {
		return trace.BadParameter("missing parameter Command")
	}
	if len(r.Namespace) > 63 {
		return trace.BadParameter("Namespace is too long, maximum length is 63 characters")
	}
	if len(r.Pod) > 63 {
		return trace.BadParameter("Pod is too long, maximum length is 63 characters")
	}
	if len(r.Container) > 63 {
		return trace.BadParameter("Container is too long, maximum length is 63 characters")
	}
	if len(r.Command) > 10000 {
		return trace.BadParameter("Command is too long, maximum length is 10000 characters")
	}

	return nil
}

// ServeHTTP sends session metadata to web UI to signal beginning of the session, then
// handles Kube exec request and connects it to web based terminal input/output.
func (p *podHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	// Allow closing websocket if the user logs out before exiting
	// the session.
	p.sctx.AddClosers(p)
	defer p.sctx.RemoveCloser(p)

	sessionMetadataResponse, err := json.Marshal(siteSessionGenerateResponse{Session: p.sess})
	if err != nil {
		p.logger.ErrorContext(r.Context(), "failed marshaling session data", "error", err)
		if err := p.sendErrorMessage(err); err != nil {
			p.logger.ErrorContext(r.Context(), "failed to send error message to client", "error", err)
		}
		return
	}

	envelope := &terminal.Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketSessionMetadata,
		Payload: string(sessionMetadataResponse),
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		p.logger.ErrorContext(r.Context(), "failed marshaling message envelope", "error", err)
		if err := p.sendErrorMessage(err); err != nil {
			p.logger.ErrorContext(r.Context(), "failed to send error message to client", "error", err)
		}

		return
	}

	err = p.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	if err != nil {
		p.logger.ErrorContext(r.Context(), "failed write session data message", "error", err)
		if err := p.sendErrorMessage(err); err != nil {
			p.logger.ErrorContext(r.Context(), "failed to send error message to client", "error", err)
		}

		return
	}

	if err := p.handler(r); err != nil {
		p.logger.ErrorContext(r.Context(), "handling kube session unexpectedly terminated", "error", err)
		if err := p.sendErrorMessage(err); err != nil {
			p.logger.ErrorContext(r.Context(), "failed to send error message to client", "error", err)
		}
	}
}

func (p *podHandler) Close() error {
	return trace.Wrap(p.ws.Close())
}

func (p *podHandler) sendErrorMessage(err error) error {
	if p.closedByClient.Load() {
		return nil
	}

	envelope := &terminal.Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketError,
		Payload: err.Error(),
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return trace.Wrap(err, "creating envelope payload")
	}
	if err := p.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes); err != nil {
		return trace.Wrap(err, "writing error message")
	}

	return nil
}

func (p *podHandler) handler(r *http.Request) error {
	p.logger.DebugContext(r.Context(), "Creating websocket stream for a kube exec request")

	// Create a context for signaling when the terminal session is over and
	// link it first with the trace context from the request context
	tctx := oteltrace.ContextWithRemoteSpanContext(context.Background(), oteltrace.SpanContextFromContext(r.Context()))
	ctx, cancel := context.WithCancel(tctx)
	defer cancel()

	defaultCloseHandler := p.ws.CloseHandler()
	p.ws.SetCloseHandler(func(code int, text string) error {
		p.closedByClient.Store(true)
		p.logger.DebugContext(r.Context(), "websocket connection was closed by client")

		cancel()
		// Call the default close handler if one was set.
		if defaultCloseHandler != nil {
			err := defaultCloseHandler(code, text)
			return trace.NewAggregate(err, p.Close())
		}

		return trace.Wrap(p.Close())
	})

	// Start sending ping frames through websocket to the client.
	go startWSPingLoop(r.Context(), p.ws, p.keepAliveInterval, p.logger, p.Close)

	pk, err := keys.ParsePrivateKey(p.sctx.cfg.Session.GetTLSPriv())
	if err != nil {
		return trace.Wrap(err, "failed getting user private key from the session")
	}

	privateKeyPEM, err := pk.SoftwarePrivateKeyPEM()
	if err != nil {
		return trace.Wrap(err, "failed getting software private key")
	}
	publicKeyPEM, err := keys.MarshalPublicKey(pk.Public())
	if err != nil {
		return trace.Wrap(err, "failed to marshal public key")
	}

	resizeQueue := newTermSizeQueue(ctx, remotecommand.TerminalSize{
		Width:  p.req.Term.Winsize().Width,
		Height: p.req.Term.Winsize().Height,
	})
	stream := terminal.NewStream(ctx, terminal.StreamConfig{
		WS:     p.ws,
		Logger: p.logger,
		Handlers: map[string]terminal.WSHandlerFunc{
			defaults.WebsocketResize: p.handleResize(resizeQueue),
		},
	})

	certsReq := clientproto.UserCertsRequest{
		TLSPublicKey:      publicKeyPEM,
		Username:          p.sctx.GetUser(),
		Expires:           p.sctx.cfg.Session.GetExpiryTime(),
		Format:            constants.CertificateFormatStandard,
		RouteToCluster:    p.teleportCluster,
		KubernetesCluster: p.req.KubeCluster,
		Usage:             clientproto.UserCertsRequest_Kubernetes,
	}

	_, certs, err := client.PerformSessionMFACeremony(ctx, client.PerformSessionMFACeremonyParams{
		CurrentAuthClient: p.userClient,
		RootAuthClient:    p.sctx.cfg.RootClient,
		MFACeremony:       newMFACeremony(stream.WSStream, p.sctx.cfg.RootClient.CreateAuthenticateChallenge),
		MFAAgainstRoot:    p.sctx.cfg.RootClusterName == p.teleportCluster,
		MFARequiredReq: &clientproto.IsMFARequiredRequest{
			Target: &clientproto.IsMFARequiredRequest_KubernetesCluster{KubernetesCluster: p.req.KubeCluster},
		},
		CertsReq: &certsReq,
	})
	if err != nil && !errors.Is(err, services.ErrSessionMFANotRequired) {
		return trace.Wrap(err, "failed performing mfa ceremony")
	}

	if certs == nil {
		certs, err = p.sctx.cfg.RootClient.GenerateUserCerts(ctx, certsReq)
		if err != nil {
			return trace.Wrap(err, "failed issuing user certs")
		}
	}

	restConfig, err := createKubeRestConfig(p.configServerAddr, p.configTLSServerName, p.localCA, certs.TLS, privateKeyPEM)
	if err != nil {
		return trace.Wrap(err, "failed creating Kubernetes rest config")
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return trace.Wrap(err, "failed creating Kubernetes client")
	}

	kubeReq := kubeClient.CoreV1().RESTClient().Post().Resource("pods").Name(p.req.Pod).
		Namespace(p.req.Namespace).SubResource("exec")
	option := &v1.PodExecOptions{
		Container: p.req.Container,
		Command:   strings.Split(p.req.Command, " "),
		TTY:       p.req.IsInteractive,
		Stdin:     p.req.IsInteractive,
		Stdout:    true,
		Stderr:    !p.req.IsInteractive,
	}

	kubeReq.VersionedParams(option, scheme.ParameterCodec)
	p.logger.DebugContext(ctx, "Web kube exec request created", "url", logutils.StringerAttr(kubeReq.URL()))

	wsExec, err := remotecommand.NewWebSocketExecutor(restConfig, "POST", kubeReq.URL().String())
	if err != nil {
		return trace.Wrap(err, "failed creating websocket executor")
	}

	streamOpts := remotecommand.StreamOptions{
		Stdin:             stream,
		Stdout:            stream,
		Tty:               p.req.IsInteractive,
		TerminalSizeQueue: resizeQueue,
	}
	if !p.req.IsInteractive {
		streamOpts.Stderr = stderrWriter{stream: stream}
	}

	if err := wsExec.StreamWithContext(ctx, streamOpts); err != nil {
		return trace.Wrap(err, "failed exec command streaming")
	}

	if p.closedByClient.Load() {
		return nil // No need to send close envelope to the web UI if it was already closed by user.
	}

	// TODO(anton): refactor UI - right now if we send the close message UI will remove all text
	// from the document, which doesn't make sense for non-interactive command, since user
	// never has the chance to see the output.
	if p.req.IsInteractive {
		// Send close envelope to web terminal upon exit without an error.
		if err := stream.SendCloseMessage(""); err != nil {
			p.logger.ErrorContext(ctx, "unable to send close event to web client", "error", err)
		}
	}

	if err := stream.Close(); err != nil {
		p.logger.ErrorContext(ctx, "unable to close websocket stream to web client", "error", err)
		return nil
	}

	p.logger.DebugContext(ctx, "Sent close event to web client", "error", err)

	return nil
}

func (p *podHandler) handleResize(termSizeQueue *termSizeQueue) func(context.Context, terminal.Envelope) {
	return func(ctx context.Context, envelope terminal.Envelope) {
		var e map[string]any
		if err := json.Unmarshal([]byte(envelope.Payload), &e); err != nil {
			p.logger.WarnContext(ctx, "Failed to parse resize payload", "error", err)
			return
		}

		size, ok := e["size"].(string)
		if !ok {
			p.logger.ErrorContext(ctx, "got unexpected size type, expected type string", "size_type", logutils.TypeAttr(size))
			return
		}

		params, err := session.UnmarshalTerminalParams(size)
		if err != nil {
			p.logger.WarnContext(ctx, "Failed to retrieve terminal size", "error", err)
			return
		}

		// nil params indicates the channel was closed
		if params == nil {
			return
		}

		termSizeQueue.AddSize(remotecommand.TerminalSize{
			Width:  params.Winsize().Width,
			Height: params.Winsize().Height,
		})
	}
}

type termSizeQueue struct {
	incoming chan remotecommand.TerminalSize
	ctx      context.Context
}

func newTermSizeQueue(ctx context.Context, initialSize remotecommand.TerminalSize) *termSizeQueue {
	queue := &termSizeQueue{
		incoming: make(chan remotecommand.TerminalSize, 1),
		ctx:      ctx,
	}
	queue.AddSize(initialSize)
	return queue
}

func (r *termSizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case <-r.ctx.Done():
		return nil
	case size := <-r.incoming:
		return &size
	}
}

func (r *termSizeQueue) AddSize(term remotecommand.TerminalSize) {
	select {
	case <-r.ctx.Done():
	case r.incoming <- term:
	}
}

func createKubeRestConfig(serverAddr, tlsServerName string, ca types.CertAuthority, clientCert, rsaKey []byte) (*rest.Config, error) {
	var clusterCACerts [][]byte
	for _, keyPair := range ca.GetTrustedTLSKeyPairs() {
		clusterCACerts = append(clusterCACerts, keyPair.Cert)
	}
	return &rest.Config{
		Host: serverAddr,
		TLSClientConfig: rest.TLSClientConfig{
			CertData:   clientCert,
			KeyData:    rsaKey,
			CAData:     bytes.Join(clusterCACerts, []byte("\n")),
			ServerName: tlsServerName,
		},
	}, nil
}
