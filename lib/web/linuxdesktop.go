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
	"crypto/tls"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"log/slog"
	"net/http"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// GET /webapi/sites/:site/linuxdesktops/:desktopName/connect?username=<username>
func (h *Handler) linuxDesktopConnectHandle(
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

	if err := h.createLinuxDesktopConnection(r, desktopName, cluster.GetName(), log, sctx, cluster, ws); err != nil {
		// createDesktopConnection makes a best effort attempt to send an error to the user
		// (via websocket) before terminating the connection. We log the error here, but
		// return nil because our HTTP middleware will try to write the returned error in JSON
		// format, and this will fail since the HTTP connection has been upgraded to websockets.
		log.ErrorContext(r.Context(), "creating desktop connection failed", "error", err)
	}

	return nil, nil
}

func (h *Handler) createLinuxDesktopConnection(
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

	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		return sendTDPError(trace.Wrap(err))
	}

	// Parse the private key of the user from the session context.
	pk, err := keys.ParsePrivateKey(sctx.cfg.Session.GetTLSPriv())
	if err != nil {
		return sendTDPError(err)
	}

	// Check if MFA is required and create a UserCertsRequest.
	mfaRequired, certsReq, err := h.prepareForCertIssuance(ctx, sctx, cluster, pk.Public(), desktopName, username)
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

	d, err := clt.GetLinuxDesktop(ctx, desktopName)
	if err != nil {
		return sendTDPError(err)
	}

	serviceConn, err := cluster.DialTCP(reversetunnelclient.DialParams{
		From:                  clientSrcAddr,
		To:                    &utils.NetAddr{AddrNetwork: "tcp", Addr: d.GetSpec().Addr},
		ConnType:              types.LinuxDesktopTunnel,
		ServerID:              desktopName + "." + clusterName,
		ProxyIDs:              nil,
		OriginalClientDstAddr: clientDstAddr,
	})

	if err != nil {
		return sendTDPError(trace.Wrap(err, "cannot connect to Linux Desktop Service"))
	}
	defer serviceConn.Close()

	serviceConnTLS := tls.Client(serviceConn, tlsConfig)

	if err := serviceConnTLS.HandshakeContext(ctx); err != nil {
		return sendTDPError(err)
	}
	log.DebugContext(ctx, "Connected to windows_desktop_service")

	tdpConn := tdp.NewConn(serviceConnTLS)

	// Now that we have a connection to the Linux Desktop Service, we can
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

	if err := tdpConn.WriteMessage(keyboardLayout); err != nil {
		return sendTDPError(err)
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
		proxyWebsocketConn(ctx, ws, serviceConnTLS, log, "18.0.0"),
		log,
	)

	return nil
}
