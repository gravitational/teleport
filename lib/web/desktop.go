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
	"context"
	"crypto/tls"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
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

	if err := h.createDesktopConnection(w, r, desktopName, log, ctx, site); err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

func (h *Handler) createDesktopConnection(
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

	// TODO(awly): trusted cluster support - if this request is for a different
	// cluster, dial their proxy and forward the websocket request as is.

	// Pick a random Windows desktop service as our gateway.
	// When agent mode is implemented in the service, we'll have to filter out
	// the services in agent mode.
	//
	// In the future, we may want to do something smarter like latency-based
	// routing.
	winDesktops, err := ctx.unsafeCachedAuthClient.GetWindowsDesktops(r.Context(),
		types.WindowsDesktopFilter{Name: desktopName})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(winDesktops) == 0 {
		return trace.NotFound("no windows_desktops were found")
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

	c := &connector{
		log:      log,
		clt:      ctx.clt,
		site:     site,
		userAddr: r.RemoteAddr,
	}
	serviceConn, err := c.connectToWindowsService(ctx.parent.clusterName, validServiceIDs)
	if err != nil {
		return trace.Wrap(err)
	}
	defer serviceConn.Close()

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	pc, err := proxyClient(r.Context(), ctx, h.ProxyHostPort())
	if err != nil {
		return trace.Wrap(err)
	}
	defer pc.Close()

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer ws.Close()

	sendTDPError := func(ws *websocket.Conn, err error) error {
		tdpErr := tdp.NewConn(&WebsocketIO{Conn: ws}).SendError(err.Error())
		if tdpErr != nil {
			return trace.Wrap(tdpErr)
		}
		return nil
	}

	tlsConfig, err := desktopTLSConfig(r.Context(), ws, pc, ctx, desktopName, username, site.GetName())
	if err != nil {
		return trace.NewAggregate(err, sendTDPError(ws, err))
	}
	serviceConnTLS := tls.Client(serviceConn, tlsConfig)

	if err := serviceConnTLS.Handshake(); err != nil {
		return trace.NewAggregate(err, sendTDPError(ws, err))
	}
	log.Debug("Connected to windows_desktop_service")

	tdpConn := tdp.NewConn(serviceConnTLS)
	err = tdpConn.OutputMessage(tdp.ClientUsername{Username: username})
	if err != nil {
		return trace.NewAggregate(err, sendTDPError(ws, err))
	}
	err = tdpConn.OutputMessage(tdp.ClientScreenSpec{Width: uint32(width), Height: uint32(height)})
	if err != nil {
		return trace.NewAggregate(err, sendTDPError(ws, err))
	}

	if err := proxyWebsocketConn(ws, serviceConnTLS); err != nil {
		log.WithError(err).Warningf("Error proxying a desktop protocol websocket to windows_desktop_service")
	}
	return nil
}

func proxyClient(ctx context.Context, sessCtx *SessionContext, addr string) (*client.ProxyClient, error) {
	cfg, err := makeTeleportClientConfig(ctx, sessCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := cfg.ParseProxyHost(addr); err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pc, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pc, nil
}

func desktopTLSConfig(ctx context.Context, ws *websocket.Conn, pc *client.ProxyClient, sessCtx *SessionContext, desktopName, username, siteName string) (*tls.Config, error) {
	priv, err := ssh.ParsePrivateKey(sessCtx.session.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var wsLock sync.Mutex
	key, err := pc.IssueUserCertsWithMFA(ctx, client.ReissueParams{
		RouteToWindowsDesktop: proto.RouteToWindowsDesktop{
			WindowsDesktop: desktopName,
			Login:          username,
		},
		RouteToCluster: siteName,
		ExistingCreds: &client.Key{
			Pub:                 ssh.MarshalAuthorizedKey(priv.PublicKey()),
			Priv:                sessCtx.session.GetPriv(),
			Cert:                sessCtx.session.GetPub(),
			TLSCert:             sessCtx.session.GetTLSCert(),
			WindowsDesktopCerts: make(map[string][]byte),
		},
	}, promptMFAChallenge(ws, &wsLock, tdpMFACodec{}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	windowsDesktopCerts, ok := key.WindowsDesktopCerts[desktopName]
	if !ok {
		return nil, trace.NotFound("failed to find windows desktop certificates for %q", desktopName)
	}
	certConf, err := tls.X509KeyPair(windowsDesktopCerts, sessCtx.session.GetPriv())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig, err := sessCtx.ClientTLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.Certificates = []tls.Certificate{certConf}
	// Pass target desktop name via SNI.
	tlsConfig.ServerName = desktopName + desktop.SNISuffix
	return tlsConfig, nil
}

type connector struct {
	log      *logrus.Entry
	clt      *auth.Client
	site     reversetunnel.RemoteSite
	userAddr string
}

// connectToWindowsService tries to make a connection to a Windows Desktop Service
// by trying each of the services provided. It returns an error if it could not connect
// to any of the services or if it encounters an error that is not a connection problem.
func (c *connector) connectToWindowsService(clusterName string, desktopServiceIDs []string) (net.Conn, error) {
	for _, id := range desktopServiceIDs {
		conn, err := c.tryConnect(clusterName, id)
		if err != nil && !trace.IsConnectionProblem(err) {
			return nil, trace.WrapWithMessage(err,
				"error connecting to windows_desktop_service %q", id)
		}
		if trace.IsConnectionProblem(err) {
			continue
		}
		if err == nil {
			return conn, err
		}
	}
	return nil, trace.Errorf("failed to connect to any windows_desktop_service")
}

func (c *connector) tryConnect(clusterName, desktopServiceID string) (net.Conn, error) {
	service, err := c.clt.GetWindowsDesktopService(context.Background(), desktopServiceID)
	if err != nil {
		log.Errorf("Error finding service with id %s", desktopServiceID)
		return nil, trace.NotFound("could not find windows desktop service %s: %v", desktopServiceID, err)
	}

	*c.log = *c.log.WithField("windows-service-uuid", service.GetName())
	*c.log = *c.log.WithField("windows-service-addr", service.GetAddr())
	return c.site.DialTCP(reversetunnel.DialParams{
		From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: c.userAddr},
		To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: service.GetAddr()},
		ConnType: types.WindowsDesktopTunnel,
		ServerID: service.GetName() + "." + clusterName,
	})
}

func proxyWebsocketConn(ws *websocket.Conn, con net.Conn) error {
	var closeOnce sync.Once
	close := func() {
		ws.Close()
		con.Close()
	}

	errs := make(chan error, 2)
	stream := &WebsocketIO{Conn: ws}
	go func() {
		defer closeOnce.Do(close)

		_, err := io.Copy(stream, con)
		if utils.IsOKNetworkError(err) {
			err = nil
		}
		errs <- err
	}()
	go func() {
		defer closeOnce.Do(close)

		_, err := io.Copy(con, stream)
		if utils.IsOKNetworkError(err) {
			err = nil
		}
		errs <- err
	}()

	var retErrs []error
	for i := 0; i < 2; i++ {
		retErrs = append(retErrs, <-errs)
	}
	return trace.NewAggregate(retErrs...)
}
