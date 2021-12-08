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
	"strconv"
	"strings"
	"sync"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/srv/desktop"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
)

// GET /webapi/sites/:site/desktops/:desktopName/connect?access_token=<bearer_token>&username=<username>&width=<width>&height=<height>
func (h *Handler) desktopConnectHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
	site reversetunnel.RemoteSite,
) (interface{}, error) {
	desktopName := p.ByName("desktopName")
	if desktopName == "" {
		return nil, trace.BadParameter("missing desktopName in request URL")
	}

	log := ctx.log.WithField("desktop-name", desktopName)
	log.Debug("New desktop access websocket connection")

	if err := createDesktopConnection(w, r, desktopName, log, ctx, site); err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

func createDesktopConnection(
	w http.ResponseWriter,
	r *http.Request,
	desktopName string,
	log *logrus.Entry,
	ctx *SessionContext,
	site reversetunnel.RemoteSite,
) error {

	q := r.URL.Query()
	username := q.Get("username")
	if username == "" {
		return trace.BadParameter("missing username")
	}
	width, err := strconv.Atoi(q.Get("width"))
	if err != nil {
		return trace.BadParameter("width missing or invalid")
	}
	height, err := strconv.Atoi(q.Get("height"))
	if err != nil {
		return trace.BadParameter("height missing or invalid")
	}

	log.Debugf("Attempting to connect to desktop using username=%v, width=%v, height=%v\n", username, width, height)

	winServices, err := ctx.unsafeCachedAuthClient.GetWindowsDesktopServices(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(awly): trusted cluster support - if this request is for a different
	// cluster, dial their proxy and forward the websocket request as is.

	if len(winServices) == 0 {
		return trace.NotFound("No windows_desktop_services are registered in this cluster")
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
	c, err := site.DialTCP(reversetunnel.DialParams{
		From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: r.RemoteAddr},
		To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: service.GetAddr()},
		ConnType: types.WindowsDesktopTunnel,
		ServerID: service.GetName(),
	})
	if err != nil {
		return trace.WrapWithMessage(err, "failed to connect to windows_desktop_service at %q: %v", service.GetAddr(), err)
	}
	defer c.Close()
	tlsConfig := ctx.clt.Config()
	// Pass target desktop UUID via SNI.
	tlsConfig.ServerName = desktopName + desktop.SNISuffix
	windowsDesktopService := tls.Client(c, ctx.clt.Config())
	log.Debug("Connected to windows_desktop_service")

	// Send TDP initialization messages to the service.
	tdpConn := tdp.NewConn(windowsDesktopService)
	err = tdpConn.Write(tdp.ClientUsername{Username: username})
	if err != nil {
		return trace.Wrap(err)
	}
	err = tdpConn.Write(tdp.ClientScreenSpec{Width: uint32(width), Height: uint32(height)})
	if err != nil {
		return trace.Wrap(err)
	}

	websocket.Handler(func(ws *websocket.Conn) {
		if err := proxyConns(ws, windowsDesktopService); err != nil {
			log.WithError(err).Warningf("Error proxying a TDP websocket to windows_desktop_service")
		} else {
			log.Info("TDP websocket to windows_desktop_service proxy terminated correctly")
		}
	}).ServeHTTP(w, r)
	return nil
}

func proxyConns(ws *websocket.Conn, service net.Conn) error {
	// Ensure we send binary frames to the browser.
	ws.PayloadType = websocket.BinaryFrame

	// Channel for handling error returns from goroutines.
	errs := make(chan error, 2)

	// The io.Copy calls in proxyConn hangs, copying from src to dst until an EOF is reached on src (or an error occurs).
	// A successful Copy returns err == nil, not err == EOF. Because Copy is defined to read from src until EOF,
	// it does not treat an EOF from Read as an error to be reported.
	//
	// Whether EOF is reached is determined by the underlying src (io.Reader) implementation, which signals that by
	// returning an io.EOF error. For example, if the browser closes the websocket connection, the *websocket.Conn
	// recognizes a close message from the websocket spec was sent to it, and on it's next Read() returns an io.EOF error.
	// Subsequently, the io.Copy(service, ws) returns `_, nil`, and that goroutine ultimately returns.
	//
	// We have a problem, though, in that the deferred `service.Close()` (from proxyConn) causes `io.Copy(ws, service)` in the other goroutine
	// to return a non-nil error of the form "read tcp ip_addr_0:port_0->ip_addr_1:port_1: use of closed network connection".
	// That's because `service.Close()` sends a message to the other side of the connection; but our side never sees a message signaling it to return an io.EOF,
	//  it just tries to Read() from a closed net.Conn which gives us that non-nil error.
	//
	// This `cleanClose` variable plus some logic in each goroutine allows us to distinguish between a true error (which we want to report) and
	// the error described above (which we wish to consider a clean disconnection).
	type safeBool struct {
		mu    sync.Mutex
		clean bool
	}
	var cleanClose safeBool

	handleCopyErr := func(err error) error {
		cleanClose.mu.Lock()
		defer cleanClose.mu.Unlock()

		if err == nil {
			cleanClose.clean = true
			return nil
		} else if cleanClose.clean && strings.HasPrefix(err.Error(), "read tcp") && strings.HasSuffix(err.Error(), net.ErrClosed.Error()) {
			// If the other goroutine set cleanClose and we got the expected error, treat that as a nil error.
			return nil
		}
		// Non-nil error and the other goroutine didn't set cleanClose to true, or it did but we got an unexpected error.
		cleanClose.clean = false
		return err
	}

	proxyConn := func(from net.Conn, to net.Conn) {
		defer from.Close()
		defer to.Close()

		_, copyErr := io.Copy(to, from)
		err := handleCopyErr(copyErr)

		errs <- err
	}

	go func() {
		proxyConn(service, ws)
	}()

	go func() {
		proxyConn(ws, service)
	}()

	var retErrs []error
	for i := 0; i < 2; i++ {
		retErrs = append(retErrs, <-errs)
	}
	return trace.NewAggregate(retErrs...)
}
