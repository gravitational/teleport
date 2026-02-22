/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	oteltrace "go.opentelemetry.io/otel/trace"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/gravitational/teleport"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kubetui"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/web/terminal"
)

// kubeTUIConnectParams contains the query parameters for the kube TUI connection.
type kubeTUIConnectParams struct {
	// KubeCluster specifies which Kubernetes cluster to connect to.
	KubeCluster string `json:"kubeCluster"`
	// Term is the initial terminal size.
	Term session.TerminalParams `json:"term"`
}

// kubeTUIHandler connects a K9s-like TUI to a web-based terminal via WebSocket.
// Instead of exec-ing into a pod, it runs a Bubble Tea program in-process that
// provides a pod explorer interface (list, logs, describe).
type kubeTUIHandler struct {
	teleportCluster     string
	configTLSServerName string
	configServerAddr    string
	publicProxyAddr     string
	kubeCluster         string
	term                session.TerminalParams
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

// ServeHTTP sends session metadata to the web UI and then runs the TUI.
func (h *kubeTUIHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	h.sctx.AddClosers(h)
	defer h.sctx.RemoveCloser(h)

	sessionMetadataResponse, err := json.Marshal(siteSessionGenerateResponse{Session: h.sess})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed marshaling session data", "error", err)
		if err := h.sendErrorMessage(err); err != nil {
			h.logger.ErrorContext(r.Context(), "failed to send error message to client", "error", err)
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
		h.logger.ErrorContext(r.Context(), "failed marshaling message envelope", "error", err)
		if err := h.sendErrorMessage(err); err != nil {
			h.logger.ErrorContext(r.Context(), "failed to send error message to client", "error", err)
		}
		return
	}

	if err := h.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes); err != nil {
		h.logger.ErrorContext(r.Context(), "failed write session data message", "error", err)
		if err := h.sendErrorMessage(err); err != nil {
			h.logger.ErrorContext(r.Context(), "failed to send error message to client", "error", err)
		}
		return
	}

	if err := h.handler(r); err != nil {
		h.logger.ErrorContext(r.Context(), "handling kube TUI session unexpectedly terminated", "error", err)
		if err := h.sendErrorMessage(err); err != nil {
			h.logger.ErrorContext(r.Context(), "failed to send error message to client", "error", err)
		}
	}
}

// Close closes the underlying WebSocket connection.
func (h *kubeTUIHandler) Close() error {
	return trace.Wrap(h.ws.Close())
}

func (h *kubeTUIHandler) sendErrorMessage(err error) error {
	if h.closedByClient.Load() {
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
	if err := h.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes); err != nil {
		return trace.Wrap(err, "writing error message")
	}

	return nil
}

// handler performs the MFA ceremony, creates a K8s client, and runs the Bubble Tea TUI.
func (h *kubeTUIHandler) handler(r *http.Request) error {
	h.logger.DebugContext(r.Context(), "Creating kube TUI session")

	tctx := oteltrace.ContextWithRemoteSpanContext(context.Background(), oteltrace.SpanContextFromContext(r.Context()))
	ctx, cancel := context.WithCancel(tctx)
	defer cancel()

	// Handle WebSocket close from client
	defaultCloseHandler := h.ws.CloseHandler()
	h.ws.SetCloseHandler(func(code int, text string) error {
		h.closedByClient.Store(true)
		h.logger.DebugContext(r.Context(), "websocket connection was closed by client")

		cancel()
		if defaultCloseHandler != nil {
			err := defaultCloseHandler(code, text)
			return trace.NewAggregate(err, h.Close())
		}
		return trace.Wrap(h.Close())
	})

	// Start sending ping frames to keep the connection alive
	go startWSPingLoop(r.Context(), h.ws, h.keepAliveInterval, h.logger, h.Close)

	// Extract user private key
	pk, err := keys.ParsePrivateKey(h.sctx.cfg.Session.GetTLSPriv())
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

	// onResize is swapped between TUI mode (sends to Bubble Tea) and exec
	// mode (sends to the K8s terminal size queue).
	var onResize func(w, h int)

	// Create WebSocket stream with a resize handler that delegates to onResize.
	stream := terminal.NewWStream(ctx, h.ws, h.logger, map[string]terminal.WSHandlerFunc{
		defaults.WebsocketResize: func(_ context.Context, envelope terminal.Envelope) {
			var e map[string]any
			if err := json.Unmarshal([]byte(envelope.Payload), &e); err != nil {
				h.logger.WarnContext(ctx, "Failed to parse resize payload", "error", err)
				return
			}

			size, ok := e["size"].(string)
			if !ok {
				return
			}

			params, err := session.UnmarshalTerminalParams(size)
			if err != nil {
				h.logger.WarnContext(ctx, "Failed to retrieve terminal size", "error", err)
				return
			}

			if params == nil {
				return
			}

			if onResize != nil {
				onResize(int(params.Winsize().Width), int(params.Winsize().Height))
			}
		},
	})

	// MFA ceremony and certificate generation (same pattern as podExecHandler)
	certsReq := clientproto.UserCertsRequest{
		TLSPublicKey:      publicKeyPEM,
		Username:          h.sctx.GetUser(),
		Expires:           h.sctx.cfg.Session.GetExpiryTime(),
		Format:            constants.CertificateFormatStandard,
		RouteToCluster:    h.teleportCluster,
		KubernetesCluster: h.kubeCluster,
		Usage:             clientproto.UserCertsRequest_Kubernetes,
	}

	var certs *clientproto.Certs
	result, err := client.PerformSessionMFACeremony(ctx, client.PerformSessionMFACeremonyParams{
		CurrentAuthClient: h.userClient,
		RootAuthClient:    h.sctx.cfg.RootClient,
		MFACeremony:       newMFACeremony(stream, h.sctx.cfg.RootClient.CreateAuthenticateChallenge, h.publicProxyAddr),
		MFAAgainstRoot:    h.sctx.cfg.RootClusterName == h.teleportCluster,
		MFARequiredReq: &clientproto.IsMFARequiredRequest{
			Target: &clientproto.IsMFARequiredRequest_KubernetesCluster{KubernetesCluster: h.kubeCluster},
		},
		CertsReq: &certsReq,
	})
	if err != nil && !errors.Is(err, services.ErrSessionMFANotRequired) {
		return trace.Wrap(err, "failed performing mfa ceremony")
	} else if result != nil {
		certs = result.NewCerts
	}

	if certs == nil {
		certs, err = h.sctx.cfg.RootClient.GenerateUserCerts(ctx, certsReq)
		if err != nil {
			return trace.Wrap(err, "failed issuing user certs")
		}
	}

	// Create in-memory Kubernetes REST config
	restConfig, err := createKubeRestConfig(h.configServerAddr, h.configTLSServerName, h.localCA, certs.TLS, privateKeyPEM)
	if err != nil {
		return trace.Wrap(err, "failed creating Kubernetes rest config")
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return trace.Wrap(err, "failed creating Kubernetes client")
	}

	// Run the Bubble Tea TUI
	return h.runTUI(ctx, stream, kubeClient, restConfig, &onResize)
}

// runTUI creates and runs the Bubble Tea program with the WebSocket stream as I/O.
// When the user selects "exec", the program quits, the handler runs the K8s exec
// session directly, and then restarts the TUI.
func (h *kubeTUIHandler) runTUI(ctx context.Context, stream *terminal.WSStream, kubeClient kubernetes.Interface, restConfig *rest.Config, onResize *func(w, h int)) error {
	// Force TrueColor since xterm.js supports it
	lipgloss.SetColorProfile(0)

	model := kubetui.NewModel(kubeClient, restConfig, h.kubeCluster, int(h.term.Winsize().Width), int(h.term.Winsize().Height))

	for {
		p := tea.NewProgram(
			model,
			tea.WithInput(stream),
			tea.WithOutput(stream),
			tea.WithAltScreen(),
			tea.WithoutSignalHandler(),
			tea.WithContext(ctx),
		)

		// Point the resize handler at this program instance.
		*onResize = func(w, ht int) {
			p.Send(tea.WindowSizeMsg{Width: w, Height: ht})
		}

		finalModel, err := p.Run()
		if err != nil {
			return trace.Wrap(err, "TUI program exited with error")
		}

		m, ok := finalModel.(kubetui.Model)
		if !ok || !m.WantsExec() {
			break // normal quit (:q or ctrl+c)
		}

		// Clear the normal screen buffer before exec so previous exec
		// output doesn't reappear when alt screen is exited again.
		stream.Write([]byte("\033[H\033[2J")) //nolint:errcheck

		// Run exec session
		execReq := m.GetExecRequest()
		if err := h.runExec(ctx, stream, m.Client(), execReq, onResize); err != nil {
			h.logger.WarnContext(ctx, "exec session ended with error", "error", err)
		}

		// Clear the screen after exec so the normal buffer is clean
		// before the TUI re-enters alt screen.
		stream.Write([]byte("\033[H\033[2J")) //nolint:errcheck

		// Restart TUI with cleaned-up model
		m.ClearExec()
		model = m
	}

	if h.closedByClient.Load() {
		return nil
	}

	// Send close message to web UI
	if err := stream.SendCloseMessage(""); err != nil {
		h.logger.ErrorContext(ctx, "unable to send close event to web client", "error", err)
	}

	return nil
}

// runExec runs an interactive shell exec session inside a pod, routing the
// WSStream directly to the K8s exec stream and terminal resizes to the size queue.
func (h *kubeTUIHandler) runExec(ctx context.Context, stream *terminal.WSStream, client *kubetui.Client, req *kubetui.ExecRequest, onResize *func(w, h int)) error {
	sizeQueue := newTermSizeQueue(ctx, remotecommand.TerminalSize{
		Width:  h.term.Winsize().Width,
		Height: h.term.Winsize().Height,
	})

	// Swap resize handler to feed the exec size queue.
	*onResize = func(w, ht int) {
		sizeQueue.AddSize(remotecommand.TerminalSize{Width: uint16(w), Height: uint16(ht)})
	}

	return client.ExecPod(ctx, kubetui.ExecConfig{
		Namespace: req.Namespace,
		Pod:       req.Pod,
		Container: req.Container,
		Stdin:     stream,
		Stdout:    stream,
		SizeQueue: sizeQueue,
	})
}

// kubeTUIConnect handles the kube TUI WebSocket connection.
func (h *Handler) kubeTUIConnect(
	w http.ResponseWriter,
	r *http.Request,
	_ httprouter.Params,
	sctx *SessionContext,
	cluster reversetunnelclient.Cluster,
	ws *websocket.Conn,
) (any, error) {
	q := r.URL.Query()
	if q.Get("params") == "" {
		return nil, trace.BadParameter("missing params")
	}
	var params kubeTUIConnectParams
	if err := json.Unmarshal([]byte(q.Get("params")), &params); err != nil {
		return nil, trace.Wrap(err)
	}

	if params.KubeCluster == "" {
		return nil, trace.BadParameter("missing kubeCluster parameter")
	}

	sess := session.Session{
		Kind:                  types.KubernetesSessionKind,
		Login:                 "root",
		ClusterName:           cluster.GetName(),
		KubernetesClusterName: params.KubeCluster,
		ID:                    session.NewID(),
		Created:               h.clock.Now().UTC(),
		LastActive:            h.clock.Now().UTC(),
		Namespace:             apidefaults.Namespace,
		Owner:                 sctx.GetUser(),
		Command:               "kubetui",
	}

	h.logger.DebugContext(r.Context(), "New kube TUI request",
		"kubeCluster", params.KubeCluster,
		"sid", sess.ID,
		"websid", sctx.GetSessionID(),
	)

	authAccessPoint, err := cluster.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	netConfig, err := authAccessPoint.GetClusterNetworkingConfig(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverAddr, tlsServerName, err := h.getKubeExecClusterData(netConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostCA, err := h.auth.accessPoint.GetCertAuthority(r.Context(), types.CertAuthID{
		Type:       types.HostCA,
		DomainName: h.auth.clusterName,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	th := &kubeTUIHandler{
		kubeCluster:         params.KubeCluster,
		term:                params.Term,
		sess:                sess,
		sctx:                sctx,
		teleportCluster:     cluster.GetName(),
		ws:                  ws,
		keepAliveInterval:   netConfig.GetKeepAliveInterval(),
		logger:              h.logger.With(teleport.ComponentKey, "kubetui"),
		userClient:          clt,
		localCA:             hostCA,
		configServerAddr:    serverAddr,
		configTLSServerName: tlsServerName,
		publicProxyAddr:     h.cfg.PublicProxyAddr,
	}

	th.ServeHTTP(w, r)
	return nil, nil
}
