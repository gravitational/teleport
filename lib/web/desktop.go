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
	"context"
	"crypto"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"

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
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client"
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
	site reversetunnelclient.RemoteSite,
	ws *websocket.Conn,
) (interface{}, error) {
	desktopName := p.ByName("desktopName")
	if desktopName == "" {
		return nil, trace.BadParameter("missing desktopName in request URL")
	}

	log := sctx.cfg.Log.With(
		"desktop_name", desktopName,
		"cluster_name", site.GetName(),
	)
	log.DebugContext(r.Context(), "New desktop access websocket connection")

	if err := h.createDesktopConnection(r, desktopName, site.GetName(), log, sctx, site, ws); err != nil {
		// createDesktopConnection makes a best effort attempt to send an error to the user
		// (via websocket) before terminating the connection. We log the error here, but
		// return nil because our HTTP middleware will try to write the returned error in JSON
		// format, and this will fail since the HTTP connection has been upgraded to websockets.
		log.ErrorContext(r.Context(), "creating desktop connection failed", "error", err)
	}

	return nil, nil
}

func (h *Handler) createDesktopConnection(
	r *http.Request,
	desktopName string,
	clusterName string,
	log *slog.Logger,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
	ws *websocket.Conn,
) error {
	defer ws.Close()
	ctx := r.Context()

	sendTDPError := func(err error) error {
		sendErr := sendTDPAlert(ws, err, tdp.SeverityError)
		if sendErr != nil {
			return sendErr
		}
		return err
	}

	readClientMessage := func() (tdp.Message, error) {
		typ, data, err := ws.ReadMessage()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if typ != websocket.BinaryMessage {
			return nil, trace.BadParameter("expected binary websocket message, got %v", typ)
		}

		msg, err := tdp.Decode(data)
		return msg, trace.Wrap(err)
	}

	username, err := readUsername(r)
	if err != nil {
		return sendTDPError(err)
	}
	log.DebugContext(ctx, "Attempting to connect to desktop", "username", username)

	// The first thing we expect from the client is the screen spec.
	msg, err := readClientMessage()
	if err != nil {
		return trace.Wrap(err)
	}
	screenSpec, ok := msg.(tdp.ClientScreenSpec)
	if !ok {
		return sendTDPError(trace.BadParameter("client sent unexpected message %T", msg))
	}

	width, height := screenSpec.Width, screenSpec.Height
	if width > types.MaxRDPScreenWidth || height > types.MaxRDPScreenHeight {
		return sendTDPError(trace.BadParameter(
			"screen size of %d x %d is greater than the maximum allowed by RDP (%d x %d)",
			width, height, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight,
		))
	}

	log = log.With("username", username, "width", width, "height", height)

	// Holds any messages withheld while issuing certs.
	var withheld []tdp.Message

	// Try to read the keyboard layout, which is sent by v18+ clients.
	msg, err = readClientMessage()
	if err != nil {
		return trace.Wrap(err)
	}
	keyboardLayout, gotKeyboardLayout := msg.(tdp.ClientKeyboardLayout)
	if !gotKeyboardLayout {
		log.InfoContext(ctx, "client did not send keyboard layout", "message_type", logutils.TypeAttr(msg))
		withheld = append(withheld, msg)
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return sendTDPError(trace.Wrap(err))
	}

	// Parse the private key of the user from the session context.
	pk, err := keys.ParsePrivateKey(sctx.cfg.Session.GetTLSPriv())
	if err != nil {
		return sendTDPError(err)
	}

	// Check if MFA is required and create a UserCertsRequest.
	mfaRequired, certsReq, err := h.prepareForCertIssuance(ctx, sctx, site, pk.Public(), desktopName, username)
	if err != nil {
		return sendTDPError(err)
	}

	// Issue certificate for the user/desktop combination and perform MFA ceremony if required.
	certs, err := h.issueCerts(ctx, ws, sctx, mfaRequired, certsReq, &withheld)
	if err != nil {
		return sendTDPError(err)
	}

	// Create a TLS config for connecting to the Windows Desktop Service.
	tlsConfig, err := h.createDesktopTLSConfig(ctx, sctx, desktopName, pk, certs)
	if err != nil {
		return sendTDPError(err)
	}

	clientSrcAddr, clientDstAddr := authz.ClientAddrsFromContext(ctx)

	serviceConn, version, err := desktop.ConnectToWindowsService(ctx, &desktop.ConnectionConfig{
		Log:            log,
		DesktopsGetter: clt,
		Site:           site,
		ClientSrcAddr:  clientSrcAddr,
		ClientDstAddr:  clientDstAddr,
		DesktopName:    desktopName,
		ClusterName:    clusterName,
	})
	if err != nil {
		return sendTDPError(trace.Wrap(err, "cannot connect to Windows Desktop Service"))
	}
	defer serviceConn.Close()

	serviceConnTLS := tls.Client(serviceConn, tlsConfig)

	if err := serviceConnTLS.HandshakeContext(ctx); err != nil {
		return sendTDPError(err)
	}
	log.DebugContext(ctx, "Connected to windows_desktop_service")

	tdpConn := tdp.NewConn(serviceConnTLS)

	// Now that we have a connection to the Windows Desktop Service, we can
	// send the username and screen spec to the service, and any withheld
	// messages that were received before the MFA ceremony was completed.
	err = tdpConn.WriteMessage(tdp.ClientUsername{Username: username})
	if err != nil {
		return sendTDPError(err)
	}
	err = tdpConn.WriteMessage(screenSpec)
	if err != nil {
		return sendTDPError(err)
	}

	// Forward the user's keyboard layout to the agent, as long as the agent is new enough.
	if keyboardLayoutSupported, _ := utils.MinVerWithoutPreRelease(version, "18.0.0"); keyboardLayoutSupported && gotKeyboardLayout {
		if err := tdpConn.WriteMessage(keyboardLayout); err != nil {
			return sendTDPError(err)
		}
	} else {
		log.DebugContext(ctx, "Client sent keyboard layout but agent is too old", "agent_version", version)
	}

	for _, msg := range withheld {
		log.DebugContext(ctx, "Sending withheld message", "message", logutils.TypeAttr(msg))
		if err := tdpConn.WriteMessage(msg); err != nil {
			return sendTDPError(err)
		}
	}
	// nil out the slice so we don't hang on to these messages
	// for the rest of the connection
	withheld = nil

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
	site reversetunnelclient.RemoteSite,
	publicKey crypto.PublicKey,
	desktopName, username string,
) (mfaRequired bool, certsReq *proto.UserCertsRequest, err error) {
	// Check if MFA is required for this user/desktop combination.
	mfaRequired, err = h.checkMFARequired(ctx, &IsMFARequiredRequest{
		WindowsDesktop: &isMFARequiredWindowsDesktop{
			DesktopName: desktopName,
			Login:       username,
		},
	}, sctx, site)
	if err != nil {
		return false, nil, trace.Wrap(err)
	}

	certsReq, err = createUserCertsRequest(sctx, publicKey, desktopName, username, site.GetName())
	if err != nil {
		return false, nil, trace.Wrap(err)
	}

	return mfaRequired, certsReq, nil
}

// issueCerts issues certificates for the user/desktop combination, performing
// the MFA ceremony if required.
func (h *Handler) issueCerts(
	ctx context.Context,
	ws *websocket.Conn,
	sctx *SessionContext,
	mfaRequired bool,
	certsReq *proto.UserCertsRequest,
	withheld *[]tdp.Message,
) (certs *proto.Certs, err error) {
	if mfaRequired {
		certs, err = h.performSessionMFACeremony(ctx, ws, sctx, certsReq, withheld)
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
	ws *websocket.Conn,
	sctx *SessionContext,
	certsReq *proto.UserCertsRequest,
	withheld *[]tdp.Message,
) (_ *proto.Certs, err error) {
	ctx, span := h.tracer.Start(ctx, "desktop/performSessionMFACeremony")
	defer func() {
		span.RecordError(err)
		span.End()
	}()

	mfaCeremony := &mfa.Ceremony{
		PromptConstructor: func(po ...mfa.PromptOpt) mfa.Prompt {
			return mfa.PromptFunc(func(ctx context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
				codec := tdpMFACodec{}

				if chal.WebauthnChallenge == nil {
					return nil, trace.AccessDenied("Desktop access requires WebAuthn MFA, please register a WebAuthn device to connect")
				}
				// Send the challenge over the socket.
				msg, err := codec.Encode(
					&client.MFAAuthenticateChallenge{
						WebauthnChallenge: wantypes.CredentialAssertionFromProto(chal.WebauthnChallenge),
					},
					defaults.WebsocketMFAChallenge,
				)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				if err := ws.WriteMessage(websocket.BinaryMessage, msg); err != nil {
					return nil, trace.Wrap(err)
				}

				// Special case: if we've already received an MFA response (because an old web UI
				// that doesn't send the keyboard layout is connected), then we're done.
				if len(*withheld) > 0 {
					mfaResp, ok := (*withheld)[0].(*tdp.MFA)
					if ok {
						return mfaResp.MFAAuthenticateResponse, nil
					}
				}

				span.AddEvent("waiting for user to complete mfa ceremony")
				var buf []byte
				// Loop through incoming messages until we receive an MFA message that lets us
				// complete the ceremony. Non-MFA messages (e.g. ClientScreenSpecs representing
				// screen resizes) are withheld for later.
				for {
					var ty int
					ty, buf, err = ws.ReadMessage()
					if err != nil {
						return nil, trace.Wrap(err)
					}
					if ty != websocket.BinaryMessage {
						return nil, trace.BadParameter("received unexpected web socket message type %d", ty)
					}
					if len(buf) == 0 {
						return nil, trace.BadParameter("empty message received")
					}

					if tdp.MessageType(buf[0]) != tdp.TypeMFA {
						// This is not an MFA message, withhold it for later.
						msg, err := tdp.Decode(buf)
						h.logger.DebugContext(ctx, "Received non-MFA message, withholding", "msg_type", logutils.TypeAttr(msg))
						if err != nil {
							return nil, trace.Wrap(err)
						}
						*withheld = append(*withheld, msg)
						continue
					}

					break
				}

				assertion, err := codec.DecodeResponse(buf, defaults.WebsocketMFAChallenge)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				span.AddEvent("mfa ceremony completed")

				return assertion, nil
			})
		},
		CreateAuthenticateChallenge: sctx.cfg.RootClient.CreateAuthenticateChallenge,
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
				log.DebugContext(ctx, "sending latency stats", "client", stats.Client, "server", stats.Server)
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
func sendTDPAlert(ws *websocket.Conn, err error, severity tdp.Severity) error {
	msg := tdp.Alert{Message: err.Error(), Severity: severity}
	b, err := msg.Encode()
	if err != nil {
		return trace.Wrap(err)
	}
	return ws.WriteMessage(websocket.BinaryMessage, b)
}
