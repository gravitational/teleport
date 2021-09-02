/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package web

import (
	"crypto/tls"
	"io"
	"math/rand"
	"net"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/websocket"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/srv/desktop"
	"github.com/gravitational/teleport/lib/utils"
)

func (h *Handler) handleDesktopAccessWebsocket(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
	site reversetunnel.RemoteSite,
) (interface{}, error) {
	desktopUUID := p.ByName("desktopUUID")
	if desktopUUID == "" {
		return nil, trace.BadParameter("missing desktopUUID in request URL")
	}
	log := ctx.log.WithField("desktop-uuid", desktopUUID)
	log.Debug("New desktop access websocket connection")

	winServices, err := ctx.unsafeCachedAuthClient.GetWindowsDesktopServices(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(awly): trusted cluster support - if this request is for a different
	// cluster, dial their proxy and forward the websocket request as is.

	if len(winServices) == 0 {
		return nil, trace.NotFound("No windows_desktop_services are registered in this cluster")
	}
	// Pick a random Windows desktop service as our gateway.
	// When agent mode is implemented in the service, we'll have to filter out
	// the services in agent mode.
	//
	// In the future, we may want to do something smarter like latency-based
	// routing.
	service := winServices[rand.Intn(len(winServices))]
	log = log.WithField("windows-service-uuid", service.GetName())
	log = log.WithField("windows-service-addr", service.GetAddr())
	serviceCon, err := site.DialTCP(reversetunnel.DialParams{
		From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: r.RemoteAddr},
		To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: service.GetAddr()},
		ConnType: types.WindowsDesktopTunnel,
		ServerID: service.GetName(),
	})
	if err != nil {
		return nil, trace.WrapWithMessage(err, "failed to connect to windows_desktop_service at %q: %v", service.GetAddr(), err)
	}
	defer serviceCon.Close()
	tlsConfig := ctx.clt.Config()
	// Pass target desktop UUID via SNI.
	tlsConfig.ServerName = desktopUUID + desktop.SNISuffix
	serviceConTLS := tls.Client(serviceCon, ctx.clt.Config())
	log.Debug("Connected to windows_desktop_service")

	websocket.Handler(func(conn *websocket.Conn) {
		if err := proxyWebsocketConn(conn, serviceConTLS); err != nil {
			log.WithError(err).Warningf("Error proxying a desktop protocol websocket to windows_desktop_service")
		}
	}).ServeHTTP(w, r)
	return nil, nil
}

func proxyWebsocketConn(ws *websocket.Conn, con net.Conn) error {
	// Ensure we send binary frames to the browser.
	ws.PayloadType = websocket.BinaryFrame

	errs := make(chan error, 2)
	go func() {
		defer ws.Close()
		defer con.Close()

		_, err := io.Copy(ws, con)
		errs <- err
	}()
	go func() {
		defer ws.Close()
		defer con.Close()

		_, err := io.Copy(con, ws)
		errs <- err
	}()

	var retErrs []error
	for i := 0; i < 2; i++ {
		retErrs = append(retErrs, <-errs)
	}
	return trace.NewAggregate(retErrs...)
}
