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
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/defaults"
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
	protocolTDPB       = "teleport-tdpb-1.0"
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

// Implements legacy.MessageReadWriter
type wsAdapter struct {
	Conn *websocket.Conn
	// Determines how ReadMessage will interpret incoming datagrams
	// (TDP or TDPB)
	Decoder func(*websocket.Conn) (tdp.Message, error)
}

func (w *wsAdapter) ReadMessage() (tdp.Message, error) {
	return w.Decoder(w.Conn)
}

func (w *wsAdapter) WriteMessage(msg tdp.Message) error {
	data, err := msg.Encode()
	if err != nil {
		return trace.Wrap(err)
	}
	return w.Conn.WriteMessage(websocket.BinaryMessage, data)
}

// Receive screenspec and keyboardlayout
func readTDPInitialMessages(ctx context.Context, rw tdp.MessageReadWriter, log *slog.Logger) (handshakeData, error) {
	msg, err := rw.ReadMessage()
	if err != nil {
		return handshakeData{}, trace.Wrap(err)
	}

	screenSpec, ok := msg.(legacy.ClientScreenSpec)
	if !ok {
		return handshakeData{}, trace.BadParameter("client sent unexpected message %T", msg)
	}

	data := handshakeData{
		screenSpec: &screenSpec,
	}

	width, height := screenSpec.Width, screenSpec.Height
	if width > types.MaxRDPScreenWidth || height > types.MaxRDPScreenHeight {
		return handshakeData{}, trace.BadParameter(
			"screen size of %d x %d is greater than the maximum allowed by RDP (%d x %d)",
			width, height, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight,
		)
	}

	log = log.With("width", width, "height", height)
	msg, err = rw.ReadMessage() //readTDPMessage(rw)
	if err != nil {
		return handshakeData{}, trace.Wrap(err)
	}

	keyboardLayout, gotKeyboardLayout := msg.(legacy.ClientKeyboardLayout)
	if !gotKeyboardLayout {
		log.InfoContext(ctx, "client did not send keyboard layout", "message_type", logutils.TypeAttr(msg))
	} else {
		data.keyboardLayout = &keyboardLayout
	}
	return data, nil
}

type handshakeInitializer struct {
	clientHandshakeHandler func(ctx context.Context, rw tdp.MessageReadWriter, log *slog.Logger) (handshakeData, error)
	promptBuilder          mfaPromptBuilder
}

// Send upgrade. Ignore messages until Client Hello is received
func handleTDPUpgrade(ctx context.Context, rw tdp.MessageReadWriter, log *slog.Logger) (handshakeData, error) {
	upgrade := legacy.TDPUpgrade{Version: uint8(1)}
	err := rw.WriteMessage(upgrade)
	if err != nil {
		return handshakeData{}, trace.Wrap(err)
	}

	// Now wait patiently for the client to reply with a CLIENT_HELLO TDPB message
	// The ReadWriter implementation is expected to discard any legacy TDP messages
	// while waiting for the client hello.
	msg, err := rw.ReadMessage()
	if err != nil {
		return handshakeData{}, trace.Wrap(err)
	}
	hello, ok := msg.(*tdpb.ClientHello)
	if !ok {
		return handshakeData{}, trace.Errorf("Expected client hello message but got %T", msg)
	}

	log.InfoContext(ctx, "Received client hello message", "message", hello)
	return handshakeData{hello: hello}, nil
}

// handshakeData handles translation of "handshake" messages
// between TDP and TDPB.
type handshakeData struct {
	// Think of this as a union of TDP handshake data and
	// TDPB handshake data.
	// As an invariant we expect that
	// ClientScreenSpec != nil XOR ClientHello != nil
	screenSpec *legacy.ClientScreenSpec
	// May or may not be nil. Some web client versions will send this and
	// others will not.
	keyboardLayout *legacy.ClientKeyboardLayout

	// TDPB capable clients are required to send a hello
	hello *tdpb.ClientHello
}

// ForwardTDP forwards legacy TDP handshake messages (Username, ClientScreenSpec, KeyboardLayout (optional))
func (h *handshakeData) ForwardTDP(w io.Writer, username string, forwardKeyboardLayout bool) error {
	// Do we need to construct the screenspec from modern messages?
	if h.screenSpec == nil {
		if h.hello != nil {
			if h.hello.ScreenSpec != nil {
				h.screenSpec = &legacy.ClientScreenSpec{Width: h.hello.ScreenSpec.Width, Height: h.hello.ScreenSpec.Height}
			} else {
				return trace.Errorf("Client Hello does not contain required ScreenSpec field")
			}
		} else {
			return trace.Errorf("No client screen spec nor client hello message reeived. Cannot complete TDP/TDPB handhsake")
		}
		h.keyboardLayout = &legacy.ClientKeyboardLayout{KeyboardLayout: h.hello.KeyboardLayout}
	}

	messages := make([]tdp.Message, 0, 3)
	messages = append(messages, legacy.ClientUsername{Username: username}, h.screenSpec)
	if forwardKeyboardLayout && h.keyboardLayout != nil {
		messages = append(messages, h.keyboardLayout)
	}

	for _, msg := range messages {
		data, err := msg.Encode()
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = w.Write(data)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ForwardTDPB forwards the handshake data in the form of a TDPB CLIENT_HELLO message
func (h *handshakeData) ForwardTDPB(w io.Writer, username string) error {
	// Do we need to construct the hello from legacy messages?
	if h.hello == nil {
		if h.screenSpec == nil {
			return trace.Errorf("Received neither a client hello nor client screenspec messages. Cannot forward TDPB handshake data")
		}
		h.hello = &tdpb.ClientHello{
			ScreenSpec: &tdpbv1.ClientScreenSpec{
				Width:  h.screenSpec.Width,
				Height: h.screenSpec.Height,
			},
		}

		if h.keyboardLayout != nil {
			h.hello.KeyboardLayout = h.keyboardLayout.KeyboardLayout
		}
	}
	h.hello.Username = username

	return trace.Wrap(tdp.EncodeTo(w, h.hello))
}

func sendTDPError(w tdp.MessageReadWriter, err error) error {
	if err == nil {
		slog.Warn("SendTDPError called with empty message")
		err = trace.Errorf("undefined error")
	}

	err = w.WriteMessage(legacy.Alert{
		Message:  err.Error(),
		Severity: legacy.SeverityError,
	})
	return trace.Wrap(err)
}

func sendTDPBError(w tdp.MessageReadWriter, err error) error {
	if err == nil {
		slog.Warn("SendTDPBError called with empty message")
		err = errors.New("")
	}

	err = w.WriteMessage((&tdpb.Alert{
		Message:  err.Error(),
		Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR,
	}))
	return trace.Wrap(err)
}

type mfaPromptBuilder func(string) mfa.PromptFunc

func readWebSocketMessage(ws *websocket.Conn) ([]byte, error) {
	mType, data, err := ws.ReadMessage()
	if err != nil {
		return nil, err
	}

	if mType != websocket.BinaryMessage {
		return nil, trace.BadParameter("received unexpected web socket message type %d", mType)
	}
	return data, nil
}

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

	// Client may speak TDP or TDPB. We'll know based on the existence of the 'tdpb' query parameter
	// - If 'tdpb' query param is present, then we'll need to send an upgrade message to the client,
	//   then listen for a "CLIENT_HELLO" message (while discarding any TDP messages received).
	//   Note: We *always* upgrade the client connection to TDPB if possible.
	// - Otherwise fall back to the "legacy" behavior
	//
	// After either receiving a CLIENT_HELLO or our initial TDP messages, we can dial the server which
	// ALSO might speak TDP or TDPB. Unlike the client, the agent only speaks on or the other, so we'll
	// translate on its behalf if needed.
	clientProtocol, err := readClientProtocol(r)
	if err != nil {
		log.ErrorContext(ctx, "Error reading client desktop protocol", "error", err)
		return trace.Wrap(err)
	}
	withheld := []tdp.Message{}

	// Initialize a few utilties that will allow us to create the desktop connection
	// for either a TDP or TDPB client.
	var init handshakeInitializer
	var adapter wsAdapter
	var sendError func(tdp.MessageReadWriter, error) error
	if clientProtocol == protocolTDPB {
		log.DebugContext(ctx, "Creating Desktop connection for TDPB capable client")
		adapter = wsAdapter{Conn: ws, Decoder: func(r *websocket.Conn) (tdp.Message, error) {
			for {
				data, err := readWebSocketMessage(ws)
				switch {
				case err != nil:
					return nil, trace.Wrap(err)
				case len(data) < 1:
					return nil, trace.Errorf("received empty message")
				case data[0] != 0:
					// "Legacy" TDP messages begin with non-zero first byte
					// discard any legacy TDP messages received
					continue
				default:
					msg, err := tdpb.Decode(bytes.NewReader(data))
					return msg, trace.Wrap(err)
				}
			}
		}}

		sendError = sendTDPBError
		init = handshakeInitializer{
			clientHandshakeHandler: handleTDPUpgrade,
			promptBuilder:          mfaPromptBuilder(newTDPBMFAPrompt(&adapter, &withheld)),
		}
	} else {
		log.DebugContext(ctx, "Creating Desktop connection for legacy TDP client")
		adapter = wsAdapter{Conn: ws, Decoder: func(r *websocket.Conn) (tdp.Message, error) {
			data, err := readWebSocketMessage(ws)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			msg, err := legacy.Decode(bytes.NewBuffer(data))
			return msg, trace.Wrap(err)
		}}

		sendError = sendTDPError
		init = handshakeInitializer{
			clientHandshakeHandler: readTDPInitialMessages,
			promptBuilder:          newTDPMFAPrompt(&adapter, &withheld),
		}
	}
	// Read the initial set of TDP messages, or handle TDP upgrade and subsequent
	// Client Hello message.
	handshakeData, err := init.clientHandshakeHandler(ctx, &adapter, log)

	username, err := readUsername(r)
	if err != nil {
		return sendError(&adapter, err)
	}

	// Parse the private key of the user from the session context.
	pk, err := keys.ParsePrivateKey(sctx.cfg.Session.GetTLSPriv())
	if err != nil {
		return sendError(&adapter, err)
	}

	// Check if MFA is required and create a UserCertsRequest.
	mfaRequired, certsReq, err := h.prepareForCertIssuance(ctx, sctx, cluster, pk.Public(), desktopName, username)
	if err != nil {
		return sendError(&adapter, err)
	}

	// Issue certificate for the user/desktop combination and perform MFA ceremony if required.
	certs, err := h.issueCerts(ctx, sctx, mfaRequired, certsReq, init.promptBuilder)
	if err != nil {
		return sendError(&adapter, err)
	}

	// Create a TLS config for connecting to the Windows Desktop Service.
	tlsConfig, err := h.createDesktopTLSConfig(ctx, sctx, desktopName, pk, certs)
	if err != nil {
		return sendError(&adapter, err)
	}

	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		return sendError(&adapter, err)
	}

	log.DebugContext(ctx, "Attempting to connect to desktop")
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
		return sendError(&adapter, (trace.Wrap(err, "cannot connect to Windows Desktop Service")))
	}
	defer serviceConn.Close()

	serviceConnTLS := tls.Client(serviceConn, tlsConfig)

	if err := serviceConnTLS.HandshakeContext(ctx); err != nil {
		return sendError(&adapter, err)
	}
	log.DebugContext(ctx, "Connected to windows_desktop_service")

	// ALPN informs us which dialect the server will be using.
	alpnResult := serviceConnTLS.ConnectionState().NegotiatedProtocol
	log.InfoContext(ctx, "ALPN", "result", alpnResult)
	switch alpnResult {
	case "":
		alpnResult = protocolTDP
	case protocolTDPB:
		// Intentionally empty
	default:
		return trace.Errorf("unknown desktop agent protocol")
	}
	// Now that we have a connection to the Windows Desktop Service, we can
	// forward the client_hello message (TDPB) or username and screen spec (TDP)
	// to the service, and any withheld messages that were received before the MFA
	// ceremony was completed.
	if alpnResult == protocolTDPB {
		log.InfoContext(ctx, "Desktop Service negotiated TDPB")
		err = handshakeData.ForwardTDPB(serviceConnTLS, username)
	} else {
		log.InfoContext(ctx, "Desktop Service negotiated TDP")
		sendKeyboardLayout, _ := utils.MinVerWithoutPreRelease(version, "18.0.0")
		err = handshakeData.ForwardTDP(serviceConnTLS, username, sendKeyboardLayout)
	}

	// Forward any withheld messages during MFA
	for _, msg := range withheld {
		if err := tdp.EncodeTo(serviceConnTLS, msg); err != nil {
			err = trace.WrapWithMessage(err, "error forwarding message to desktop agent")
			sendError(&adapter, err)
			return err
		}
	}

	// this blocks until the connection is closed
	handleProxyWebsocketConnErr(
		ctx,
		proxyWebsocketConn(ctx, ws, serviceConnTLS, version, clientProtocol, alpnResult),
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
	tlsConfig.NextProtos = []string{protocolTDPB}
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
			return mfa.PromptFunc(promptConstructor(channelID))
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
	case protocolTDPB:
		return protocolTDPB, nil
	default:
		return "", trace.Errorf("unknown TDPB version '%s'", tdpbVersion)
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
		slog.Warn("received unexpected Ping message from server (this is a bug)")
		// Swallow the ping message, but there's no need to return an error
		// (which will probably kill the connection)
		return nil, nil
	}

	d.ch <- uuid
	// We've handled the ping. Do not pass it along to the proxy.
	return nil, nil

}

func (d desktopPinger) ping(ctx context.Context, msg tdp.Message) error {
	if err := d.server.WriteMessage(msg); err != nil {
		return trace.Wrap(err)
	}
	for {
		select {
		case pong := <-d.ch:
			if bytes.Equal(pong, pong) {
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
	return d.ping(ctx, legacy.Ping{UUID: uuid.New()})
}

func (d desktopPinger) pingTDPB(ctx context.Context) error {
	uuid := uuid.New()
	return d.ping(ctx, &tdpb.Ping{
		Uuid: uuid[:],
	})
}

func newConn(rwc io.ReadWriteCloser, protocol string) *tdp.Conn {
	if protocol == protocolTDPB {
		return tdp.NewConn(rwc, tdp.DecoderAdapter(tdpb.Decode))
	}
	return tdp.NewConn(rwc, legacy.Decode)
}

// proxyWebsocketConn does a bidrectional copy between the websocket
// connection to the browser (ws) and the mTLS connection to Windows
// Desktop Serivce (wds)
func proxyWebsocketConn(ctx context.Context, ws *websocket.Conn, wds net.Conn, version string, clientProtocol, serverProtocol string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		ws.Close()
		wds.Close()
	}()

	latencySupported, err := utils.MinVerWithoutPreRelease(version, "17.5.0")
	if err != nil {
		return trace.Wrap(err)
	}

	// Create a single pair of legacy.Conn instances. legacy.Conn protects the underlying
	// streams with a mutex to allow for concurrent writes.
	serverConn := tdp.MessageReadWriteCloser(newConn(wds, serverProtocol))
	clientConn := tdp.MessageReadWriteCloser(newConn(&WebsocketIO{Conn: ws}, clientProtocol))

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
	needTranslation := clientProtocol != serverProtocol
	if needTranslation {
		// Translation is needed
		if serverProtocol == protocolTDPB {
			slog.InfoContext(ctx, "Proxying desktop connection with translation", "server_dialect", protocolTDPB, "client_dialect", protocolTDP)
			// Server speaks TDPB
			// Translate to TDPB when writing to the server. Intercept pings when reading from the server.
			serverConn = tdp.NewReadWriteInterceptor(serverConn, nil, tdpb.TranslateToModern)
			// Client speaks TDP
			// Translate to TDP (legacy) when writing to this connection
			clientConn = tdp.NewReadWriteInterceptor(clientConn, nil, tdpb.TranslateToLegacy)
		} else {
			slog.InfoContext(ctx, "Proxying desktop connection with translation", "server_dialect", protocolTDP, "client_dialect", protocolTDPB)
			// Server speaks TDP
			// Translate to TDPB when reading from this connection.
			serverConn = tdp.NewReadWriteInterceptor(serverConn, nil, tdpb.TranslateToLegacy)
			// The client speaks TDPB
			// Translate to TDPB (modern) when writing to this connection
			clientConn = tdp.NewReadWriteInterceptor(clientConn, nil, tdpb.TranslateToModern)
		}
	} else {
		slog.InfoContext(ctx, "Proxying desktop connection without translation", "dialect", serverProtocol)
	}

	proxy := tdp.NewConnProxy(clientConn, serverConn)

	if latencySupported {
		// Default to TDPB
		pingerFunc := pinger.pingTDPB
		reportFunc := pinger.reportTDPB
		// Optionally use TDP versions
		if serverProtocol == protocolTDP {
			pingerFunc = pinger.pingTDP
		}
		if clientProtocol == protocolTDP {
			reportFunc = pinger.reportTDP
		}

		go monitorLatency(
			ctx,
			clockwork.NewRealClock(),
			ws,
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

// handleProxyWebsocketConnErr handles the error returned by proxyWebsocketConn by
// unwrapping it and determining whether to log an error.
func handleProxyWebsocketConnErr(ctx context.Context, proxyWsConnErr error, log *slog.Logger) {
	if proxyWsConnErr == nil {
		log.DebugContext(ctx, "proxyWebsocketConn returned with no error")
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

var (
	errUnexpectedMessageType = errors.New("unexpected message type")
)

// convertChallenge converts an MFA challenge to a Message. Returns
// a non-nil error if the conversion fails
type convertChallenge func(*proto.MFAAuthenticateChallenge) (tdp.Message, error)

// isMFAResponse returns:
//   - ErrUnexpectedMessageType if a valid messages was received but was not an MFA message.
//   - Any other non-nil error if there was an error intepreeting the message.
//   - nil if a valid, non-nil MFA messages was found.
type isMFAResponse func(tdp.Message) (*proto.MFAAuthenticateResponse, error)

// newMfaPrompt constructs a function that reads, encodes, and sends an MFA challenge to the client,
// then waits for the corresponding MFA response message. It caches any non-MFA messages received so
// that they may be forwarded to the server later on.
func newMfaPrompt(rw tdp.MessageReadWriter, isResponse isMFAResponse, toMessage convertChallenge, withheld *[]tdp.Message) mfa.PromptFunc {
	return func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
		challengeMsg, err := toMessage(chal)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		slog.DebugContext(ctx, "Writing MFA challenge to client")
		if err = rw.WriteMessage(challengeMsg); err != nil {
			return nil, trace.Wrap(err)
		}

		for {
			msg, err := rw.ReadMessage()
			if err != nil {
				return nil, trace.Wrap(err)
			}

			resp, err := isResponse(msg)
			if err != nil {
				if errors.Is(err, errUnexpectedMessageType) {
					// Withhold this non-MFA message and try reading again
					slog.DebugContext(ctx, "Received non-MFA message", "message", msg)
					*withheld = append(*withheld, msg)
					continue
				} else {
					slog.DebugContext(ctx, "Error receiving MFA response", "error", err)
					// Unexpected error occurred while inspecting the message
					return nil, trace.Wrap(err)
				}
			}
			// Found our MFA response!
			slog.DebugContext(ctx, "Received MFA response")
			return resp, nil
		}
	}
}

// Handle TDP MFA ceremony
func newTDPMFAPrompt(rw tdp.MessageReadWriter, withheld *[]tdp.Message) func(string) mfa.PromptFunc {
	return func(channelID string) mfa.PromptFunc {
		convert := func(chal *proto.MFAAuthenticateChallenge) (tdp.Message, error) {
			// Convert from proto to JSON types.
			var challenge client.MFAAuthenticateChallenge
			if chal.WebauthnChallenge != nil {
				challenge.WebauthnChallenge = wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge)
			}

			if chal.SSOChallenge != nil {
				challenge.SSOChallenge = client.SSOChallengeFromProto(chal.SSOChallenge)
				challenge.SSOChallenge.ChannelID = channelID
			}

			if chal.WebauthnChallenge == nil && chal.SSOChallenge == nil && chal.TOTP == nil {
				return nil, trace.Wrap(authclient.ErrNoMFADevices)
			}

			tdpMsg := &legacy.MFA{
				Type:                     defaults.WebsocketMFAChallenge[0],
				MFAAuthenticateChallenge: &challenge,
			}
			return tdpMsg, nil
		}

		isResponse := func(msg tdp.Message) (*proto.MFAAuthenticateResponse, error) {
			switch t := msg.(type) {
			case *legacy.MFA:
				return t.MFAAuthenticateResponse, nil
			default:
				return nil, errUnexpectedMessageType
			}
		}

		return newMfaPrompt(rw, isResponse, convert, withheld)
	}
}

// Handle TDPB MFA ceremony
func newTDPBMFAPrompt(rw tdp.MessageReadWriter, withheld *[]tdp.Message) func(string) mfa.PromptFunc {
	return func(channelID string) mfa.PromptFunc {
		convert := func(challenge *proto.MFAAuthenticateChallenge) (tdp.Message, error) {
			if challenge == nil {
				return nil, errors.New("empty MFA challenge")
			}

			mfaMsg := &tdpb.MFA{
				ChannelId: channelID,
			}

			if challenge.WebauthnChallenge != nil {
				mfaMsg.Challenge = &mfav1.AuthenticateChallenge{
					WebauthnChallenge: challenge.WebauthnChallenge,
				}
			}

			if challenge.SSOChallenge != nil {
				mfaMsg.Challenge = &mfav1.AuthenticateChallenge{
					SsoChallenge: &mfav1.SSOChallenge{
						RequestId:   challenge.SSOChallenge.RequestId,
						RedirectUrl: challenge.SSOChallenge.RedirectUrl,
						Device:      challenge.SSOChallenge.Device,
					},
				}
			}

			if challenge.WebauthnChallenge == nil && challenge.SSOChallenge == nil && challenge.TOTP == nil {
				return nil, trace.Wrap(authclient.ErrNoMFADevices)
			}

			return mfaMsg, nil
		}

		isResponse := func(msg tdp.Message) (*proto.MFAAuthenticateResponse, error) {
			mfaMsg, ok := msg.(*tdpb.MFA)
			if !ok {
				return nil, errUnexpectedMessageType
			}

			if mfaMsg.AuthenticationResponse == nil {
				return nil, trace.Errorf("MFA response is empty")
			}

			switch response := mfaMsg.AuthenticationResponse.Response.(type) {
			case *mfav1.AuthenticateResponse_Sso:
				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_SSO{
						SSO: &proto.SSOResponse{
							RequestId: response.Sso.RequestId,
							Token:     response.Sso.Token,
						},
					},
				}, nil
			case *mfav1.AuthenticateResponse_Webauthn:
				return &proto.MFAAuthenticateResponse{
					Response: &proto.MFAAuthenticateResponse_Webauthn{
						Webauthn: response.Webauthn,
					},
				}, nil
			default:
				return nil, trace.Errorf("Unexpected MFA response type %T", mfaMsg.AuthenticationResponse)
			}
		}

		return newMfaPrompt(rw, isResponse, convert, withheld)
	}
}
