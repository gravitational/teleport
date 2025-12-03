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
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/diagnostics/latency"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	tdpbQueryParameter = "tdpb"
	tdpbVersionOne     = "teleport-tdpb-1.0"
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

// In summary, we need to:
//   - Receive initial client message(s)
//   - Cut certs and handle MFA (if required)
//   - Connect to Desktop agent and forward initial message(s)
//   - Proxy the connections (with translation if needed)
type MessageReadWriter interface {
	ReadMessage() (tdp.Message, error)
	WriteMessage(tdp.Message) error
}

// Implements MessageReadWriter and TDP read writer
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
		return err
	}
	return w.Conn.WriteMessage(websocket.BinaryMessage, data)
}

// Receive screenspec and keyboardlayout
func readTDPInitialMessages(ctx context.Context, rw MessageReadWriter, log *slog.Logger) (handshakeData, error) {
	msg, err := rw.ReadMessage()
	if err != nil {
		return handshakeData{}, trace.Wrap(err)
	}

	screenSpec, ok := msg.(tdp.ClientScreenSpec)
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

	keyboardLayout, gotKeyboardLayout := msg.(tdp.ClientKeyboardLayout)
	if !gotKeyboardLayout {
		log.InfoContext(ctx, "client did not send keyboard layout", "message_type", logutils.TypeAttr(msg))
	} else {
		data.keyboardLayout = &keyboardLayout
	}
	return data, nil
}

type handshakeInitializer struct {
	ClientHandshakeHandler func(ctx context.Context, rw MessageReadWriter, log *slog.Logger) (handshakeData, error)
	MFAPrompconstructor    MFAPromptconstructor
}

// Send upgrade. Ignore messages until Client Hello is received
func handleTDPUpgrade(ctx context.Context, rw MessageReadWriter, log *slog.Logger) (handshakeData, error) {
	upgrade := tdp.TDPUpgrade{Version: uint8(1)}
	err := rw.WriteMessage(upgrade)
	if err != nil {
		return handshakeData{}, trace.Wrap(err)
	}

	// Now wait patiently for the client to reply with a CLIENT_HELLO TDPB message
	// The ReadWriter implementation is expected to discard any legacy TDP messages
	// while waiting for the client hello.
	msg, err := rw.ReadMessage()
	hello := &tdpbv1.ClientHello{}
	if err = tdp.AsTDPB(msg, hello); err != nil {
		return handshakeData{}, trace.Wrap(err)
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
	screenSpec *tdp.ClientScreenSpec
	// May or may not be nil. Some web client versions will send this and
	// others will not.
	keyboardLayout *tdp.ClientKeyboardLayout

	// TDPB capable clients are required to send a hello
	hello *tdpbv1.ClientHello
}

// ForwardTDP forwards legacy TDP handshake messages (Username, ClientScreenSpec, KeyboardLayout (optional))
func (h *handshakeData) ForwardTDP(w io.Writer, username string, forwardKeyboardLayout bool) error {
	// Do we need to construct the screenspec from modern messages?
	if h.screenSpec == nil {
		h.screenSpec = &tdp.ClientScreenSpec{Width: h.hello.ScreenSpec.Width, Height: h.screenSpec.Height}
		h.keyboardLayout = &tdp.ClientKeyboardLayout{KeyboardLayout: h.keyboardLayout.KeyboardLayout}
	}

	messages := make([]tdp.Message, 0, 3)
	messages = append(messages, tdp.ClientUsername{Username: username}, h.screenSpec)
	if forwardKeyboardLayout && h.keyboardLayout != nil {
		messages = append(messages, h.keyboardLayout)
	}

	for _, msg := range messages {
		data, err := msg.Encode()
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}

// ForwardTDPB forwards the handshake data in the form of a TDPB CLIENT_HELLO message
func (h *handshakeData) ForwardTDPB(w io.Writer, username string) error {
	// Do we need to construct the hello from legacy messages?
	if h.hello == nil {
		h.hello = &tdpbv1.ClientHello{
			ScreenSpec: &tdpbv1.ClientScreenSpec{
				Width:  h.screenSpec.Width,
				Height: h.screenSpec.Height,
			},
			Username: username,
		}

		if h.keyboardLayout != nil {
			h.hello.KeyboardLayout = h.keyboardLayout.KeyboardLayout
		}
	}
	err := tdp.NewTDPBMessage(h.hello).EncodeTo(w)
	if err != nil {
		trace.Wrap(err)
	}
	return err
}

func SendTDPError(w MessageReadWriter, err error) error {
	if err != nil {
		slog.Warn("SendTDPError called with empty message")
		err = errors.New("")
	}

	err = w.WriteMessage(tdp.Alert{
		Message:  err.Error(),
		Severity: tdp.SeverityError,
	})
	return trace.Wrap(err)
}

func SendTDPBError(w MessageReadWriter, err error) error {
	if err != nil {
		slog.Warn("SendTDPBError called with empty message")
		err = errors.New("")
	}

	err = w.WriteMessage(tdp.NewTDPBMessage(&tdpbv1.Alert{
		Message:  err.Error(),
		Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR,
	}))
	return err
}

type MFAPromptconstructor func(string) mfa.PromptFunc

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
	// - if 'tdpb' query param is present, then we'll need to send an upgrade message to the client,
	//   then listen for a "CLIENT_HELLO" message (while discarding any TDP messages received).
	// - Otherwise fall back to the "legacy" behavior
	//
	// After either receiving a CLIENT_HELLO or our initial TDP messages, we can dial the server which
	// ALSO might speak TDP or TDPB. Unlike the client, the agent only speaks on or the other, so we'll
	// translate on its behalf.
	isTDPB := r.URL.Query().Get(tdpbQueryParameter) == tdpbQueryParameter
	withheld := []tdp.Message{}

	var init handshakeInitializer
	var adapter wsAdapter
	var sendError func(MessageReadWriter, error) error
	if isTDPB {
		log.DebugContext(ctx, "Creating Desktop connection for TDPB capable client")
		adapter = wsAdapter{Conn: ws, Decoder: func(r *websocket.Conn) (tdp.Message, error) {
			for {
				data, err := readWebSocketMessage(ws)
				switch {
				case err != nil:
					return nil, trace.Wrap(err)
				case len(data) < 1:
					return nil, errors.New("received empty message")
				case data[0] != 0:
					// "Legacy" TDP messages begin with non-zero first byte
					// discard any legacy TDP messages received
					continue
				default:
					msg, err := tdp.DecodeTDPB(bytes.NewReader(data))
					return &msg, trace.Wrap(err)
				}
			}
		}}

		sendError = SendTDPBError
		init = handshakeInitializer{
			ClientHandshakeHandler: handleTDPUpgrade,
			MFAPrompconstructor:    MFAPromptconstructor(tdp.NewTDPBMFAPrompt(&adapter, &withheld)),
		}
	} else {
		log.DebugContext(ctx, "Creating Desktop connection for legacy TDP client")
		adapter = wsAdapter{Conn: ws, Decoder: func(r *websocket.Conn) (tdp.Message, error) {
			data, err := readWebSocketMessage(ws)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			msg, err := tdp.Decode(data)
			return msg, trace.Wrap(err)
		}}

		sendError = SendTDPError
		init = handshakeInitializer{
			ClientHandshakeHandler: readTDPInitialMessages,
			MFAPrompconstructor:    tdp.NewTDPBMFAPrompt(&adapter, &withheld),
		}
	}
	// Handles either TDP upgrade or listens for TDPB client hello
	handshakeData, err := init.ClientHandshakeHandler(ctx, &adapter, log)

	username, err := readUsername(r)
	if err != nil {
		return sendError(&adapter, err)
	}
	log = log.With("username", username)

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
	certs, err := h.issueCerts(ctx, sctx, mfaRequired, certsReq, init.MFAPrompconstructor)
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
	// Now that we have a connection to the Windows Desktop Service, we can
	// forward the client_hello message (TDPB) or username and screen spec (TDP)
	// to the service, and any withheld messages that were received before the MFA
	// ceremony was completed.
	if alpnResult == tdpbVersionOne {
		err = handshakeData.ForwardTDPB(serviceConnTLS, username)
	} else {
		sendKeyboardLayout, _ := utils.MinVerWithoutPreRelease(version, "18.0.0")
		err = handshakeData.ForwardTDP(serviceConnTLS, username, sendKeyboardLayout)
	}

	// this blocks until the connection is closed
	handleProxyWebsocketConnErr(
		ctx,
		proxyWebsocketConn(ctx, ws, serviceConnTLS, version),
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
	promptConstructor MFAPromptconstructor,
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
	promptConstructor MFAPromptconstructor,
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

// desktopPinger measures latency between proxy and the desktop by sending tdp.Ping messages
// Windows Desktop Service and measuring the time it takes to receive message with the same UUID back.
type desktopPinger struct {
	server tdp.MessageWriter
	ch     <-chan tdp.Ping
}

func (d desktopPinger) Ping(ctx context.Context) error {
	ping := tdp.Ping{
		UUID: uuid.New(),
	}
	if err := d.server.WriteMessage(ping); err != nil {
		return trace.Wrap(err)
	}
	for {
		select {
		case pong := <-d.ch:
			if pong.UUID == ping.UUID {
				return nil
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

// proxyWebsocketConn does a bidrectional copy between the websocket
// connection to the browser (ws) and the mTLS connection to Windows
// Desktop Serivce (wds)
func proxyWebsocketConn(ctx context.Context, ws *websocket.Conn, wds *tls.Conn, version string) error {
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

	pings := make(chan tdp.Ping)
	serverReadInterceptor := func(msg tdp.Message) ([]tdp.Message, error) {
		if ping, ok := msg.(tdp.Ping); ok {
			if !latencySupported {
				return nil, trace.BadParameter("received unexpected Ping message from server (this is a bug)")
			}
			select {
			case pings <- ping:
			case <-ctx.Done():
			}
			return nil, nil
		}
		return []tdp.Message{msg}, nil
	}

	clientConn := tdp.NewConn(&WebsocketIO{Conn: ws})
	serverConn := tdp.NewConn(wds)
	proxy := tdp.NewConnProxy(clientConn, tdp.NewReadWriteInterceptor(serverConn, serverReadInterceptor, nil))
	if latencySupported {
		pinger := desktopPinger{
			server: serverConn,
			ch:     pings,
		}

		go monitorLatency(ctx, clockwork.NewRealClock(), ws, pinger,
			latency.ReporterFunc(func(ctx context.Context, stats latency.Statistics) error {
				return clientConn.WriteMessage(tdp.LatencyStats{
					ClientLatency: uint32(stats.Client),
					ServerLatency: uint32(stats.Server),
				})
			}),
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
