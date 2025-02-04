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
	"math/rand/v2"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
)

// GET /webapi/sites/:site/desktops/:desktopName/connect?access_token=<bearer_token>&username=<username>
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

	log := sctx.cfg.Log.WithField("desktop-name", desktopName).WithField("cluster-name", site.GetName())
	log.Debug("New desktop access websocket connection")

	if err := h.createDesktopConnection(r, desktopName, site.GetName(), log, sctx, site, ws); err != nil {
		// createDesktopConnection makes a best effort attempt to send an error to the user
		// (via websocket) before terminating the connection. We log the error here, but
		// return nil because our HTTP middleware will try to write the returned error in JSON
		// format, and this will fail since the HTTP connection has been upgraded to websockets.
		log.Error(err)
	}

	return nil, nil
}

func (h *Handler) createDesktopConnection(
	r *http.Request,
	desktopName string,
	clusterName string,
	log *logrus.Entry,
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

	username, err := readUsername(r)
	if err != nil {
		return sendTDPError(err)
	}
	log.Debugf("Attempting to connect to desktop using username=%v\n", username)

	// Read the tdp.ClientScreenSpec from the websocket.
	// This is always the first thing sent by the client.
	// Certificate issuance may rely on the client sending
	// a subsequent tdp.MFA message, hence we need to make
	// sure that this message has been read from the wire
	// beforehand.
	screenSpec, err := readClientScreenSpec(ws)
	if err != nil {
		return sendTDPError(err)
	}

	width, height := screenSpec.Width, screenSpec.Height
	if width > types.MaxRDPScreenWidth || height > types.MaxRDPScreenHeight {
		return sendTDPError(trace.BadParameter(
			"screen size of %d x %d is greater than the maximum allowed by RDP (%d x %d)",
			width, height, types.MaxRDPScreenWidth, types.MaxRDPScreenHeight,
		))
	}

	log.Debugf("Attempting to connect to desktop using username=%v, width=%v, height=%v\n", username, width, height)

	// Pick a random Windows desktop service as our gateway.
	// When agent mode is implemented in the service, we'll have to filter out
	// the services in agent mode.
	//
	// In the future, we may want to do something smarter like latency-based
	// routing.
	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return sendTDPError(trace.Wrap(err))
	}
	winDesktops, err := clt.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{Name: desktopName})
	if err != nil {
		return sendTDPError(trace.Wrap(err, "cannot get Windows desktops"))
	}
	if len(winDesktops) == 0 {
		return sendTDPError(trace.NotFound("no Windows desktops were found"))
	}
	var validServiceIDs []string
	for _, desktop := range winDesktops {
		if desktop.GetHostID() == "" {
			// desktops with empty host ids are invalid and should
			// only occur when migrating from an old version of teleport
			continue
		}
		validServiceIDs = append(validServiceIDs, desktop.GetHostID())
	}
	rand.Shuffle(len(validServiceIDs), func(i, j int) {
		validServiceIDs[i], validServiceIDs[j] = validServiceIDs[j], validServiceIDs[i]
	})

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

	// Holds any messages withheld while issuing certs.
	var withheld []tdp.Message
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

	c := &connector{
		log:           log,
		clt:           clt,
		site:          site,
		clientSrcAddr: clientSrcAddr,
		clientDstAddr: clientDstAddr,
	}
	serviceConn, _, err := c.connectToWindowsService(clusterName, validServiceIDs)
	if err != nil {
		return sendTDPError(trace.Wrap(err, "cannot connect to Windows Desktop Service"))
	}
	defer serviceConn.Close()

	serviceConnTLS := tls.Client(serviceConn, tlsConfig)

	if err := serviceConnTLS.HandshakeContext(ctx); err != nil {
		return sendTDPError(err)
	}
	log.Debug("Connected to windows_desktop_service")

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
	for _, msg := range withheld {
		log.Debugf("Sending withheld message: %v", msg)
		if err := tdpConn.WriteMessage(msg); err != nil {
			return sendTDPError(err)
		}
	}
	// nil out the slice so we don't hang on to these messages
	// for the rest of the connection
	withheld = nil

	// proxyWebsocketConn hangs here until connection is closed
	handleProxyWebsocketConnErr(
		proxyWebsocketConn(ws, serviceConnTLS), log)

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
						h.log.Debugf("Received non-MFA message, withholding:", msg)
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

	_, newCerts, err := client.PerformSessionMFACeremony(ctx, client.PerformSessionMFACeremonyParams{
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

	return newCerts, nil
}

func readUsername(r *http.Request) (string, error) {
	q := r.URL.Query()
	username := q.Get("username")
	if username == "" {
		return "", trace.BadParameter("missing username in URL")
	}

	return username, nil
}

func readClientScreenSpec(ws *websocket.Conn) (*tdp.ClientScreenSpec, error) {
	tdpConn := tdp.NewConn(&WebsocketIO{Conn: ws})
	return tdpConn.ReadClientScreenSpec()
}

type connector struct {
	log           *logrus.Entry
	clt           authclient.ClientI
	site          reversetunnelclient.RemoteSite
	clientSrcAddr net.Addr
	clientDstAddr net.Addr
}

// connectToWindowsService tries to make a connection to a Windows Desktop Service
// by trying each of the services provided. It returns an error if it could not connect
// to any of the services or if it encounters an error that is not a connection problem.
func (c *connector) connectToWindowsService(
	clusterName string,
	desktopServiceIDs []string,
) (conn net.Conn, version string, err error) {
	for _, id := range desktopServiceIDs {
		conn, ver, err := c.tryConnect(clusterName, id)
		if err != nil && !trace.IsConnectionProblem(err) {
			return nil, "", trace.WrapWithMessage(err,
				"error connecting to windows_desktop_service %q", id)
		}
		if trace.IsConnectionProblem(err) {
			c.log.Warnf("failed to connect to windows_desktop_service %q: %v", id, err)
			continue
		}
		if err == nil {
			return conn, ver, nil
		}
	}
	return nil, "", trace.Errorf("failed to connect to any windows_desktop_service")
}

func (c *connector) tryConnect(clusterName, desktopServiceID string) (conn net.Conn, version string, err error) {
	service, err := c.clt.GetWindowsDesktopService(context.Background(), desktopServiceID)
	if err != nil {
		log.Errorf("Error finding service with id %s", desktopServiceID)
		return nil, "", trace.NotFound("could not find windows desktop service %s: %v", desktopServiceID, err)
	}

	ver := service.GetTeleportVersion()
	*c.log = *c.log.WithField("windows-service-version", ver)
	*c.log = *c.log.WithField("windows-service-uuid", service.GetName())
	*c.log = *c.log.WithField("windows-service-addr", service.GetAddr())

	conn, err = c.site.DialTCP(reversetunnelclient.DialParams{
		From:                  c.clientSrcAddr,
		To:                    &utils.NetAddr{AddrNetwork: "tcp", Addr: service.GetAddr()},
		ConnType:              types.WindowsDesktopTunnel,
		ServerID:              service.GetName() + "." + clusterName,
		ProxyIDs:              service.GetProxyIDs(),
		OriginalClientDstAddr: c.clientDstAddr,
	})
	return conn, ver, trace.Wrap(err)
}

// proxyWebsocketConn does a bidrectional copy between the websocket
// connection to the browser (ws) and the mTLS connection to Windows
// Desktop Serivce (wds)
func proxyWebsocketConn(ws *websocket.Conn, wds net.Conn) error {
	var closeOnce sync.Once
	close := func() {
		ws.Close()
		wds.Close()
	}

	errs := make(chan error, 2)

	go func() {
		defer closeOnce.Do(close)

		// we avoid using io.Copy here, as we want to make sure
		// each TDP message is sent as a unit so that a single
		// 'message' event is emitted in the browser
		// (io.Copy's internal buffer could split one message
		// into multiple ws.WriteMessage calls)
		tc := tdp.NewConn(wds)

		// we don't care about the content of the message, we just
		// need to split the stream into individual messages and
		// write them to the websocket
		for {
			msg, err := tc.ReadMessage()
			if utils.IsOKNetworkError(err) {
				errs <- nil
				return
			} else if err != nil {
				isFatal := tdp.IsFatalErr(err)
				severity := tdp.SeverityError
				if !isFatal {
					severity = tdp.SeverityWarning
				}
				sendErr := sendTDPAlert(ws, err, severity)

				// If the error wasn't fatal and we successfully
				// sent it back to the client, continue.
				if !isFatal && sendErr == nil {
					continue
				}

				// If the error was fatal or we failed to send it back
				// to the client, send it to the errs channel and end
				// the session.
				if sendErr != nil {
					err = sendErr
				}
				errs <- err
				return
			}
			encoded, err := msg.Encode()
			if err != nil {
				errs <- err
				return
			}
			err = ws.WriteMessage(websocket.BinaryMessage, encoded)
			if utils.IsOKNetworkError(err) {
				errs <- nil
				return
			}
			if err != nil {
				errs <- err
				return
			}
		}
	}()

	go func() {
		defer closeOnce.Do(close)

		var buf bytes.Buffer
		for {
			_, reader, err := ws.NextReader()
			switch {
			case utils.IsOKNetworkError(err):
				errs <- nil
				return
			case err != nil:
				errs <- err
				return
			}
			buf.Reset()
			if _, err := io.Copy(&buf, reader); err != nil {
				errs <- err
				return
			}

			if _, err := wds.Write(buf.Bytes()); err != nil {
				errs <- trace.Wrap(err, "sending TDP message to desktop agent")
				return
			}
		}
	}()

	var retErrs []error
	for i := 0; i < 2; i++ {
		retErrs = append(retErrs, <-errs)
	}
	return trace.NewAggregate(retErrs...)
}

// handleProxyWebsocketConnErr handles the error returned by proxyWebsocketConn by
// unwrapping it and determining whether to log an error.
func handleProxyWebsocketConnErr(proxyWsConnErr error, log *logrus.Entry) {
	if proxyWsConnErr == nil {
		log.Debug("proxyWebsocketConn returned with no error")
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
				log.Debugf("Web socket closed by client with code: %v", closeErr.Code)
				return
			}
			return
		default:
			if wrapped := errors.Unwrap(err); wrapped != nil {
				errs = append(errs, wrapped)
			}
		}
	}

	log.WithError(proxyWsConnErr).Warning("Error proxying a desktop protocol websocket to windows_desktop_service")
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
