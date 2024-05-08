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
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
)

// podHandler connects Kube exec session and web-based terminal via a websocket.
type podHandler struct {
	teleportCluster     string
	configTLSServerName string
	configServerAddr    string
	req                 PodExecRequest
	sess                session.Session
	sctx                *SessionContext
	ws                  *websocket.Conn
	keepAliveInterval   time.Duration
	log                 *logrus.Entry
	userClient          auth.ClientI
	localCA             types.CertAuthority

	// closedByClient indicates if the websocket connection was closed by the
	// user (closing the browser tab, exiting the session, etc).
	closedByClient atomic.Bool
}

// PodExecRequest describes a request to create a web-based terminal
// to exec into a pod.
type PodExecRequest struct {
	// Namespace is the namespace of the target pod
	Namespace string `json:"namespace"`
	// Pod is the target pod to connect to.
	Pod string `json:"pod"`
	// Container is a container within the target pod to connect to, optional.
	Container string `json:"container"`
	// Command is the command to run at the target pod.
	Command string `json:"command"`
	// KubeCluster specifies what Kubernetes cluster to connect to.
	KubeCluster string `json:"kube_cluster"`
	// IsInteractive specifies whether exec request should have interactive TTY.
	IsInteractive bool `json:"is_interactive"`
	// Term is the initial PTY size.
	Term session.TerminalParams `json:"term"`
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
		p.sendAndLogError(err)
		return
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketSessionMetadata,
		Payload: string(sessionMetadataResponse),
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		p.sendAndLogError(err)
		return
	}

	err = p.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	if err != nil {
		p.sendAndLogError(err)
		return
	}

	if err := p.handler(r); err != nil {
		p.sendAndLogError(err)
	}
}

func (p *podHandler) Close() error {
	return trace.Wrap(p.ws.Close())
}

func (p *podHandler) sendAndLogError(err error) {
	p.log.Error(err)

	if p.closedByClient.Load() {
		return
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketError,
		Payload: err.Error(),
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		p.log.WithError(err).Error("failed to marshal error message")
		return
	}
	if err := p.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes); err != nil {
		p.log.WithError(err).Error("failed to send error message")
	}
}

func (p *podHandler) handler(r *http.Request) error {
	p.log.Debug("Creating websocket stream for a kube exec request")

	// Create a context for signaling when the terminal session is over and
	// link it first with the trace context from the request context
	tctx := oteltrace.ContextWithRemoteSpanContext(context.Background(), oteltrace.SpanContextFromContext(r.Context()))
	ctx, cancel := context.WithCancel(tctx)
	defer cancel()

	defaultCloseHandler := p.ws.CloseHandler()
	p.ws.SetCloseHandler(func(code int, text string) error {
		p.closedByClient.Store(true)
		p.log.Debug("websocket connection was closed by client")

		cancel()
		// Call the default close handler if one was set.
		if defaultCloseHandler != nil {
			err := defaultCloseHandler(code, text)
			return trace.NewAggregate(err, p.Close())
		}

		return trace.Wrap(p.Close())
	})

	// Start sending ping frames through websocket to the client.
	go startWSPingLoop(r.Context(), p.ws, p.keepAliveInterval, p.log, p.Close)

	pk, err := keys.ParsePrivateKey(p.sctx.cfg.Session.GetPriv())
	if err != nil {
		return trace.Wrap(err, "failed getting user private key from the session")
	}
	userKey := &client.Key{
		PrivateKey: pk,
		Cert:       p.sctx.cfg.Session.GetPub(),
		TLSCert:    p.sctx.cfg.Session.GetTLSCert(),
	}

	stream := NewTerminalStream(ctx, TerminalStreamConfig{WS: p.ws, Logger: p.log})

	certsReq := clientproto.UserCertsRequest{
		PublicKey:         userKey.MarshalSSHPublicKey(),
		Username:          p.sctx.GetUser(),
		Expires:           p.sctx.cfg.Session.GetExpiryTime(),
		Format:            constants.CertificateFormatStandard,
		RouteToCluster:    p.teleportCluster,
		KubernetesCluster: p.req.KubeCluster,
		Usage:             clientproto.UserCertsRequest_Kubernetes,
	}

	_, certs, err := client.PerformMFACeremony(ctx, client.PerformMFACeremonyParams{
		CurrentAuthClient: p.userClient,
		RootAuthClient:    p.sctx.cfg.RootClient,
		MFAPrompt: mfa.PromptFunc(func(ctx context.Context, chal *clientproto.MFAAuthenticateChallenge) (*clientproto.MFAAuthenticateResponse, error) {
			assertion, err := promptMFAChallenge(stream.WSStream, protobufMFACodec{}).Run(ctx, chal)
			return assertion, trace.Wrap(err)
		}),
		MFAAgainstRoot: p.sctx.cfg.RootClusterName == p.teleportCluster,
		MFARequiredReq: &clientproto.IsMFARequiredRequest{
			Target: &clientproto.IsMFARequiredRequest_KubernetesCluster{KubernetesCluster: p.req.KubeCluster},
		},
		ChallengeExtensions: mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
		},
		CertsReq: &certsReq,
		Key:      userKey,
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

	rsaKey, err := userKey.PrivateKey.RSAPrivateKeyPEM()
	if err != nil {
		return trace.Wrap(err, "failed getting rsa private key")
	}

	restConfig, err := createKubeRestConfig(p.configServerAddr, p.configTLSServerName, p.localCA, certs.TLS, rsaKey)
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
	p.log.Debugf("Web kube exec request URL: %s", kubeReq.URL())

	wsExec, err := remotecommand.NewWebSocketExecutor(restConfig, "POST", kubeReq.URL().String())
	if err != nil {
		return trace.Wrap(err, "failed creating websocket executor")
	}

	streamOpts := remotecommand.StreamOptions{
		Stdin:  stream,
		Stdout: stream,
		Tty:    p.req.IsInteractive,
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
		if err := stream.SendCloseMessage(sessionEndEvent{}); err != nil {
			p.log.WithError(err).Error("unable to send close event to web client.")
		}
	}

	if err := stream.Close(); err != nil {
		p.log.WithError(err).Error("unable to close websocket stream to web client.")
		return nil
	}

	p.log.Debug("Sent close event to web client.")

	return nil
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
