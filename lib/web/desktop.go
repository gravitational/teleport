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
	TDPB_QUERY_PARAMETER = "tdpb"
	TDPB_VERSION_ONE     = "teleport-tdpb-1.0"
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
	Write(p []byte) (n int, err error)
	Read() ([]byte, error)
	ReadMessage() (tdp.Message, error)
	WriteMessage(tdp.Message) error
}

// Implements MessageReadWriter and TDP read writer
type wsAdapter struct {
	Conn    *websocket.Conn
	Decoder func(io.Reader) (tdp.Message, error)
}

func (w *wsAdapter) Write(p []byte) (n int, err error) {
	return len(p), w.Conn.WriteMessage(websocket.BinaryMessage, p)
}

func (w *wsAdapter) Read() ([]byte, error) {
	mType, msg, err := w.Conn.ReadMessage()
	if mType != websocket.BinaryMessage {
		return nil, trace.BadParameter("received unexpected web socket message type %d", mType)
	}
	return msg, err
}

func (w *wsAdapter) ReadMessage() (tdp.Message, error) {
	mType, rdr, err := w.Conn.NextReader()
	if mType != websocket.BinaryMessage {
		return nil, trace.BadParameter("received unexpected web socket message type %d", mType)
	}
	if err != nil {
		return nil, err
	}
	return w.Decoder(rdr)
}

func (w *wsAdapter) WriteMessage(msg tdp.Message) error {
	wc, err := w.Conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return err
	}

	_, err = msg.WriteTo(wc)
	return errors.Join(err, wc.Close())
}

// 1. Check if this will be TDP or TDPB client connection
// 2. If TDP, no change from today (except setting up translation)
// 3. If TDPB, send the upgrade request and wait for a client hello
// 4. Eventually need a TDPB only handshake
func readTDPMessage(rw MessageReadWriter) (tdp.Message, error) {
	data, err := rw.Read()
	if err != nil {
		return nil, err
	}
	return tdp.Decode(data)
}

// Reads TDPB messages from the reader and discards TDP messages
// TDP messages are identified as any message whose first octet is non-zero
func readTDPBMessageWithDiscard(rw MessageReadWriter) (tdp.TdpbMessage, error) {
	for {
		data, err := rw.Read()
		if err != nil {
			return tdp.TdpbMessage{}, err
		}
		if len(data) < 1 {
			return tdp.TdpbMessage{}, errors.New("received empty message")
		}

		if data[0] == 0 {
			return tdp.DecodeTDPB(bytes.NewBuffer(data))
		}
	}
}

// Receive screenspec and keyboardlayout
func readTDPInitialMessages(ctx context.Context, rw MessageReadWriter, log *slog.Logger) (data handshakeData, errorFunc func(io.Writer, error) error, err error) {
	// Handle wrapping returned errors and sending TDP errors to the client
	errorFunc = SendTDPError
	defer func() {
		if err != nil {
			err = trace.Wrap(errors.Join(SendTDPError(rw, err)))
			return
		}
	}()

	// 1. READ TDP Screen spec
	msg, err := readTDPMessage(rw)
	if err != nil {
		return
	}
	screenSpec, ok := msg.(tdp.ClientScreenSpec)
	if !ok {
		err = trace.BadParameter("client sent unexpected message %T", msg)
		return
	}

	data.screenSpec = &screenSpec
	width, height := screenSpec.Width, screenSpec.Height
	if width > types.MaxRDPScreenWidth || height > types.MaxRDPScreenHeight {
		err = trace.BadParameter(
			"screen size of %d x %d is greater than the maximum allowed by RDP (%d x %d)",
			width, height, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight,
		)
		return
	}

	log = log.With("width", width, "height", height)
	msg, err = readTDPMessage(rw)
	if err != nil {
		return
	}

	keyboardLayout, gotKeyboardLayout := msg.(tdp.ClientKeyboardLayout)
	if !gotKeyboardLayout {
		log.InfoContext(ctx, "client did not send keyboard layout", "message_type", logutils.TypeAttr(msg))
		//withheld = append(withheld, msg)
	} else {
		data.keyboardLayout = &keyboardLayout
	}
	return
}

type handshakeInitializer struct {
	ClientHandshakeHandler func(ctx context.Context, rw MessageReadWriter, log *slog.Logger) (handshakeData, func(io.Writer, error) error, error)
	MFAPrompconstructor    MFAPrompconstructor
}

// Send upgrade. Ignore messages until Client Hello is received
func handleTDPUpgrade(_ context.Context, rw MessageReadWriter, _ *slog.Logger) (data handshakeData, errorFunc func(io.Writer, error) error, err error) {
	// Handle wrapping returned errors and sending TDP errors to the client
	errorFunc = SendTDPError
	defer func() {
		if err != nil {
			err = trace.Wrap(errors.Join(SendTDPError(rw, err)))
			return
		}
	}()
	// Send a TDP upgrade message.
	// Use a special TDPB decoder to read messages from the websocket while safely ignoring
	// and discarding any TDP messages received.
	upgrade := tdp.TDPUpgrade{Version: uint8(1)}
	var encoded []byte
	encoded, err = upgrade.Encode()
	if err != nil {
		return
	}

	_, err = rw.Write(encoded)
	if err != nil {
		return
	}

	// Now wait patiently for the client to reply with a CLIENT_HELLO TDPB message
	var msg tdp.TdpbMessage
	msg, err = readTDPBMessageWithDiscard(rw)
	if err != nil {
		return
	}
	protoMsg, err := msg.Proto()
	if err != nil {
		return
	}

	hello, isHello := protoMsg.(*tdpbv1.ClientHello)
	if !isHello {
		err = trace.Errorf("expected ClientHello message but got %T", protoMsg)
		return
	}

	// Switch errorFunc to TDPB
	errorFunc = SendTDPBError
	data.hello = hello
	return
}

// TDP -> TDPB (screenspec & keyboardlayout to translate client_hello)
// TDP -> TDP (forward messages)
// TDPB -> TDP (translate client_hello to screenspec & keyboardlayout)
// TDPB -> TDPB (forward hello)
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
func (h *handshakeData) ForwardTDPB(w io.Writer, username string, forwardKeyboardLayout bool) error {
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
	_, err := tdp.NewTDPBMessage(h.hello).WriteTo(w)
	return err
}

func SendTDPError(w io.Writer, err error) error {
	if err != nil {
		slog.Warn("SendTDPError called with empty message")
		err = errors.New("")
	}

	data, err := tdp.Alert{
		Message:  err.Error(),
		Severity: tdp.SeverityError,
	}.Encode()
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func SendTDPBError(w io.Writer, err error) error {
	if err != nil {
		slog.Warn("SendTDPBError called with empty message")
		err = errors.New("")
	}

	_, err = tdp.NewTDPBMessage(&tdpbv1.Alert{
		Message:  err.Error(),
		Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_ERROR,
	}).WriteTo(w)
	return err
}

type MFAPrompconstructor func(string) mfa.PromptFunc

// Write generic pre-connection handlers (receive server_hello, or screenspec & keyboard messages)
// write generic post-connect handlers (forward server_hello, or screenspec & keyboard messages)
// Write two generic implementations for performing MFA ceremony
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
	// - if 'tdpb' exists, we'll need to send an upgrade message to the client ASAP, then listen for
	//   a "CLIENT_HELLO" message (discard any TDP messages received) until then.
	// - Otherwise fall back to the "legacy" behavior
	//
	// After either receiving a CLIENT_HELLO or our initial TDP messages, we can dial the server which
	// ALSO might speak TDP or TDPB. Unlike the client, the agent only speaks on or the other, so we'll
	// translate here.
	isTDPB := r.URL.Query().Get(TDPB_QUERY_PARAMETER) == TDPB_QUERY_PARAMETER
	withheld := []tdp.Message{}

	var init handshakeInitializer
	var adapter wsAdapter
	if isTDPB {
		log.Info("Creating Desktop connection for TDPB capable client")
		adapter = wsAdapter{Conn: ws, Decoder: func(r io.Reader) (tdp.Message, error) {
			msg, err := tdp.DecodeTDPB(r)
			return &msg, err
		}}

		init = handshakeInitializer{
			ClientHandshakeHandler: handleTDPUpgrade,
			MFAPrompconstructor:    MFAPrompconstructor(tdp.NewTDPBMFAPrompt(&adapter, &withheld)),
		}
	} else {
		log.Info("Creating Desktop connection for legacy TDP client")
		adapter = wsAdapter{Conn: ws, Decoder: func(r io.Reader) (tdp.Message, error) {
			msg, err := tdp.DecodeFrom(r)
			return msg, err
		}}

		init = handshakeInitializer{
			ClientHandshakeHandler: readTDPInitialMessages,
			MFAPrompconstructor:    tdp.NewTDPBMFAPrompt(&adapter, &withheld),
		}
	}
	// Handles either TDP upgrade or listens for TDPB client hello
	handshakeData, sendError, err := init.ClientHandshakeHandler(ctx, &adapter, log)

	username, err := readUsername(r)
	if err != nil {
		return sendError(&adapter, err)
	}
	log = log.With("username", username)
	log.DebugContext(ctx, "Attempting to connect to desktop")

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
	// 3. OPTIONALLY HANDLE MFA (MUST HANDLE TDP OR TDPB SELECTION)
	certs, err := h.issueCerts(ctx, sctx, mfaRequired, certsReq, init.MFAPrompconstructor)
	if err != nil {
		return sendError(&adapter, err)
	}

	// Create a TLS config for connecting to the Windows Desktop Service.
	// 4. DIAL THE DESKTOP
	tlsConfig, err := h.createDesktopTLSConfig(ctx, sctx, desktopName, pk, certs)
	if err != nil {
		return sendError(&adapter, err)
	}

	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		return sendError(&adapter, err)
	}

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

	alpnResult := serviceConnTLS.ConnectionState().NegotiatedProtocol
	sendKeyboardLayout, _ := utils.MinVerWithoutPreRelease(version, "18.0.0")
	// Now that we have a connection to the Windows Desktop Service, we can
	// send the username and screen spec to the service, and any withheld
	// messages that were received before the MFA ceremony was completed.
	// 5. FORWARD CLIENT SCREEN SPEC AND OPTIONALLY KEYBOARDLAYOUT
	if alpnResult == TDPB_VERSION_ONE {
		err = handshakeData.ForwardTDPB(serviceConnTLS, username, sendKeyboardLayout)
	} else {
		err = handshakeData.ForwardTDP(serviceConnTLS, username, sendKeyboardLayout)
	}

	// this blocks until the connection is closed
	handleProxyWebsocketConnErr(
		ctx,
		proxyWebsocketConn(ctx, ws, serviceConnTLS, log, version),
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
	promptConstructor MFAPrompconstructor,
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
	promptConstructor MFAPrompconstructor,
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
		//{
		//return
		//func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
		//// Convert from proto to JSON types.
		//var challenge client.MFAAuthenticateChallenge
		//if chal.WebauthnChallenge != nil {
		//	challenge.WebauthnChallenge = wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge)
		//}
		//
		//if chal.SSOChallenge != nil {
		//	challenge.SSOChallenge = client.SSOChallengeFromProto(chal.SSOChallenge)
		//	challenge.SSOChallenge.ChannelID = channelID
		//}
		//
		//if chal.WebauthnChallenge == nil && chal.SSOChallenge == nil {
		//	return nil, trace.Wrap(authclient.ErrNoMFADevices)
		//}
		//
		//// Send the challenge over the socket.
		//var codec tdpMFACodec
		//msg, err := codec.Encode(&challenge, defaults.WebsocketMFAChallenge)
		//if err != nil {
		//	return nil, trace.Wrap(err)
		//}
		//
		//if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		//	return nil, trace.Wrap(err)
		//}
		//
		//// Special case: if we've already received an MFA response (because an old web UI
		//// that doesn't send the keyboard layout is connected), then we're done.
		//if len(*withheld) > 0 {
		//	mfaResp, ok := (*withheld)[0].(*tdp.MFA)
		//	if ok {
		//		return mfaResp.MFAAuthenticateResponse, nil
		//	}
		//}
		//
		//span.AddEvent("waiting for user to complete mfa ceremony")
		//var buf []byte
		//// Loop through incoming messages until we receive an MFA message that lets us
		//// complete the ceremony. Non-MFA messages (e.g. ClientScreenSpecs representing
		//// screen resizes) are withheld for later.
		//for {
		//	var ty int
		//	ty, buf, err = ws.ReadMessage()
		//	if err != nil {
		//		return nil, trace.Wrap(err)
		//	}
		//	if ty != websocket.BinaryMessage {
		//		return nil, trace.BadParameter("received unexpected web socket message type %d", ty)
		//	}
		//	if len(buf) == 0 {
		//		return nil, trace.BadParameter("empty message received")
		//	}
		//
		//	if tdp.MessageType(buf[0]) != tdp.TypeMFA {
		//		// This is not an MFA message, withhold it for later.
		//		msg, err := tdp.Decode(buf)
		//		h.logger.DebugContext(ctx, "Received non-MFA message, withholding", "msg_type", logutils.TypeAttr(msg))
		//		if err != nil {
		//			return nil, trace.Wrap(err)
		//		}
		//		*withheld = append(*withheld, msg)
		//		continue
		//	}
		//
		//	break
		//}
		//
		//assertion, err := codec.DecodeResponse(buf, defaults.WebsocketMFAChallenge)
		//if err != nil {
		//	return nil, trace.Wrap(err)
		//}
		//span.AddEvent("mfa ceremony completed")
		//
		//return assertion, nil
		//})
		//},
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
	proxy *tdp.ConnProxy
	ch    <-chan tdp.Ping
}

func (d desktopPinger) Ping(ctx context.Context) error {
	ping := tdp.Ping{
		UUID: uuid.New(),
	}
	if err := d.proxy.SendToServer(ping); err != nil {
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
func proxyWebsocketConn(ctx context.Context, ws *websocket.Conn, wds *tls.Conn, log *slog.Logger, version string) error {
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

	tdpConnProxy := tdp.NewConnProxy(&WebsocketIO{Conn: ws}, wds, func(_ *tdp.Conn, msg tdp.Message) (tdp.Message, error) {
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
		return msg, nil
	})

	if latencySupported {
		pinger := desktopPinger{
			proxy: tdpConnProxy,
			ch:    pings,
		}

		go monitorLatency(ctx, clockwork.NewRealClock(), ws, pinger,
			latency.ReporterFunc(func(ctx context.Context, stats latency.Statistics) error {
				return trace.Wrap(tdpConnProxy.SendToClient(tdp.LatencyStats{
					ClientLatency: uint32(stats.Client),
					ServerLatency: uint32(stats.Server),
				}))
			}),
		)

	}

	return trace.Wrap(tdpConnProxy.Run())
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

// sendTDPAlert sends a tdp Notification over the supplied websocket with the
// error message of err.
//func sendTDPAlert(ws *websocket.Conn, err error, severity tdp.Severity) error {
//	msg := tdp.Alert{Message: err.Error(), Severity: severity}
//	b, err := msg.Encode()
//	if err != nil {
//		return trace.Wrap(err)
//	}
//	return ws.WriteMessage(websocket.BinaryMessage, b)
//}
