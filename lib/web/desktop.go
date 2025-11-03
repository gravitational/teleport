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
	"bufio"
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"errors"
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
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	tdpb "github.com/gravitational/teleport/desktop"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/desktop"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/diagnostics/latency"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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

type readMessageFunc func() (tdp.Message, error)
type writeMessageFunc func(tdp.Message) error

// represents the initial set of messages
// received by the client.
type clientInitdata struct {
	// TDP
	screenSpec     *tdp.ClientScreenSpec
	keyboardLayout *tdp.ClientKeyboardLayout
	username       string
	// TDPB
	hello *tdpb.ClientHello
}

func (c *clientInitdata) tdpb() []tdp.Message {
	return []tdp.Message{
		&tdp.TDPBMessage{
			// TODO
			Message: &tdpb.ClientHello{},
		},
	}
}

func (c *clientInitdata) tdp() []tdp.Message {
	out := []tdp.Message{}
	out = append(out, tdp.ClientUsername{
		Username: c.username,
	})
	out = append(out, tdp.ClientScreenSpec{
		Width:  c.GetScreenWidth(),
		Height: c.GetScreenHeight(),
	})
	if ok, val := c.GetKeyboardLayout(); ok {
		out = append(out, tdp.ClientKeyboardLayout{
			KeyboardLayout: val,
		})
	}
	return out
}

func (c *clientInitdata) GetScreenWidth() uint32 {
	if c.screenSpec != nil {
		return c.screenSpec.Width
	}
	return 0
}

func (c *clientInitdata) GetScreenHeight() uint32 {
	if c.screenSpec != nil {
		return c.screenSpec.Height
	}
	return 0
}

func (c *clientInitdata) GetKeyboardLayout() (bool, uint32) {
	if c.keyboardLayout != nil {
		return true, c.keyboardLayout.KeyboardLayout
	}
	return false, 0
}

type clientInitExchange struct {
	reader readMessageFunc
	writer writeMessageFunc
	// dialect in use by the client
	dialect string
	// witheld messages in the client's native dialect
	withheld []tdp.Message

	data clientInitdata
}

func (c *clientInitExchange) ForwardClientHello(dialect string) []tdp.Message {
	// Server dialect
	switch dialect {
	case "":
		return c.data.tdp()
	default: /* TDP */
		return c.data.tdpb()
	}
}

// Reads either client hello or screenspec / keyboard layout
func (c *clientInitExchange) ReadClientInit(ctx context.Context) (*clientInitdata, error) {
	switch c.dialect {
	case "tdpb/1.0":
		data, err := c.handleClientHandshakeTDPB(ctx, c.reader)
		c.data = *data
		return data, err
	default: /* TDP */
		data, err := c.handleClientHandshakeTDP(ctx, c.reader)
		c.data = *data
		return data, err
	}
}

func (c *clientInitExchange) sendTDPAlert(err error) error {
	var msg tdp.Message
	switch c.dialect {
	case "tdpb/1.0":
		msg = &tdp.TDPBMessage{Message: &tdpb.Alert{Message: err.Error(), Severseverity: tdpb.AlertSeverity_ALERT_SEVERITY_ERROR}}
	default:
		msg = tdp.Alert{Message: err.Error(), Severity: tdp.SeverityError}
	}
	return c.writer(msg)
}

func (c *clientInitExchange) sendTDPError(err error) error {
	var msg tdp.Message
	switch c.dialect {
	case "tdpb/1.0":
		slog.Warn("sending error in TDPB")
		msg = &tdp.TDPBMessage{Message: &tdpb.Error{Message: err.Error()}}
	default:
		slog.Warn("sending error in TDP")
		msg = tdp.Error{Message: err.Error()}
	}
	return c.writer(msg)
}

func (c *clientInitExchange) SendChallenge(challenge client.MFAAuthenticateChallenge, n string) error {
	return nil
}
func (c *clientInitExchange) ReceiveAssertion() (*proto.MFAAuthenticateResponse, error) {
	return nil, nil
}

// Manages sending/receiving MFA challenge to the desktop client
//type mfaHandler struct {
//	reader readMessageFunc
//	writer writeMessageFunc
//}
//
//func (m *mfaHandler) SendChallenge(challenge client.MFAAuthenticateChallenge, n string) error {
//	return nil
//}
//func (m *mfaHandler) ReceiveAssertion() (*proto.MFAAuthenticateResponse, error) {
//	return nil, nil
//}

// Listen for sceen spec, and keyboard layout
func (c *clientInitExchange) handleClientHandshakeTDP(ctx context.Context, readClientMessage func() (tdp.Message, error)) (*clientInitdata, error) {
	// The first thing we expect from the client is the screen spec.
	msg, err := readClientMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	screenSpec, ok := msg.(*tdp.ClientScreenSpec)
	if !ok {
		return nil, trace.Errorf("client sent unexpected message %T", msg) //sendTDPError(trace.BadParameter("client sent unexpected message %T", msg))
	}

	width, height := screenSpec.Width, screenSpec.Height
	if width > types.MaxRDPScreenWidth || height > types.MaxRDPScreenHeight {
		return nil, trace.Errorf("screen size of %d x %d is greater than the maximum allowed by RDP (%d x %d)",
			width, height, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight)
	}

	// Try to read the keyboard layout, which is sent by v18+ clients.
	msg, err = readClientMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyboardLayout, gotKeyboardLayout := msg.(*tdp.ClientKeyboardLayout)
	if !gotKeyboardLayout {
		slog.InfoContext(ctx, "client did not send keyboard layout", "message_type", logutils.TypeAttr(msg))
		c.withheld = append(c.withheld, msg)
	}

	return &clientInitdata{
		screenSpec:     screenSpec,
		keyboardLayout: keyboardLayout,
	}, nil

}

func (c *clientInitExchange) handleClientHandshakeTDPB(ctx context.Context, readClientMessage func() (tdp.Message, error)) (*clientInitdata, error) {
	// The first thing we expect from the client is the screen spec.
	msg, err := readClientMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	screenSpec := &tdpb.ClientScreenSpec{}
	ok := tdp.As(msg, screenSpec)
	if !ok {
		return nil, trace.Errorf("client sent unexpected message %T", msg) //sendTDPError(trace.BadParameter("client sent unexpected message %T", msg))
	}
	slog.Warn("client screen spec recieved during handshake", "height", screenSpec.Height, "width", screenSpec.Width)

	width, height := screenSpec.Width, screenSpec.Height
	if width > types.MaxRDPScreenWidth || height > types.MaxRDPScreenHeight {
		return nil, trace.Errorf("screen size of %d x %d is greater than the maximum allowed by RDP (%d x %d)",
			width, height, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight)
	}

	// Try to read the keyboard layout, which is sent by v18+ clients.
	msg, err = readClientMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keyboardLayout := &tdpb.ClientKeyboardLayout{}
	gotKeyboardLayout := tdp.As(msg, keyboardLayout)
	//keyboardLayout, gotKeyboardLayout := msg.(*tdp.ClientKeyboardLayout)
	if !gotKeyboardLayout {
		slog.InfoContext(ctx, "client did not send keyboard layout", "message_type", logutils.TypeAttr(msg))
		c.withheld = append(c.withheld, msg)
	}
	slog.Warn("tdpb screen spec", "width", width, "height", height)

	return &clientInitdata{
		screenSpec: &tdp.ClientScreenSpec{
			Width:  width,
			Height: height,
		},
		keyboardLayout: &tdp.ClientKeyboardLayout{
			KeyboardLayout: keyboardLayout.KeyboardLayout,
		},
	}, nil
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

	//sendTDPError := func(err error) error {
	//	sendErr := sendTDPAlert(ws, err, tdp.SeverityError)
	//	if sendErr != nil {
	//		return sendErr
	//	}
	//	return err
	//}

	var dec tdp.Decoder
	dialect := "tdpb/1.0"
	switch dialect {
	case "tdpb/1.0":
		var err error
		dec, err = tdp.NewMessageDecoder()
		if err != nil {
			return trace.Wrap(err, "error creating TDPB decoder")
		}
	default:
		dec = tdp.DecoderFunc(tdp.DecodeTDP)
	}

	readClientMessage := func() (tdp.Message, error) {
		typ, data, err := ws.ReadMessage()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if typ != websocket.BinaryMessage {
			return nil, trace.BadParameter("expected binary websocket message, got %v", typ)
		}

		//msg, data, err := tdp.ReadTDPBMessage(bytes.NewBuffer(data))
		msg, err := dec.Decode(bufio.NewReader(bytes.NewBuffer(data)))

		//msg, err := tdp.Decode(data)
		return msg, trace.Wrap(err)
	}

	writeClientMessage := func(msg tdp.Message) error {
		outData, err := msg.Encode()
		if err == nil {
			err = ws.WriteMessage(websocket.BinaryMessage, outData)
		}
		return err
	}

	// todo: write a constructor for this type
	exchangeHandler := clientInitExchange{
		reader: readClientMessage,
		writer: writeClientMessage,
		// Client dialect
		dialect: "tdpb/1.0",
	}

	username, err := readUsername(r)
	if err != nil {
		return exchangeHandler.sendTDPError(err)
	}

	log.DebugContext(ctx, "Attempting to connect to desktop", "username", username)
	_, err = exchangeHandler.ReadClientInit(ctx)
	if err != nil {
		exchangeHandler.sendTDPAlert(err)
		return trace.Wrap(err)
	}
	// hack
	exchangeHandler.data.username = username

	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		return exchangeHandler.sendTDPError(trace.Wrap(err))
	}

	// Parse the private key of the user from the session context.
	pk, err := keys.ParsePrivateKey(sctx.cfg.Session.GetTLSPriv())
	if err != nil {
		return exchangeHandler.sendTDPError(err)
	}

	// Check if MFA is required and create a UserCertsRequest.
	mfaRequired, certsReq, err := h.prepareForCertIssuance(ctx, sctx, cluster, pk.Public(), desktopName, username)
	if err != nil {
		return exchangeHandler.sendTDPError(err)
	}

	// Issue certificate for the user/desktop combination and perform MFA ceremony if required.
	certs, err := h.issueCerts(ctx, sctx, mfaRequired, certsReq, &exchangeHandler)
	if err != nil {
		return exchangeHandler.sendTDPError(err)
	}

	// Create a TLS config for connecting to the Windows Desktop Service.
	tlsConfig, err := h.createDesktopTLSConfig(ctx, sctx, desktopName, pk, certs)
	if err != nil {
		return exchangeHandler.sendTDPError(err)
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
		return exchangeHandler.sendTDPError(trace.Wrap(err, "cannot connect to Windows Desktop Service"))
	}
	defer serviceConn.Close()

	serviceConnTLS := tls.Client(serviceConn, tlsConfig)

	if err := serviceConnTLS.HandshakeContext(ctx); err != nil {
		return exchangeHandler.sendTDPError(err)
	}
	log.DebugContext(ctx, "Connected to windows_desktop_service")

	// TODO(rhammonds): Pick decoder based on ALPN negotiation
	tdpConn := tdp.NewConn(serviceConnTLS, tdp.DecoderFunc(tdp.DecodeTDP))
	//
	// Now figure out which dialect to use for the other side of the connection

	// Now that we have a connection to the Windows Desktop Service, we can
	// send the username and screen spec to the service, and any withheld
	// messages that were received before the MFA ceremony was completed.
	agentDialect := ""
	// Older agents will receive a clientScreenSpec and (optionally) a
	// clientKeyboardSpec. New agents will receive a single ClientHello message
	// containing both
	for _, msg := range exchangeHandler.ForwardClientHello(agentDialect) {
		err = tdpConn.WriteMessage(msg)
		if err != nil {
			return exchangeHandler.sendTDPError(err)
		}
	}

	// Optionally wait for hello from server
	var serverHello *tdpb.ServerHello
	if agentDialect == "tdpb/1.0" {
		msg, err := tdpConn.ReadMessage()
		if err != nil {
			exchangeHandler.sendTDPError(err)
		}
		tdpbMsg, ok := msg.(*tdp.TDPBMessage)
		if ok {
			serverHello, ok = tdpbMsg.Proto().(*tdpb.ServerHello)
		}

		if !ok {
			exchangeHandler.sendTDPError(errors.New("expected server hello message"))
		}
	}
	// Nothing to do with it right now
	_ = serverHello

	//err = tdpConn.WriteMessage(tdp.ClientUsername{Username: username})
	//if err != nil {
	//	return exchangeHandler.sendTDPError(err)
	//}
	//err = tdpConn.WriteMessage(screenSpec)
	//if err != nil {
	//	return exchangeHandler.sendTDPError(err)
	//}

	// Forward the user's keyboard layout to the agent, as long as the agent is new enough.
	//if keyboardLayoutSupported, _ := utils.MinVerWithoutPreRelease(version, "18.0.0"); keyboardLayoutSupported && gotKeyboardLayout {
	//	if err := tdpConn.WriteMessage(keyboardLayout); err != nil {
	//		return sendTDPError(err)
	//	}
	//} else {
	//	log.DebugContext(ctx, "Client sent keyboard layout but agent is too old", "agent_version", version)
	//}
	//
	//for _, msg := range withheld {
	//	log.DebugContext(ctx, "Sending withheld message", "message", logutils.TypeAttr(msg))
	//	if err := tdpConn.WriteMessage(msg); err != nil {
	//		return sendTDPError(err)
	//	}
	//}
	//// nil out the slice so we don't hang on to these messages
	//// for the rest of the connection
	//withheld = nil

	// this blocks until the connection is closed
	handleProxyWebsocketConnErr(
		ctx,
		proxyWebsocketConn(ctx, ws, serviceConnTLS, log, version, exchangeHandler.withheld, "", ""),
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
	//ws *websocket.Conn,
	sctx *SessionContext,
	mfaRequired bool,
	certsReq *proto.UserCertsRequest,
	//withheld *[]tdp.Message,
	mfaHandler MFAHandler,
) (certs *proto.Certs, err error) {
	if mfaRequired {
		certs, err = h.performSessionMFACeremony(ctx, sctx, certsReq, mfaHandler)
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

type MFAHandler interface {
	SendChallenge(client.MFAAuthenticateChallenge, string) error
	ReceiveAssertion() (*proto.MFAAuthenticateResponse, error)
}

// performSessionMFACeremony completes the mfa ceremony and returns the raw TLS certificate
// on success. The user will be prompted to tap their security key by the UI
// in order to perform the assertion.
func (h *Handler) performSessionMFACeremony(
	ctx context.Context,
	//ws *websocket.Conn,
	sctx *SessionContext,
	certsReq *proto.UserCertsRequest,
	//withheld *[]tdp.Message,
	mfaHandler MFAHandler,
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
		PromptConstructor: func(...mfa.PromptOpt) mfa.Prompt {
			return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
				// Convert from proto to JSON types.
				var challenge client.MFAAuthenticateChallenge
				if chal.WebauthnChallenge != nil {
					challenge.WebauthnChallenge = wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge)
				}

				if chal.SSOChallenge != nil {
					challenge.SSOChallenge = client.SSOChallengeFromProto(chal.SSOChallenge)
					challenge.SSOChallenge.ChannelID = channelID
				}

				if chal.WebauthnChallenge == nil && chal.SSOChallenge == nil {
					return nil, trace.Wrap(authclient.ErrNoMFADevices)
				}

				// Send the challenge over the who knows what.
				err = mfaHandler.SendChallenge(challenge, defaults.WebsocketMFAChallenge)
				//var codec tdpMFACodec
				//msg, err := codec.Encode(&challenge, defaults.WebsocketMFAChallenge)
				//if err != nil {
				//	return nil, trace.Wrap(err)
				//}
				//
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

				span.AddEvent("waiting for user to complete mfa ceremony")
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
				return mfaHandler.ReceiveAssertion()
			})
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

type UUIDGetter interface {
	GetUUID() []byte
}

// desktopPinger measures latency between proxy and the desktop by sending tdp.Ping messages
// Windows Desktop Service and measuring the time it takes to receive message with the same UUID back.
type tdpPinger struct {
	server tdp.MessageWriter
	ch     chan tdp.Ping
}

type desktopPinger struct {
	handler PingHandler
	server  tdp.MessageWriter
	ch      chan uuid.UUID
}

type PingHandler interface {
	NewPing() (uuid.UUID, tdp.Message)
	IsPing(msg tdp.Message) (uuid.UUID, bool)
}

type TDPBPinger struct{}

func (t *TDPBPinger) NewPing() (uuid.UUID, tdp.Message) {
	id := uuid.New()
	return id, &tdp.TDPBMessage{
		Message: &tdpb.Ping{
			UUID: id[:],
		},
	}
}

func (t *TDPBPinger) IsPing(msg tdp.Message) (uuid.UUID, bool) {
	if msg, ok := msg.(*tdp.TDPBMessage); ok {
		if p, ok := msg.Proto().(*tdpb.Ping); ok {
			id, _ := uuid.FromBytes(p.UUID)
			return id, true
		}
	}
	return uuid.Nil, false
}

type TDPPinger struct{}

func (t *TDPPinger) NewPing() (uuid.UUID, tdp.Message) {
	id := uuid.New()
	return id, tdp.Ping{UUID: id}
}

func (t *TDPPinger) IsPing(msg tdp.Message) (uuid.UUID, bool) {
	if msg, ok := msg.(*tdp.Ping); ok {
		return msg.UUID, true
	}
	return uuid.Nil, false
}

func (d desktopPinger) interceptor(msg tdp.Message) ([]tdp.Message, error) {
	if id, ok := d.handler.IsPing(msg); ok {
		d.ch <- id
		return nil, nil
	}
	return []tdp.Message{msg}, nil
}

func (d desktopPinger) Ping(ctx context.Context) error {
	id, ping := d.handler.NewPing()
	if err := d.server.WriteMessage(ping); err != nil {
		return trace.Wrap(err)
	}
	for {
		select {
		case pong := <-d.ch:
			if id == pong {
				return nil
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

func intoTDPB(message tdp.Message) ([]tdp.Message, error) {
	// Read interceptor translates messages from the desktop agent from TDP to TDPB
	return tdp.TranslateToModern(message), nil
}

func intoTDP(message tdp.Message) ([]tdp.Message, error) {
	// Write interceptor translates messages from the client from TDPB to TDP
	if msg, ok := message.(*tdp.TDPBMessage); ok {
		return tdp.TranslateToLegacy(msg.Proto()), nil
	}
	return nil, trace.BadParameter("Message is not a protocol buffer. Cannot translate from TDPB to TDP")
}

// proxyWebsocketConn does a bidrectional copy between the websocket
// connection to the browser (ws) and the mTLS connection to Windows
// Desktop Serivce (wds)
func proxyWebsocketConn(ctx context.Context, ws *websocket.Conn, wds *tls.Conn, _ *slog.Logger, version string, clientMessages []tdp.Message, clientDialect, serverDialect string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		ws.Close()
		wds.Close()
	}()
	// Hardcode for now
	serverDialect = ""
	clientDialect = "tdpb/1.0"

	latencySupported, err := utils.MinVerWithoutPreRelease(version, "17.5.0")
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(rhammonds): We need some input from the caller informing us which TDP dialect is in used
	// by the client. For now, assume that the client always speaks TDPB.
	decoder, err := tdp.NewMessageDecoder()
	if err != nil {
		return err
	}

	serverConn := tdp.MessageReadWriteCloser(tdp.NewConn(wds, tdp.DecoderFunc(tdp.DecodeTDP)))
	if clientDialect != serverDialect {
		// Translation layer is needed
		if serverDialect == "tdpb/1.0" {
			// client speaks TDP, but server speaks TDPB
			slog.Warn("Translating to TDP for client, TDPB for server")
			serverConn = tdp.NewReadWriteInterceptor(serverConn, intoTDP, intoTDPB)
		} else {
			// client speaks TDPB, but server speaks TDP
			slog.Warn("Translating to TDPB for client, TDP for server")
			serverConn = tdp.NewReadWriteInterceptor(serverConn, intoTDPB, intoTDP)
		}
	} else {
		slog.Warn("No translation layer in use")
	}

	// Now that translation is in place, we can write any witheld client messages to
	// server using the intercepted server connection handle.
	for _, msg := range clientMessages {
		if err := serverConn.WriteMessage(msg); err != nil {
			return trace.Wrap(err)
		}
	}

	// Desktop pinger should natively speak the client dialect
	var pingerHandler PingHandler
	if clientDialect == "tdpb/1.0" {
		slog.Warn("picked TDPB Pinger")
		pingerHandler = &TDPBPinger{}
	} else {
		slog.Warn("picked TDP Pinger")
		pingerHandler = &TDPPinger{}
	}

	// Server speaks TDP, client speaks TDPB
	pinger := desktopPinger{
		handler: pingerHandler,
		// The write interceptor (if applicable)
		// Will translate this message to the server's supported dialect
		server: serverConn,
		ch:     make(chan uuid.UUID),
	}

	clientConn := tdp.NewConn(&WebsocketIO{Conn: ws}, decoder)
	// Wrap the server once again with an interceptor
	proxy := tdp.NewConnProxy(clientConn, tdp.NewReadWriteInterceptor(serverConn, pinger.interceptor, nil))
	if latencySupported {
		go monitorLatency(ctx, clockwork.NewRealClock(), ws, pinger,
			latency.ReporterFunc(func(ctx context.Context, stats latency.Statistics) error {
				lstats := tdpb.LatencyStats{
					ClientLatency: uint32(stats.Client),
					ServerLatency: uint32(stats.Server),
				}

				_ = clientConn.WriteMessage(&tdp.TDPBMessage{Message: &lstats})
				return nil
			}),
		)

	}

	// Run joins and returns any read, write, or close errors from each side of the
	// connection proxy. We can inspect this singular error chain for any "real"
	// network errors (as opposed to errors that are expected from a normal teardown).
	slog.Warn("proxying connection")
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

// sendTDPAlert sends a tdp Notification over the supplied websocket with the
// error message of err.
func sendTDPAlert(ws *websocket.Conn, err error, severity tdp.Severity) error {
	msg := tdp.Alert{Message: err.Error(), Severity: severity}
	b, err := msg.Encode()
	if err != nil {
		return trace.Wrap(err)
	}
	return ws.WriteMessage(websocket.BinaryMessage, b)
}
