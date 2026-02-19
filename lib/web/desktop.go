/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"crypto"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/desktop"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/diagnostics/latency"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	tdpbQueryParameter = "tdpb"
	protocolTDP        = "teleport-tdp"
)

// GET /webapi/sites/:site/desktops/:desktopName/connect?username=<username>
func (h *Handler) desktopConnectHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	cluster reversetunnelclient.Cluster,
	ws *websocket.Conn,
) (any, error) {
	desktopName := p.ByName("desktopName")
	if desktopName == "" {
		return nil, trace.BadParameter("missing desktopName in request URL")
	}

	log := sctx.cfg.Log.With(
		"desktop_name", desktopName,
		"cluster_name", cluster.GetName(),
	)
	log.DebugContext(r.Context(), "New desktop access websocket connection")

	if err := h.createDesktopConnection(r, desktopName, cluster.GetName(), log, sctx, cluster, ws); err != nil {
		// createDesktopConnection makes a best effort attempt to send an error to the user
		// (via websocket) before terminating the connection. We log the error here, but
		// return nil because our HTTP middleware will try to write the returned error in JSON
		// format, and this will fail since the HTTP connection has been upgraded to websockets.
		log.ErrorContext(r.Context(), "creating desktop connection failed", "error", err)
	}

	return nil, nil
}

// Adapts a websocket to a tdp.MessageReadWriter.
// Quietly discards TDP messages.
type desktopWebsocketAdapter struct {
	conn *websocket.Conn
	// Avoid allocating a new byte slice with each received message
	// be re-using a buffer.
	buf bytes.Buffer
}

// ReadMessage returns a new Message read from the underlying websocket.
func (w *desktopWebsocketAdapter) ReadMessage() (tdp.Message, error) {
	for {
		w.buf.Reset()

		mType, rdr, err := w.conn.NextReader()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if mType != websocket.BinaryMessage {
			return nil, trace.Errorf("expected binary message, got: %d", mType)
		}

		if _, err := io.Copy(&w.buf, rdr); err != nil {
			return nil, trace.Wrap(err)
		}

		msg, err := tdpb.DecodeWithTDPDiscard(w.buf.Bytes())
		if err != nil {
			if errors.Is(err, tdpb.ErrIsTDP) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		return msg, nil
	}
}

// WriteMessage writes a new Message to the underlying websocket.
func (w *desktopWebsocketAdapter) WriteMessage(m tdp.Message) error {
	data, err := m.Encode()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(w.conn.WriteMessage(websocket.BinaryMessage, data))
}

// implements handshaker for legacy TDP clients
// TODO(rhammonds) DELETE IN v20.0.0
type tdpHandshaker struct {
	connection tdp.MessageReadWriter
	withheld   []tdp.Message
	screenSpec legacy.ClientScreenSpec
	// May or may not be nil. Not all web client versions will send a keyboard layout.
	keyboardLayout *legacy.ClientKeyboardLayout
}

func (t *tdpHandshaker) sendError(ctx context.Context, log *slog.Logger, err error) error {
	if err == nil {
		log.WarnContext(ctx, "SendError called with empty message")
		err = errors.New("an an unknown error has occurred")
	}

	return trace.Wrap(t.connection.WriteMessage((&legacy.Alert{
		Message:  err.Error(),
		Severity: legacy.SeverityError,
	})))
}

func (t *tdpHandshaker) getPromptBuilder(log *slog.Logger) mfaPromptBuilder {
	return mfaPromptBuilder(legacy.NewTDPMFAPrompt(t.connection, &t.withheld, log))
}

func (t *tdpHandshaker) performInitialHandshake(ctx context.Context, log *slog.Logger) error {
	msg, err := t.connection.ReadMessage()
	if err != nil {
		return trace.Wrap(err)
	}

	screenSpec, ok := msg.(legacy.ClientScreenSpec)
	if !ok {
		return trace.BadParameter("client sent unexpected message %T", msg)
	}
	t.screenSpec = screenSpec

	width, height := screenSpec.Width, screenSpec.Height
	if width > types.MaxRDPScreenWidth || height > types.MaxRDPScreenHeight {
		return trace.BadParameter(
			"screen size of %d x %d is greater than the maximum allowed by RDP (%d x %d)",
			width, height, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight,
		)
	}

	msg, err = t.connection.ReadMessage()
	if err != nil {
		return trace.Wrap(err)
	}

	keyboardLayout, gotKeyboardLayout := msg.(legacy.ClientKeyboardLayout)
	if !gotKeyboardLayout {
		t.withheld = append(t.withheld, msg)
		log.InfoContext(ctx, "client did not send keyboard layout", "message_type", logutils.TypeAttr(msg), "width", width, "height", height)
	} else {
		t.keyboardLayout = &keyboardLayout
	}
	return nil
}

func (t *tdpHandshaker) forwardTDP(w io.Writer, username string, forwardKeyboardLayout bool) error {
	messages := make([]tdp.Message, 0, 3)
	messages = append(messages, legacy.ClientUsername{Username: username})
	messages = append(messages, t.screenSpec)

	if t.keyboardLayout != nil && forwardKeyboardLayout {
		// TDPB clients will always send the keyboard layout with the Client Hello.
		messages = append(messages, t.keyboardLayout)
	}

	return sendAll(w, append(messages, t.withheld...))
}

func (t *tdpHandshaker) forwardTDPB(w io.Writer, username string, _ bool) error {
	// Convert to Client Hello
	hello := &tdpb.ClientHello{
		ScreenSpec: &tdpbv1.ClientScreenSpec{
			Height: t.screenSpec.Height,
			Width:  t.screenSpec.Width,
		},
		Username: username,
	}
	if t.keyboardLayout != nil {
		hello.KeyboardLayout = t.keyboardLayout.KeyboardLayout
	}

	withheld, err := translateAll(t.withheld, tdpb.TranslateToModern)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(sendAll(w, append([]tdp.Message{hello}, withheld...)))
}

func translateAll(messages []tdp.Message, translate func(tdp.Message) ([]tdp.Message, error)) ([]tdp.Message, error) {
	translated := make([]tdp.Message, 0, len(messages))
	for _, msg := range messages {
		out, err := translate(msg)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if len(out) > 0 {
			translated = append(translated, out...)
		}
	}
	return translated, nil
}

// implements handshaker for TDPB clients
type tdpbHandshaker struct {
	connection tdp.MessageReadWriter
	withheld   []tdp.Message
	hello      *tdpb.ClientHello
}

func (t *tdpbHandshaker) sendError(ctx context.Context, log *slog.Logger, err error) error {
	if err == nil {
		log.WarnContext(ctx, "sendError called with empty message")
		err = errors.New("an unknown error has occurred")
	}

	return trace.Wrap(t.connection.WriteMessage((&tdpb.Alert{
		Message:  err.Error(),
		Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR,
	})))
}

func (t *tdpbHandshaker) getPromptBuilder(log *slog.Logger) mfaPromptBuilder {
	return mfaPromptBuilder(tdpb.NewTDPBMFAPrompt(t.connection, &t.withheld, log))
}

func (t *tdpbHandshaker) performInitialHandshake(ctx context.Context, log *slog.Logger) error {
	upgrade := legacy.TDPUpgrade{}
	err := t.connection.WriteMessage(upgrade)
	if err != nil {
		return trace.Wrap(err)
	}

	// Now wait patiently for the client to reply with a CLIENT_HELLO TDPB message
	// The ReadWriter implementation is expected to discard any legacy TDP messages
	// while waiting for the client hello.
	msg, err := t.connection.ReadMessage()
	if err != nil {
		return trace.Wrap(err)
	}

	var ok bool
	t.hello, ok = msg.(*tdpb.ClientHello)
	if !ok {
		return trace.Errorf("Expected client hello message but got %T", msg)
	}

	log.InfoContext(ctx, "Received client hello message", "message", t.hello)
	return nil
}

func (t *tdpbHandshaker) forwardTDP(w io.Writer, username string, forwardKeyboardLayout bool) error {
	messages := make([]tdp.Message, 0, 3)
	messages = append(messages, legacy.ClientUsername{Username: username})

	screenSpec := legacy.ClientScreenSpec{
		Height: t.hello.ScreenSpec.Height,
		Width:  t.hello.ScreenSpec.Width,
	}
	messages = append(messages, screenSpec)

	if forwardKeyboardLayout {
		// TDPB clients will always send the keyboard layout with the Client Hello.
		messages = append(messages, legacy.ClientKeyboardLayout{KeyboardLayout: t.hello.KeyboardLayout})
	}

	withheld, err := translateAll(t.withheld, tdpb.TranslateToLegacy)
	if err != nil {
		return trace.Wrap(err)
	}
	return sendAll(w, append(messages, withheld...))
}

func (t *tdpbHandshaker) forwardTDPB(w io.Writer, username string, _ bool) error {
	t.hello.Username = username
	return trace.Wrap(sendAll(w, append([]tdp.Message{t.hello}, t.withheld...)))
}

func sendAll(w io.Writer, messages []tdp.Message) error {
	for _, msg := range messages {
		if err := tdp.EncodeTo(w, msg); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

type handshaker interface {
	sendError(context.Context, *slog.Logger, error) error
	getPromptBuilder(*slog.Logger) mfaPromptBuilder
	performInitialHandshake(context.Context, *slog.Logger) error
	forwardTDP(io.Writer, string, bool) error
	forwardTDPB(io.Writer, string, bool) error
}

// creates a handshaker instance that interops with either TDP or TDPB clients
func newHandshaker(protocol string, ws *websocket.Conn) handshaker {
	if protocol == tdpb.ProtocolName {
		return &tdpbHandshaker{
			connection: &desktopWebsocketAdapter{conn: ws},
		}
	}
	// Default to TDP
	return &tdpHandshaker{
		connection: tdp.NewConn(&WebsocketIO{Conn: ws}, legacy.Decode),
	}
}

type mfaPromptBuilder func(string) mfa.PromptFunc

func (h *Handler) createDesktopConnection(
	r *http.Request,
	desktopName string,
	clusterName string,
	log *slog.Logger,
	sctx *SessionContext,
	cluster reversetunnelclient.Cluster,
	ws *websocket.Conn,
) error {
	defer ws.Close()
	ctx := r.Context()

	// Client may speak TDP or TDPB. We'll know based on the existence of the 'tdpb' query parameter.
	// - If the 'tdpb' query parameter is present, then we'll need to send an upgrade message to the client
	//   and listen for a client_hello message (while discarding any TDP messages received).
	//   Note: We *always* upgrade the client connection to TDPB if possible.
	// - Otherwise fall back to the "legacy" behavior
	//
	// After either receiving a client_hello or our initial TDP messages, we can dial the agent which
	// *also* might speak TDP or TDPB. Unlike the client, the agent only speaks one or the other so we'll
	// translate on its behalf if needed.
	clientProtocol, err := readClientProtocol(r)
	if err != nil {
		log.ErrorContext(ctx, "Error reading client desktop protocol", "error", err)
		return trace.Wrap(err)
	}
	log.InfoContext(ctx, "Creating Desktop connection", "client_protocol", clientProtocol)

	handshaker := newHandshaker(clientProtocol, ws)
	// Read the initial set of TDP messages, or handle TDP upgrade and subsequent
	// Client Hello message.
	err = handshaker.performInitialHandshake(ctx, log)
	if err != nil {
		return handshaker.sendError(ctx, log, err)
	}

	username, err := readUsername(r)
	if err != nil {
		return handshaker.sendError(ctx, log, err)
	}

	// Parse the private key of the user from the session context.
	pk, err := keys.ParsePrivateKey(sctx.cfg.Session.GetTLSPriv())
	if err != nil {
		return handshaker.sendError(ctx, log, err)
	}

	// Check if MFA is required and create a UserCertsRequest.
	mfaRequired, certsReq, err := h.prepareForCertIssuance(ctx, sctx, cluster, pk.Public(), desktopName, username)
	if err != nil {
		return handshaker.sendError(ctx, log, err)
	}

	// Issue certificate for the user/desktop combination and perform MFA ceremony if required.
	certs, err := h.issueCerts(ctx, sctx, mfaRequired, certsReq, handshaker.getPromptBuilder(log))
	if err != nil {
		return handshaker.sendError(ctx, log, err)
	}

	// Create a TLS config for connecting to the Windows Desktop Service.
	tlsConfig, err := h.createDesktopTLSConfig(ctx, sctx, desktopName, pk, certs)
	if err != nil {
		return handshaker.sendError(ctx, log, err)
	}

	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		return handshaker.sendError(ctx, log, err)
	}

	log.DebugContext(ctx, "Attempting to connect to agent")
	clientSrcAddr, clientDstAddr := authz.ClientAddrsFromContext(ctx)
	serviceConn, version, err := desktop.ConnectToWindowsService(ctx, &desktop.ConnectionConfig{
		Log:            log,
		DesktopsGetter: clt,
		Cluster:        cluster,
		ClientSrcAddr:  clientSrcAddr,
		ClientDstAddr:  clientDstAddr,
		DesktopName:    desktopName,
		ClusterName:    clusterName,
	})
	if err != nil {
		return handshaker.sendError(ctx, log, trace.Wrap(err, "cannot connect to Windows Desktop Service"))
	}
	defer serviceConn.Close()

	serviceConnTLS := tls.Client(serviceConn, tlsConfig)

	if err := serviceConnTLS.HandshakeContext(ctx); err != nil {
		return handshaker.sendError(ctx, log, err)
	}

	// ALPN informs us which dialect the server will be using.
	// Now that we have a connection to the Windows Desktop Service, we can
	// forward the client_hello message (TDPB) or username and screen spec (TDP)
	// to the service, and any withheld messages that were received before the MFA
	// ceremony was completed.
	serverProtocol := serviceConnTLS.ConnectionState().NegotiatedProtocol
	switch serverProtocol {
	case "":
		serverProtocol = protocolTDP
		sendKeyboardLayout, _ := utils.MinVerWithoutPreRelease(version, "18.0.0")
		err = handshaker.forwardTDP(serviceConnTLS, username, sendKeyboardLayout)
	case tdpb.ProtocolName:
		err = handshaker.forwardTDPB(serviceConnTLS, username, true /* unused */)
	default:
		err = trace.BadParameter("Unknown desktop agent protocol %v", serverProtocol)
	}
	log.InfoContext(ctx, "Connected to windows_desktop_service", "agent_protocol", serverProtocol)

	if err != nil {
		return handshaker.sendError(ctx, log, err)
	}
	// this blocks until the connection is closed
	handleDesktopWebsocketProxyErr(
		ctx,
		desktopWebsocketProxy{
			ws,
			serviceConnTLS,
			version,
			clientProtocol,
			serverProtocol,
			log,
		}.run(ctx),
		log,
	)

	return nil
}

const (
	// SNISuffix is the server name suffix used during SNI to specify the
	// target desktop to connect to. The client (proxy_service) will use SNI
	// like "${UUID}.desktop.teleport.cluster.local" to pass the UUID of the
	// desktop.
	// This is a copy of the same constant in `lib/srv/desktop/desktop.go` to
	// prevent depending on `lib/srv` in `lib/web`.
	SNISuffix = ".desktop." + constants.APIDomain
)

func createUserCertsRequest(
	sctx *SessionContext,
	publicKey crypto.PublicKey,
	desktopName,
	username,
	siteName string,
) (*proto.UserCertsRequest, error) {
	tlsCert, err := sctx.GetX509Certificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicKeyPEM, err := keys.MarshalPublicKey(publicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certsReq := proto.UserCertsRequest{
		TLSPublicKey:   publicKeyPEM,
		Username:       tlsCert.Subject.CommonName,
		Expires:        tlsCert.NotAfter,
		RouteToCluster: siteName,
		Usage:          proto.UserCertsRequest_WindowsDesktop,
		RouteToWindowsDesktop: proto.RouteToWindowsDesktop{
			WindowsDesktop: desktopName,
			Login:          username,
		},
	}

	return &certsReq, nil
}

// prepareForCertIssuance prepares for certificate issuance by checking if MFA
// is required for the user/desktop combination and creating a UserCertsRequest.
func (h *Handler) prepareForCertIssuance(
	ctx context.Context,
	sctx *SessionContext,
	cluster reversetunnelclient.Cluster,
	publicKey crypto.PublicKey,
	desktopName, username string,
) (mfaRequired bool, certsReq *proto.UserCertsRequest, err error) {
	// Check if MFA is required for this user/desktop combination.
	mfaRequired, err = h.checkMFARequired(ctx, &IsMFARequiredRequest{
		WindowsDesktop: &isMFARequiredWindowsDesktop{
			DesktopName: desktopName,
			Login:       username,
		},
	}, sctx, cluster)
	if err != nil {
		return false, nil, trace.Wrap(err)
	}

	certsReq, err = createUserCertsRequest(sctx, publicKey, desktopName, username, cluster.GetName())
	if err != nil {
		return false, nil, trace.Wrap(err)
	}

	return mfaRequired, certsReq, nil
}

// issueCerts issues certificates for the user/desktop combination, performing
// the MFA ceremony if required.
func (h *Handler) issueCerts(
	ctx context.Context,
	sctx *SessionContext,
	mfaRequired bool,
	certsReq *proto.UserCertsRequest,
	promptConstructor mfaPromptBuilder,
) (certs *proto.Certs, err error) {
	if mfaRequired {
		certs, err = h.performSessionMFACeremony(ctx, sctx, certsReq, promptConstructor)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		certs, err = sctx.cfg.RootClient.GenerateUserCerts(ctx, *certsReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return certs, nil
}

// createDesktopTLSConfig creates a TLS config for connecting to a Windows Desktop Service
// using the user's private key and the issued certificates.
func (h *Handler) createDesktopTLSConfig(
	ctx context.Context,
	sctx *SessionContext,
	desktopName string,
	pk *keys.PrivateKey,
	certs *proto.Certs,
) (*tls.Config, error) {
	certConf, err := pk.TLSCertificate(certs.TLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := sctx.ClientTLSConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig.Certificates = []tls.Certificate{certConf}
	tlsConfig.NextProtos = []string{tdpb.ProtocolName}
	// Pass target desktop name via SNI.
	tlsConfig.ServerName = desktopName + SNISuffix
	return tlsConfig, nil
}

// performSessionMFACeremony completes the mfa ceremony and returns the raw TLS certificate
// on success. The user will be prompted to tap their security key by the UI
// in order to perform the assertion.
func (h *Handler) performSessionMFACeremony(
	ctx context.Context,
	sctx *SessionContext,
	certsReq *proto.UserCertsRequest,
	promptConstructor mfaPromptBuilder,
) (_ *proto.Certs, err error) {
	ctx, span := h.tracer.Start(ctx, "desktop/performSessionMFACeremony")
	defer func() {
		span.RecordError(err)
		span.End()
	}()

	// channelID is used by the front end to differentiate between separate ongoing SSO challenges.
	channelID := uuid.NewString()

	mfaCeremony := &mfa.Ceremony{
		CreateAuthenticateChallenge: sctx.cfg.RootClient.CreateAuthenticateChallenge,
		SSOMFACeremonyConstructor: func(_ context.Context) (mfa.SSOMFACeremony, error) {
			u, err := url.Parse(sso.WebMFARedirect)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			u.RawQuery = url.Values{"channel_id": {channelID}}.Encode()
			return &sso.MFACeremony{
				ClientCallbackURL: u.String(),
				ProxyAddress:      h.PublicProxyAddr(),
			}, nil
		},
		PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
			return promptConstructor(channelID)
		},
	}

	result, err := client.PerformSessionMFACeremony(ctx, client.PerformSessionMFACeremonyParams{
		CurrentAuthClient: nil, // Only RootAuthClient is used.
		RootAuthClient:    sctx.cfg.RootClient,
		MFACeremony:       mfaCeremony,
		MFAAgainstRoot:    true,
		MFARequiredReq:    nil, // No need to verify.
		CertsReq:          certsReq,
		KeyRing:           nil, // We just want the certs.
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return result.NewCerts, nil
}

func readUsername(r *http.Request) (string, error) {
	q := r.URL.Query()
	username := q.Get("username")
	if username == "" {
		return "", trace.BadParameter("missing username in URL")
	}

	return username, nil
}

func readClientProtocol(r *http.Request) (string, error) {
	q := r.URL.Query()
	tdpbVersion := q.Get(tdpbQueryParameter)
	switch tdpbVersion {
	case "":
		return protocolTDP, nil
	case tdpb.ProtocolName:
		return tdpb.ProtocolName, nil
	default:
		return "", trace.BadParameter("unknown TDPB version %q", tdpbVersion)
	}
}

// desktopPinger measures latency between proxy and the desktop by sending legacy.Ping messages
// Windows Desktop Service and measuring the time it takes to receive message with the same UUID back.
type desktopPinger struct {
	server tdp.MessageWriter
	client tdp.MessageWriter
	// when false, the interceptor function swallows ping messages
	// without writing to the channel
	latencySupported bool
	ch               chan []byte
}

func (d desktopPinger) intercept(msg tdp.Message) ([]tdp.Message, error) {
	var uuid []byte
	switch m := msg.(type) {
	case legacy.Ping:
		uuid = m.UUID[:]
	case *tdpb.Ping:
		uuid = m.Uuid
	default:
		// This may be some other legacy TDP message
		return []tdp.Message{msg}, nil
	}

	if !d.latencySupported {
		return nil, trace.BadParameter("received unexpected Ping message from server (this is a bug)")
	}

	d.ch <- uuid
	// We've handled the ping. Do not pass it along to the proxy.
	return nil, nil
}

func (d desktopPinger) ping(ctx context.Context, ping []byte, msg tdp.Message) error {
	// The provided 'ping' byte slice should match the UUID contained in 'msg'
	if err := d.server.WriteMessage(msg); err != nil {
		return trace.Wrap(err)
	}
	for {
		select {
		case pong := <-d.ch:
			if bytes.Equal(ping, pong) {
				return nil
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

func (d desktopPinger) reportTDPB(_ context.Context, stats latency.Statistics) error {
	return d.client.WriteMessage(&tdpb.LatencyStats{
		ClientLatencyMs: uint32(stats.Client),
		ServerLatencyMs: uint32(stats.Server),
	})
}

func (d desktopPinger) reportTDP(_ context.Context, stats latency.Statistics) error {
	return d.client.WriteMessage(legacy.LatencyStats{
		ClientLatency: uint32(stats.Client),
		ServerLatency: uint32(stats.Server)},
	)
}

func (d desktopPinger) pingTDP(ctx context.Context) error {
	ping := uuid.New()
	return d.ping(ctx, ping[:], legacy.Ping{UUID: ping})
}

func (d desktopPinger) pingTDPB(ctx context.Context) error {
	uuid := uuid.New()
	return d.ping(ctx, uuid[:], &tdpb.Ping{
		Uuid: uuid[:],
	})
}

func newConn(rwc io.ReadWriteCloser, protocol string) *tdp.Conn {
	if protocol == tdpb.ProtocolName {
		return tdp.NewConn(rwc, tdp.DecoderAdapter(tdpb.DecodePermissive))
	}
	return tdp.NewConn(rwc, legacy.Decode)
}

type desktopWebsocketProxy struct {
	// Client websocket connection
	ws *websocket.Conn
	// Desktop agent connection
	wds net.Conn
	// Version of the Desktop Agent
	version string
	// Client protocol (TDP/TDPB)
	clientProtocol string
	// Server protocol (TDP/TDPB)
	serverProtocol string
	log            *slog.Logger
}

// run does a bidrectional copy between the websocket
// connection to the browser (ws) and the mTLS connection to Windows
// Desktop Serivce (wds)
func (p desktopWebsocketProxy) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		p.ws.Close()
		p.wds.Close()
	}()

	latencySupported, err := utils.MinVerWithoutPreRelease(p.version, "17.5.0")
	if err != nil {
		return trace.Wrap(err)
	}

	// Create a single pair of legacy.Conn instances. legacy.Conn protects the underlying
	// streams with a mutex to allow for concurrent writes.
	serverConn := tdp.MessageReadWriteCloser(newConn(p.wds, p.serverProtocol))
	clientConn := tdp.MessageReadWriteCloser(newConn(&WebsocketIO{Conn: p.ws}, p.clientProtocol))

	pinger := desktopPinger{
		// The pinger handles translation internally.
		server:           serverConn,
		client:           clientConn,
		latencySupported: latencySupported,
		ch:               make(chan []byte),
	}

	// The ping interceptor is installed on the server connection
	// regardless of whether or not translation is needed
	serverConn = tdp.NewReadWriteInterceptor(serverConn, pinger.intercept, nil)

	// Translation interceptors will be (optionally) installed in the *write* paths of each connection.
	needTranslation := p.clientProtocol != p.serverProtocol
	if needTranslation {
		// Translation is needed
		if p.serverProtocol == tdpb.ProtocolName {
			p.log.InfoContext(ctx, "Proxying desktop connection with translation", "server_dialect", tdpb.ProtocolName, "client_dialect", protocolTDP)
			// Agent speaks TDPB
			// Translate to TDPB when writing to the server. Intercept pings when reading from the server.
			serverConn = tdp.NewReadWriteInterceptor(serverConn, nil, tdpb.TranslateToModern)
			// Client speaks TDP
			// Translate to TDP (legacy) when writing to this connection
			clientConn = tdp.NewReadWriteInterceptor(clientConn, nil, tdpb.TranslateToLegacy)
		} else {
			p.log.InfoContext(ctx, "Proxying desktop connection with translation", "server_dialect", protocolTDP, "client_dialect", tdpb.ProtocolName)
			// Agent speaks TDP
			// Translate to TDPB when reading from this connection.
			serverConn = tdp.NewReadWriteInterceptor(serverConn, nil, tdpb.TranslateToLegacy)
			// The client speaks TDPB
			// Translate to TDPB (modern) when writing to this connection
			clientConn = tdp.NewReadWriteInterceptor(clientConn, nil, tdpb.TranslateToModern)
		}
	} else {
		p.log.InfoContext(ctx, "Proxying desktop connection without translation", "dialect", p.serverProtocol)
	}

	proxy := tdp.NewConnProxy(clientConn, serverConn)

	if latencySupported {
		// Default to TDPB
		pingerFunc := pinger.pingTDPB
		reportFunc := pinger.reportTDPB
		// Optionally use TDP versions
		if p.serverProtocol == protocolTDP {
			pingerFunc = pinger.pingTDP
		}
		if p.clientProtocol == protocolTDP {
			reportFunc = pinger.reportTDP
		}

		go monitorLatency(
			ctx,
			clockwork.NewRealClock(),
			p.ws,
			latency.PingerFunc(pingerFunc),
			latency.ReporterFunc(reportFunc),
		)
	}

	// Run joins and returns any read, write, or close errors from each side of the
	// connection proxy. We can inspect this singular error chain for any "real"
	// network errors (as opposed to errors that are expected from a normal teardown).
	err = proxy.Run()
	if utils.IsOKNetworkError(err) {
		err = nil
	}

	return trace.Wrap(err)
}

// handleDesktopWebsocketProxyErr handles the error returned by desktopWebsocketProxy by
// unwrapping it and determining whether to log an error.
func handleDesktopWebsocketProxyErr(ctx context.Context, proxyWsConnErr error, log *slog.Logger) {
	if proxyWsConnErr == nil {
		log.DebugContext(ctx, "desktopWebsocketProxy returned with no error")
		return
	}

	errs := []error{proxyWsConnErr}
	for len(errs) > 0 {
		err := errs[0] // pop first error
		errs = errs[1:]

		var aggregateErr trace.Aggregate
		var closeErr *websocket.CloseError
		switch {
		case errors.As(err, &aggregateErr):
			errs = append(errs, aggregateErr.Errors()...)
		case errors.As(err, &closeErr):
			switch closeErr.Code {
			case websocket.CloseNormalClosure, // when the user hits "disconnect" from the menu
				websocket.CloseGoingAway: // when the user closes the tab
				log.DebugContext(ctx, "Web socket closed by client", "close_code", closeErr.Code)
				return
			}
			return
		default:
			if wrapped := errors.Unwrap(err); wrapped != nil {
				errs = append(errs, wrapped)
			}
		}
	}

	log.WarnContext(ctx, "Error proxying a desktop protocol websocket to windows_desktop_service", "error", proxyWsConnErr)
}
