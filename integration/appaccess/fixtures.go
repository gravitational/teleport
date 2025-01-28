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

package appaccess

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

type AppTestOptions struct {
	ExtraRootApps        []servicecfg.App
	ExtraLeafApps        []servicecfg.App
	RootClusterListeners helpers.InstanceListenerSetupFunc
	LeafClusterListeners helpers.InstanceListenerSetupFunc
	Clock                clockwork.Clock
	MonitorCloseChannel  chan struct{}

	RootConfig func(config *servicecfg.Config)
	LeafConfig func(config *servicecfg.Config)
}

// Setup configures all clusters and servers needed for a test.
func Setup(t *testing.T) *Pack {
	return SetupWithOptions(t, AppTestOptions{})
}

// SetupWithOptions configures app access test with custom options.
func SetupWithOptions(t *testing.T, opts AppTestOptions) *Pack {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	log := utils.NewSlogLoggerForTests()

	// Insecure development mode needs to be set because the web proxy uses a
	// self-signed certificate during tests.
	lib.SetInsecureDevMode(true)

	p := &Pack{
		rootAppName:        "app-01",
		rootAppPublicAddr:  "app-01.example.com",
		rootAppClusterName: "example.com",
		rootMessage:        uuidWithLabel("app-01"),

		rootWSAppName:    "ws-01",
		rootWSPublicAddr: "ws-01.example.com",
		rootWSMessage:    uuidWithLabel("ws-01"),

		rootWSSAppName:    "wss-01",
		rootWSSPublicAddr: "wss-01.example.com",
		rootWSSMessage:    uuidWithLabel("wss-01"),

		rootTCPAppName:    "tcp-01",
		rootTCPPublicAddr: "tcp-01.example.com",
		rootTCPMessage:    uuidWithLabel("tcp-01"),

		rootTCPTwoWayAppName:    "tcp-twoway",
		rootTCPTwoWayPublicAddr: "tcp-twoway.example.com",
		rootTCPTwoWayMessage:    uuidWithLabel("tcp-twoway"),

		rootTCPMultiPortAppName:      "tcp-multiport-01",
		rootTCPMultiPortPublicAddr:   "tcp-multiport-01.example.com",
		rootTCPMultiPortMessageAlpha: uuidWithLabel("tcp-multiport-01-alpha"),
		rootTCPMultiPortMessageBeta:  uuidWithLabel("tcp-multiport-01-beta"),

		leafAppName:        "app-02",
		leafAppPublicAddr:  "app-02.example.com",
		leafAppClusterName: "leaf.example.com",
		leafMessage:        uuidWithLabel("app-02"),

		leafWSAppName:    "ws-02",
		leafWSPublicAddr: "ws-02.example.com",
		leafWSMessage:    uuidWithLabel("ws-02"),

		leafWSSAppName:    "wss-02",
		leafWSSPublicAddr: "wss-02.example.com",
		leafWSSMessage:    uuidWithLabel("wss-02"),

		leafTCPAppName:    "tcp-02",
		leafTCPPublicAddr: "tcp-02.example.com",
		leafTCPMessage:    uuidWithLabel("tcp-02"),

		leafTCPMultiPortAppName:      "tcp-multiport-02",
		leafTCPMultiPortPublicAddr:   "tcp-multiport-02.example.com",
		leafTCPMultiPortMessageAlpha: uuidWithLabel("tcp-multiport-02-alpha"),
		leafTCPMultiPortMessageBeta:  uuidWithLabel("tcp-multiport-02-beta"),

		jwtAppName:        "app-03",
		jwtAppPublicAddr:  "app-03.example.com",
		jwtAppClusterName: "example.com",

		headerAppName:        "app-04",
		headerAppPublicAddr:  "app-04.example.com",
		headerAppClusterName: "example.com",

		wsHeaderAppName:        "ws-header",
		wsHeaderAppPublicAddr:  "ws-header.example.com",
		wsHeaderAppClusterName: "example.com",

		flushAppName:        "app-05",
		flushAppPublicAddr:  "app-05.example.com",
		flushAppClusterName: "example.com",
	}

	createHandler := func(handler func(conn *websocket.Conn)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			upgrader := websocket.Upgrader{
				ReadBufferSize:  1024,
				WriteBufferSize: 1024,
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			require.NoError(t, err)
			handler(conn)
		}
	}

	// Start a few different HTTP server that will be acting like a proxied application.
	rootServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, p.rootMessage)
	}))
	t.Cleanup(rootServer.Close)
	// Websockets server in root cluster (ws://).
	rootWSServer := httptest.NewServer(createHandler(func(conn *websocket.Conn) {
		conn.WriteMessage(websocket.BinaryMessage, []byte(p.rootWSMessage))
		conn.Close()
	}))
	t.Cleanup(rootWSServer.Close)
	// Secure websockets server in root cluster (wss://).
	rootWSSServer := httptest.NewTLSServer(createHandler(func(conn *websocket.Conn) {
		conn.WriteMessage(websocket.BinaryMessage, []byte(p.rootWSSMessage))
		conn.Close()
	}))
	t.Cleanup(rootWSSServer.Close)
	// Plain TCP application in root cluster (tcp://).
	rootTCPServer := newTCPServer(t, func(c net.Conn) {
		c.Write([]byte(p.rootTCPMessage))
		c.Close()
	})
	t.Cleanup(func() { rootTCPServer.Close() })
	// TCP application that reads after every write in the root cluster (tcp://).
	rootTCPTwoWayServer := newTCPServer(t, func(c net.Conn) {
		buf := make([]byte, 64)
		for {
			if _, err := c.Write([]byte(p.rootTCPTwoWayMessage)); err != nil {
				break
			}
			if _, err := c.Read(buf); err != nil {
				break
			}
		}
		c.Close()
	})
	t.Cleanup(func() { rootTCPTwoWayServer.Close() })

	// Two TCP servers for the multi-port TCP application in the root cluster.
	rootTCPMultiPortServerAlpha := newTCPServer(t, func(c net.Conn) {
		c.Write([]byte(p.rootTCPMultiPortMessageAlpha))
		c.Close()
	})
	t.Cleanup(func() { rootTCPMultiPortServerAlpha.Close() })
	rootTCPMultiPortServerBeta := newTCPServer(t, func(c net.Conn) {
		c.Write([]byte(p.rootTCPMultiPortMessageBeta))
		c.Close()
	})
	t.Cleanup(func() { rootTCPMultiPortServerBeta.Close() })

	// HTTP server in leaf cluster.
	leafServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, p.leafMessage)
	}))
	t.Cleanup(leafServer.Close)
	// Websockets server in leaf cluster (ws://).
	leafWSServer := httptest.NewServer(createHandler(func(conn *websocket.Conn) {
		conn.WriteMessage(websocket.BinaryMessage, []byte(p.leafWSMessage))
		conn.Close()
	}))
	t.Cleanup(leafWSServer.Close)
	// Secure websockets server in leaf cluster (wss://).
	leafWSSServer := httptest.NewTLSServer(createHandler(func(conn *websocket.Conn) {
		conn.WriteMessage(websocket.BinaryMessage, []byte(p.leafWSSMessage))
		conn.Close()
	}))
	t.Cleanup(leafWSSServer.Close)
	// Plain TCP application in leaf cluster (tcp://).
	leafTCPServer := newTCPServer(t, func(c net.Conn) {
		c.Write([]byte(p.leafTCPMessage))
		c.Close()
	})
	t.Cleanup(func() { leafTCPServer.Close() })

	// Two TCP servers for the multi-port TCP application in the leaf cluster.
	leafTCPMultiPortServerAlpha := newTCPServer(t, func(c net.Conn) {
		c.Write([]byte(p.leafTCPMultiPortMessageAlpha))
		c.Close()
	})
	t.Cleanup(func() { leafTCPMultiPortServerAlpha.Close() })
	leafTCPMultiPortServerBeta := newTCPServer(t, func(c net.Conn) {
		c.Write([]byte(p.leafTCPMultiPortMessageBeta))
		c.Close()
	})
	t.Cleanup(func() { leafTCPMultiPortServerBeta.Close() })

	// JWT server writes generated JWT token in the response.
	jwtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.Header.Get(teleport.AppJWTHeader))
	}))
	t.Cleanup(jwtServer.Close)
	// Websocket header server dumps initial HTTP upgrade request in the response.
	wsHeaderServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		require.NoError(t, err)
		reqDump, err := httputil.DumpRequest(r, false)
		require.NoError(t, err)
		require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, reqDump))
		require.NoError(t, conn.Close())
	}))
	t.Cleanup(wsHeaderServer.Close)
	headerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, headerName := range forwardedHeaderNames {
			fmt.Fprintln(w, r.Header.Get(headerName))
		}
	}))
	t.Cleanup(headerServer.Close)
	// Start test server that will dump all request headers in the response.
	dumperServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Write(w)
	}))
	t.Cleanup(dumperServer.Close)
	flushServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.(http.Hijacker)
		conn, _, err := h.Hijack()
		require.NoError(t, err)
		defer conn.Close()
		data := "HTTP/1.1 200 OK\r\n" +
			"Transfer-Encoding: chunked\r\n" +
			"\r\n" +
			"05\r\n" +
			"hello\r\n"
		fmt.Fprint(conn, data)
		time.Sleep(500 * time.Millisecond)
		data = "05\r\n" +
			"world\r\n" +
			"0\r\n" +
			"\r\n"
		fmt.Fprint(conn, data)
	}))
	t.Cleanup(flushServer.Close)

	rootMultiPortHost, rootTCPMultiPortAppPortAlpha, err := splitHostPort(rootTCPMultiPortServerAlpha.Addr().String())
	require.NoError(t, err)
	_, rootTCPMultiPortAppPortBeta, err := splitHostPort(rootTCPMultiPortServerBeta.Addr().String())
	require.NoError(t, err)

	leafMultiPortHost, leafTCPMultiPortAppPortAlpha, err := splitHostPort(leafTCPMultiPortServerAlpha.Addr().String())
	require.NoError(t, err)
	_, leafTCPMultiPortAppPortBeta, err := splitHostPort(leafTCPMultiPortServerBeta.Addr().String())
	require.NoError(t, err)

	p.rootAppURI = rootServer.URL
	p.rootWSAppURI = rootWSServer.URL
	p.rootWSSAppURI = rootWSSServer.URL
	p.rootTCPAppURI = fmt.Sprintf("tcp://%v", rootTCPServer.Addr().String())
	p.rootTCPTwoWayAppURI = fmt.Sprintf("tcp://%v", rootTCPTwoWayServer.Addr().String())
	p.rootTCPMultiPortAppURI = fmt.Sprintf("tcp://%v", rootMultiPortHost)
	p.rootTCPMultiPortAppPortAlpha = rootTCPMultiPortAppPortAlpha
	p.rootTCPMultiPortAppPortBeta = rootTCPMultiPortAppPortBeta
	p.leafAppURI = leafServer.URL
	p.leafWSAppURI = leafWSServer.URL
	p.leafWSSAppURI = leafWSSServer.URL
	p.leafTCPAppURI = fmt.Sprintf("tcp://%v", leafTCPServer.Addr().String())
	p.leafTCPMultiPortAppURI = fmt.Sprintf("tcp://%v", leafMultiPortHost)
	p.leafTCPMultiPortAppPortAlpha = leafTCPMultiPortAppPortAlpha
	p.leafTCPMultiPortAppPortBeta = leafTCPMultiPortAppPortBeta
	p.jwtAppURI = jwtServer.URL
	p.headerAppURI = headerServer.URL
	p.wsHeaderAppURI = wsHeaderServer.URL
	p.flushAppURI = flushServer.URL
	p.dumperAppURI = dumperServer.URL

	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	// Create a new Teleport instance with passed in configuration.
	rootCfg := helpers.InstanceConfig{
		Clock:       opts.Clock,
		ClusterName: "example.com",
		HostID:      uuid.New().String(),
		NodeName:    helpers.Host,
		Priv:        privateKey,
		Pub:         publicKey,
		Logger:      log,
	}
	if opts.RootClusterListeners != nil {
		rootCfg.Listeners = opts.RootClusterListeners(t, &rootCfg.Fds)
	}
	p.rootCluster = helpers.NewInstance(t, rootCfg)

	// Create a new Teleport instance with passed in configuration.
	leafCfg := helpers.InstanceConfig{
		Clock:       opts.Clock,
		ClusterName: "leaf.example.com",
		HostID:      uuid.New().String(),
		NodeName:    helpers.Host,
		Priv:        privateKey,
		Pub:         publicKey,
		Logger:      log,
	}
	if opts.LeafClusterListeners != nil {
		leafCfg.Listeners = opts.LeafClusterListeners(t, &leafCfg.Fds)
	}
	p.leafCluster = helpers.NewInstance(t, leafCfg)

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.Logger = log
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Auth.Preference.SetDisconnectExpiredCert(true)
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebService = false
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = false
	rcConf.Apps.Enabled = false
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	if opts.RootConfig != nil {
		opts.RootConfig(rcConf)
	}
	rcConf.Clock = opts.Clock

	lcConf := servicecfg.MakeDefaultConfig()
	lcConf.Logger = log
	lcConf.DataDir = t.TempDir()
	lcConf.Auth.Enabled = true
	lcConf.Auth.Preference.SetSecondFactor("off")
	lcConf.Auth.Preference.SetDisconnectExpiredCert(true)
	lcConf.Proxy.Enabled = true
	lcConf.Proxy.DisableWebService = false
	lcConf.Proxy.DisableWebInterface = true
	lcConf.SSH.Enabled = false
	lcConf.Apps.Enabled = false
	lcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	if opts.RootConfig != nil {
		opts.RootConfig(lcConf)
	}
	lcConf.Clock = opts.Clock

	err = p.leafCluster.CreateEx(t, p.rootCluster.Secrets.AsSlice(), lcConf)
	require.NoError(t, err)
	err = p.rootCluster.CreateEx(t, p.leafCluster.Secrets.AsSlice(), rcConf)
	require.NoError(t, err)

	err = p.leafCluster.Start()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, p.leafCluster.StopAll()) })
	err = p.rootCluster.Start()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, p.rootCluster.StopAll()) })

	// At least one rootAppServer should start during the setup
	rootAppServersCount := 1
	p.rootAppServers = p.startRootAppServers(t, rootAppServersCount, opts)

	// At least one leafAppServer should start during the setup
	leafAppServersCount := 1
	p.leafAppServers = p.startLeafAppServers(t, leafAppServersCount, opts)

	// Create user for tests.
	p.initUser(t)

	// Create Web UI session.
	p.initWebSession(t)

	// Initialize cert pool with root CA's.
	p.initCertPool(t)

	// Initialize Teleport client with the user's credentials.
	p.initTeleportClient(t)

	return p
}

var forwardedHeaderNames = []string{
	teleport.AppJWTHeader,
	"X-Forwarded-Proto",
	"X-Forwarded-Host",
	"X-Forwarded-Server",
	"X-Forwarded-For",
	"X-Forwarded-Ssl",
	"X-Forwarded-Port",
}

type appAccessTestFunc func(*Pack, *testing.T)

func bind(p *Pack, fn appAccessTestFunc) func(*testing.T) {
	return func(t *testing.T) {
		fn(p, t)
	}
}

// newTCPServer starts accepting TCP connections and serving them using the
// provided handler. Handlers are expected to close client connections.
// Returns the TCP listener.
func newTCPServer(t *testing.T, handleConn func(net.Conn)) net.Listener {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		for {
			conn, err := listener.Accept()
			if err == nil {
				go handleConn(conn)
			}
			if err != nil && !utils.IsOKNetworkError(err) {
				t.Error(err)
				return
			}
		}
	}()

	return listener
}

// uuidWithLabel returns a random UUID with a specific label in front of it.
// It's mostly used to generate unique messages that various cluster app servers are going to
// respond with. The labels make it easier to differentiate between apps when a connection gets
// routed to the wrong app.
func uuidWithLabel(label string) string {
	return fmt.Sprintf("%s-%s", label, uuid.New().String())
}

func splitHostPort(hostport string) (string, int, error) {
	host, portString, err := net.SplitHostPort(hostport)
	if err != nil {
		return "", 0, trace.Wrap(err)
	}

	port, err := strconv.ParseUint(portString, 10, 16)
	if err != nil {
		return "", 0, trace.Wrap(err)
	}

	return host, int(port), nil
}
