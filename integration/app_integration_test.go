/*
Copyright 2020-2021 Gravitational, Inc.

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

package integration

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/server"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testlog"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/app"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/websocket"
)

// TestAppAccessForward tests that requests get forwarded to the target application
// within a single cluster and trusted cluster.
func TestAppAccessForward(t *testing.T) {
	// Create cluster, user, sessions, and credentials package.
	pack := setup(t)

	tests := []struct {
		desc          string
		inCookie      string
		outStatusCode int
		outMessage    string
	}{
		{
			desc:          "root cluster, valid application session cookie, success",
			inCookie:      pack.createAppSession(t, pack.rootAppPublicAddr, pack.rootAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    pack.rootMessage,
		},
		{
			desc:          "leaf cluster, valid application session cookie, success",
			inCookie:      pack.createAppSession(t, pack.leafAppPublicAddr, pack.leafAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    pack.leafMessage,
		},
		{
			desc:          "invalid application session cookie, redirect to login",
			inCookie:      "D25C463CD27861559CC6A0A6AE54818079809AA8731CB18037B4B37A80C4FC6C",
			outStatusCode: http.StatusFound,
			outMessage:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tt := tt
			status, body, err := pack.makeRequest(tt.inCookie, http.MethodGet, "/")
			require.NoError(t, err)
			require.Equal(t, tt.outStatusCode, status)
			require.Contains(t, body, tt.outMessage)
		})
	}
}

// TestAppAccessWebsockets makes sure that websocket requests get forwarded.
func TestAppAccessWebsockets(t *testing.T) {
	// Create cluster, user, sessions, and credentials package.
	pack := setup(t)

	tests := []struct {
		desc       string
		inCookie   string
		outMessage string
		err        error
	}{
		{
			desc:       "root cluster, valid application session cookie, successful websocket (ws://) request",
			inCookie:   pack.createAppSession(t, pack.rootWSPublicAddr, pack.rootAppClusterName),
			outMessage: pack.rootWSMessage,
		},
		{
			desc:       "root cluster, valid application session cookie, successful secure websocket (wss://) request",
			inCookie:   pack.createAppSession(t, pack.rootWSSPublicAddr, pack.rootAppClusterName),
			outMessage: pack.rootWSSMessage,
		},
		{
			desc:       "leaf cluster, valid application session cookie, successful websocket (ws://) request",
			inCookie:   pack.createAppSession(t, pack.leafWSPublicAddr, pack.leafAppClusterName),
			outMessage: pack.leafWSMessage,
		},
		{
			desc:       "leaf cluster, valid application session cookie, successful secure websocket (wss://) request",
			inCookie:   pack.createAppSession(t, pack.leafWSSPublicAddr, pack.leafAppClusterName),
			outMessage: pack.leafWSSMessage,
		},
		{
			desc:     "invalid application session cookie, websocket request fails to dial",
			inCookie: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			err:      &websocket.DialError{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tt := tt
			body, err := pack.makeWebsocketRequest(tt.inCookie, "/")
			if tt.err != nil {
				require.IsType(t, tt.err, trace.Unwrap(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.outMessage, body)
			}
		})
	}
}

// TestAppAccessClientCert tests mutual TLS authentication flow with application
// access typically used in CLI by curl and other clients.
func TestAppAccessClientCert(t *testing.T) {
	pack := setup(t)

	tests := []struct {
		desc          string
		inTLSConfig   *tls.Config
		outStatusCode int
		outMessage    string
	}{
		{
			desc:          "root cluster, valid TLS config, success",
			inTLSConfig:   pack.makeTLSConfig(t, pack.rootAppPublicAddr, pack.rootAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    pack.rootMessage,
		},
		{
			desc:          "leaf cluster, valid TLS config, success",
			inTLSConfig:   pack.makeTLSConfig(t, pack.leafAppPublicAddr, pack.leafAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    pack.leafMessage,
		},
		{
			desc:          "root cluster, invalid session ID",
			inTLSConfig:   pack.makeTLSConfigNoSession(t, pack.rootAppPublicAddr, pack.rootAppClusterName),
			outStatusCode: http.StatusFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tt := tt
			status, body, err := pack.makeRequestWithClientCert(tt.inTLSConfig, http.MethodGet, "/")
			require.NoError(t, err)
			require.Equal(t, tt.outStatusCode, status)
			require.Contains(t, body, tt.outMessage)
		})
	}
}

// TestAppAccessForwardModes ensures that requests are forwarded to applications
// even when the cluster is in proxy recording mode.
func TestAppAccessForwardModes(t *testing.T) {
	// Create cluster, user, sessions, and credentials package.
	pack := setup(t)

	// Update root and leaf clusters to record sessions at the proxy.
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording: services.RecordAtProxy,
	})
	require.NoError(t, err)
	err = pack.rootCluster.Process.GetAuthServer().SetClusterConfig(clusterConfig)
	require.NoError(t, err)
	err = pack.leafCluster.Process.GetAuthServer().SetClusterConfig(clusterConfig)
	require.NoError(t, err)

	// Requests to root and leaf cluster are successful.
	tests := []struct {
		desc          string
		inCookie      string
		outStatusCode int
		outMessage    string
	}{
		{
			desc:          "root cluster, valid application session cookie, success",
			inCookie:      pack.createAppSession(t, pack.rootAppPublicAddr, pack.rootAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    pack.rootMessage,
		},
		{
			desc:          "leaf cluster, valid application session cookie, success",
			inCookie:      pack.createAppSession(t, pack.leafAppPublicAddr, pack.leafAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    pack.leafMessage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tt := tt
			status, body, err := pack.makeRequest(tt.inCookie, http.MethodGet, "/")
			require.NoError(t, err)
			require.Equal(t, tt.outStatusCode, status)
			require.Contains(t, body, tt.outMessage)
		})
	}
}

// TestAppAccessLogout verifies the session is removed from the backend when the user logs out.
func TestAppAccessLogout(t *testing.T) {
	// Create cluster, user, and credentials package.
	pack := setup(t)

	// Create an application session.
	appCookie := pack.createAppSession(t, pack.rootAppPublicAddr, pack.rootAppClusterName)

	// Log user out of session.
	status, _, err := pack.makeRequest(appCookie, http.MethodGet, "/teleport-logout")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Wait until requests using the session cookie have failed.
	status, err = pack.waitForLogout(appCookie)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, status)
}

// TestAppAccessJWT ensures a JWT token is attached to requests and the JWT token can
// be validated.
func TestAppAccessJWT(t *testing.T) {
	// Create cluster, user, and credentials package.
	pack := setup(t)

	// Create an application session.
	appCookie := pack.createAppSession(t, pack.jwtAppPublicAddr, pack.jwtAppClusterName)

	// Get JWT.
	status, token, err := pack.makeRequest(appCookie, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Get and unmarshal JWKs
	status, body, err := pack.makeRequest("", http.MethodGet, "/.well-known/jwks.json")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	var jwks web.JWKSResponse
	err = json.Unmarshal([]byte(body), &jwks)
	require.NoError(t, err)
	require.Len(t, jwks.Keys, 1)
	publicKey, err := jwt.UnmarshalJWK(jwks.Keys[0])
	require.NoError(t, err)

	// Verify JWT.
	key, err := jwt.New(&jwt.Config{
		PublicKey:   publicKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: pack.jwtAppClusterName,
	})
	require.NoError(t, err)
	claims, err := key.Verify(jwt.VerifyParams{
		Username: pack.username,
		RawToken: token,
		URI:      pack.jwtAppURI,
	})
	require.NoError(t, err)
	require.Equal(t, pack.username, claims.Username)
	require.Equal(t, pack.user.GetRoles(), claims.Roles)
}

// TestAppAccessNoHeaderOverrides ensures that AAP-specific headers cannot be overridden
// by values passed in by the user.
func TestAppAccessNoHeaderOverrides(t *testing.T) {
	// Create cluster, user, and credentials package.
	pack := setup(t)

	// Create an application session.
	appCookie := pack.createAppSession(t, pack.headerAppPublicAddr, pack.headerAppClusterName)

	// Get HTTP headers forwarded to the application.
	status, origHeaderResp, err := pack.makeRequest(appCookie, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	origHeaders := strings.Split(origHeaderResp, "\n")
	require.Equal(t, len(origHeaders), len(forwardedHeaderNames)+1)

	// Construct HTTP request with custom headers.
	req, err := http.NewRequest(http.MethodGet, pack.assembleRootProxyURL("/"), nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{
		Name:  app.CookieName,
		Value: appCookie,
	})
	for _, headerName := range forwardedHeaderNames {
		req.Header.Set(headerName, uuid.New())
	}

	// Issue the request.
	status, newHeaderResp, err := pack.sendRequest(req, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	newHeaders := strings.Split(newHeaderResp, "\n")
	require.Equal(t, len(newHeaders), len(forwardedHeaderNames)+1)

	// Headers sent to the application should not be affected.
	for i := range forwardedHeaderNames {
		require.Equal(t, origHeaders[i], newHeaders[i])
	}
}

// pack contains identity as well as initialized Teleport clusters and instances.
type pack struct {
	username string
	password string

	tc *client.TeleportClient

	user services.User

	webCookie string
	webToken  string

	rootCluster   *TeleInstance
	rootAppServer *service.TeleportProcess
	rootCertPool  *x509.CertPool

	rootAppName        string
	rootAppPublicAddr  string
	rootAppClusterName string
	rootMessage        string

	rootWSAppName    string
	rootWSPublicAddr string
	rootWSMessage    string

	rootWSSAppName    string
	rootWSSPublicAddr string
	rootWSSMessage    string

	jwtAppName        string
	jwtAppPublicAddr  string
	jwtAppClusterName string
	jwtAppURI         string

	leafCluster   *TeleInstance
	leafAppServer *service.TeleportProcess

	leafAppName        string
	leafAppPublicAddr  string
	leafAppClusterName string
	leafMessage        string

	leafWSAppName    string
	leafWSPublicAddr string
	leafWSMessage    string

	leafWSSAppName    string
	leafWSSPublicAddr string
	leafWSSMessage    string

	headerAppName        string
	headerAppPublicAddr  string
	headerAppClusterName string
}

// setup configures all clusters and servers needed for a test.
func setup(t *testing.T) *pack {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	log := testlog.FailureOnly(t)

	// Insecure development mode needs to be set because the web proxy uses a
	// self-signed certificate during tests.
	lib.SetInsecureDevMode(true)

	SetTestTimeouts(time.Millisecond * time.Duration(500))

	p := &pack{
		rootAppName:        "app-01",
		rootAppPublicAddr:  "app-01.example.com",
		rootAppClusterName: "example.com",
		rootMessage:        uuid.New(),

		rootWSAppName:    "ws-01",
		rootWSPublicAddr: "ws-01.example.com",
		rootWSMessage:    uuid.New(),

		rootWSSAppName:    "wss-01",
		rootWSSPublicAddr: "wss-01.example.com",
		rootWSSMessage:    uuid.New(),

		leafAppName:        "app-02",
		leafAppPublicAddr:  "app-02.example.com",
		leafAppClusterName: "leaf.example.com",
		leafMessage:        uuid.New(),

		leafWSAppName:    "ws-02",
		leafWSPublicAddr: "ws-02.example.com",
		leafWSMessage:    uuid.New(),

		leafWSSAppName:    "wss-02",
		leafWSSPublicAddr: "wss-02.example.com",
		leafWSSMessage:    uuid.New(),

		jwtAppName:        "app-03",
		jwtAppPublicAddr:  "app-03.example.com",
		jwtAppClusterName: "example.com",

		headerAppName:        "app-04",
		headerAppPublicAddr:  "app-04.example.com",
		headerAppClusterName: "example.com",
	}

	// Start a few different HTTP server that will be acting like a proxied application. The first two applications
	rootServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, p.rootMessage)
	}))
	t.Cleanup(rootServer.Close)
	// Websockets server in root cluster (ws://).
	rootWSServer := httptest.NewServer(websocket.Handler(func(conn *websocket.Conn) {
		conn.Write([]byte(p.rootWSMessage))
		conn.Close()
	}))
	t.Cleanup(rootWSServer.Close)
	// Secure websockets server in root cluster (wss://).
	rootWSSServer := httptest.NewTLSServer(websocket.Handler(func(conn *websocket.Conn) {
		conn.Write([]byte(p.rootWSSMessage))
		conn.Close()
	}))
	t.Cleanup(rootWSSServer.Close)
	leafServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, p.leafMessage)
	}))
	t.Cleanup(leafServer.Close)
	// Websockets server in leaf cluster (ws://).
	leafWSServer := httptest.NewServer(websocket.Handler(func(conn *websocket.Conn) {
		conn.Write([]byte(p.leafWSMessage))
		conn.Close()
	}))
	t.Cleanup(leafWSServer.Close)
	// Secure websockets server in leaf cluster (wss://).
	leafWSSServer := httptest.NewTLSServer(websocket.Handler(func(conn *websocket.Conn) {
		conn.Write([]byte(p.leafWSSMessage))
		conn.Close()
	}))
	t.Cleanup(leafWSSServer.Close)
	jwtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.Header.Get(teleport.AppJWTHeader))
	}))
	t.Cleanup(jwtServer.Close)
	headerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, headerName := range forwardedHeaderNames {
			fmt.Fprintln(w, r.Header.Get(headerName))
		}
	}))
	t.Cleanup(headerServer.Close)

	p.jwtAppURI = jwtServer.URL

	privateKey, publicKey, err := testauthority.New().GenerateKeyPair("")
	require.NoError(t, err)

	// Find AllocatePortsNum free listening ports to use.
	startNumber := utils.PortStartingNumber + (AllocatePortsNum * 2) + 1
	ports, err := utils.GetFreeTCPPorts(AllocatePortsNum, startNumber+1)
	require.NoError(t, err)

	// Create a new Teleport instance with passed in configuration.
	p.rootCluster = NewInstance(InstanceConfig{
		ClusterName: "example.com",
		HostID:      uuid.New(),
		NodeName:    Host,
		Ports:       ports.PopIntSlice(6),
		Priv:        privateKey,
		Pub:         publicKey,
		log:         log,
	})

	// Create a new Teleport instance with passed in configuration.
	p.leafCluster = NewInstance(InstanceConfig{
		ClusterName: "leaf.example.com",
		HostID:      uuid.New(),
		NodeName:    Host,
		Ports:       ports.PopIntSlice(6),
		Priv:        privateKey,
		Pub:         publicKey,
		log:         log,
	})

	rcConf := service.MakeDefaultConfig()
	rcConf.Console = nil
	rcConf.Log = log
	rcConf.DataDir, err = ioutil.TempDir("", "cluster-"+p.rootCluster.Secrets.SiteName)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(rcConf.DataDir) })
	rcConf.Auth.Enabled = true
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebService = false
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = false
	rcConf.Apps.Enabled = false

	lcConf := service.MakeDefaultConfig()
	lcConf.Console = nil
	lcConf.Log = log
	lcConf.DataDir, err = ioutil.TempDir("", "cluster-"+p.leafCluster.Secrets.SiteName)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(lcConf.DataDir) })
	lcConf.Auth.Enabled = true
	lcConf.Auth.Preference.SetSecondFactor("off")
	lcConf.Proxy.Enabled = true
	lcConf.Proxy.DisableWebService = false
	lcConf.Proxy.DisableWebInterface = true
	lcConf.SSH.Enabled = false
	lcConf.Apps.Enabled = false

	err = p.leafCluster.CreateEx(p.rootCluster.Secrets.AsSlice(), lcConf)
	require.NoError(t, err)
	err = p.rootCluster.CreateEx(p.leafCluster.Secrets.AsSlice(), rcConf)
	require.NoError(t, err)

	err = p.leafCluster.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		p.leafCluster.StopAll()
	})
	err = p.rootCluster.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		p.rootCluster.StopAll()
	})

	raConf := service.MakeDefaultConfig()
	raConf.Console = nil
	raConf.Log = log
	raConf.DataDir, err = ioutil.TempDir("", "app-server-"+p.rootCluster.Secrets.SiteName)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(raConf.DataDir) })
	raConf.Token = "static-token-value"
	raConf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        net.JoinHostPort(Loopback, p.rootCluster.GetPortWeb()),
		},
	}
	raConf.Auth.Enabled = false
	raConf.Proxy.Enabled = false
	raConf.SSH.Enabled = false
	raConf.Apps.Enabled = true
	raConf.Apps.Apps = []service.App{
		{
			Name:       p.rootAppName,
			URI:        rootServer.URL,
			PublicAddr: p.rootAppPublicAddr,
		},
		{
			Name:       p.rootWSAppName,
			URI:        rootWSServer.URL,
			PublicAddr: p.rootWSPublicAddr,
		},
		{
			Name:       p.rootWSSAppName,
			URI:        rootWSSServer.URL,
			PublicAddr: p.rootWSSPublicAddr,
		},
		{
			Name:       p.jwtAppName,
			URI:        jwtServer.URL,
			PublicAddr: p.jwtAppPublicAddr,
		},
		{
			Name:       p.headerAppName,
			URI:        headerServer.URL,
			PublicAddr: p.headerAppPublicAddr,
		},
	}
	p.rootAppServer, err = p.rootCluster.StartApp(raConf)
	require.NoError(t, err)
	t.Cleanup(func() { p.rootAppServer.Close() })

	laConf := service.MakeDefaultConfig()
	laConf.Console = nil
	laConf.Log = log
	laConf.DataDir, err = ioutil.TempDir("", "app-server-"+p.leafCluster.Secrets.SiteName)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(laConf.DataDir) })
	laConf.Token = "static-token-value"
	laConf.AuthServers = []utils.NetAddr{
		{
			AddrNetwork: "tcp",
			Addr:        net.JoinHostPort(Loopback, p.leafCluster.GetPortWeb()),
		},
	}
	laConf.Auth.Enabled = false
	laConf.Proxy.Enabled = false
	laConf.SSH.Enabled = false
	laConf.Apps.Enabled = true
	laConf.Apps.Apps = []service.App{
		{
			Name:       p.leafAppName,
			URI:        leafServer.URL,
			PublicAddr: p.leafAppPublicAddr,
		},
		{
			Name:       p.leafWSAppName,
			URI:        leafWSServer.URL,
			PublicAddr: p.leafWSPublicAddr,
		},
		{
			Name:       p.leafWSSAppName,
			URI:        leafWSSServer.URL,
			PublicAddr: p.leafWSSPublicAddr,
		},
	}
	p.leafAppServer, err = p.leafCluster.StartApp(laConf)
	require.NoError(t, err)
	t.Cleanup(func() { p.leafAppServer.Close() })

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

// initUser will create a user within the root cluster.
func (p *pack) initUser(t *testing.T) {
	p.username = uuid.New()
	p.password = uuid.New()

	user, err := services.NewUser(p.username)
	require.NoError(t, err)

	role := auth.RoleForUser(user)
	role.SetLogins(services.Allow, []string{p.username})
	err = p.rootCluster.Process.GetAuthServer().UpsertRole(context.Background(), role)
	require.NoError(t, err)

	user.AddRole(role.GetName())
	err = p.rootCluster.Process.GetAuthServer().CreateUser(context.Background(), user)
	require.NoError(t, err)

	err = p.rootCluster.Process.GetAuthServer().UpsertPassword(user.GetName(), []byte(p.password))
	require.NoError(t, err)

	p.user = user
}

// initWebSession creates a Web UI session within the root cluster.
func (p *pack) initWebSession(t *testing.T) {
	csReq, err := json.Marshal(web.CreateSessionReq{
		User: p.username,
		Pass: p.password,
	})
	require.NoError(t, err)

	// Create POST request to create session.
	u := url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(Loopback, p.rootCluster.GetPortWeb()),
		Path:   "/v1/webapi/sessions/web",
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(csReq))
	require.NoError(t, err)

	// Attach CSRF token in cookie and header.
	csrfToken, err := utils.CryptoRandomHex(32)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{
		Name:  csrf.CookieName,
		Value: csrfToken,
	})
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set(csrf.HeaderName, csrfToken)

	// Issue request.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Read in response.
	var csResp *web.CreateSessionResponse
	err = json.NewDecoder(resp.Body).Decode(&csResp)
	require.NoError(t, err)

	// Extract session cookie and bearer token.
	require.Len(t, resp.Cookies(), 1)
	cookie := resp.Cookies()[0]
	require.Equal(t, cookie.Name, web.CookieName)

	p.webCookie = cookie.Value
	p.webToken = csResp.Token
}

// initTeleportClient initializes a Teleport client with this pack's user
// credentials.
func (p *pack) initTeleportClient(t *testing.T) {
	creds, err := GenerateUserCreds(UserCredsRequest{
		Process:  p.rootCluster.Process,
		Username: p.user.GetName(),
	})
	require.NoError(t, err)

	tc, err := p.rootCluster.NewClientWithCreds(ClientConfig{
		Login:   p.user.GetName(),
		Cluster: p.rootCluster.Secrets.SiteName,
		Host:    Loopback,
		Port:    p.rootCluster.GetPortSSHInt(),
	}, *creds)
	require.NoError(t, err)

	p.tc = tc
}

// createAppSession creates an application session with the root cluster. The
// application that the user connects to may be running in a leaf cluster.
func (p *pack) createAppSession(t *testing.T, publicAddr, clusterName string) string {
	require.NotEmpty(t, p.webCookie)
	require.NotEmpty(t, p.webToken)

	casReq, err := json.Marshal(web.CreateAppSessionRequest{
		FQDN:        publicAddr,
		PublicAddr:  publicAddr,
		ClusterName: clusterName,
	})
	require.NoError(t, err)

	u := url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(Loopback, p.rootCluster.GetPortWeb()),
		Path:   "/v1/webapi/sessions/app",
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(casReq))
	require.NoError(t, err)

	req.AddCookie(&http.Cookie{
		Name:  web.CookieName,
		Value: p.webCookie,
	})
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", p.webToken))

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var casResp *web.CreateAppSessionResponse
	err = json.NewDecoder(resp.Body).Decode(&casResp)
	require.NoError(t, err)

	return casResp.CookieValue
}

// initCertPool initializes root cluster CA pool.
func (p *pack) initCertPool(t *testing.T) {
	authClient := p.rootCluster.GetSiteAPI(p.rootCluster.Secrets.SiteName)
	ca, err := authClient.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: p.rootCluster.Secrets.SiteName,
	}, false)
	require.NoError(t, err)

	pool, err := auth.CertPool(ca)
	require.NoError(t, err)

	p.rootCertPool = pool
}

// makeTLSConfig returns TLS config suitable for making an app access request.
func (p *pack) makeTLSConfig(t *testing.T, publicAddr, clusterName string) *tls.Config {
	privateKey, publicKey, err := p.rootCluster.Process.GetAuthServer().GenerateKeyPair("")
	require.NoError(t, err)

	ws, err := p.tc.CreateAppSession(context.Background(), types.CreateAppSessionRequest{
		Username:    p.user.GetName(),
		PublicAddr:  publicAddr,
		ClusterName: clusterName,
	})
	require.NoError(t, err)

	certificate, err := p.rootCluster.Process.GetAuthServer().GenerateUserAppTestCert(
		server.AppTestCertRequest{
			PublicKey:   publicKey,
			Username:    p.user.GetName(),
			TTL:         time.Hour,
			PublicAddr:  publicAddr,
			ClusterName: clusterName,
			SessionID:   ws.GetName(),
		})
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(certificate, privateKey)
	require.NoError(t, err)

	return &tls.Config{
		RootCAs:            p.rootCertPool,
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
	}
}

// makeTLSConfigNoSession returns TLS config for application access without
// creating session to simulate nonexistent session scenario.
func (p *pack) makeTLSConfigNoSession(t *testing.T, publicAddr, clusterName string) *tls.Config {
	privateKey, publicKey, err := p.rootCluster.Process.GetAuthServer().GenerateKeyPair("")
	require.NoError(t, err)

	certificate, err := p.rootCluster.Process.GetAuthServer().GenerateUserAppTestCert(
		server.AppTestCertRequest{
			PublicKey:   publicKey,
			Username:    p.user.GetName(),
			TTL:         time.Hour,
			PublicAddr:  publicAddr,
			ClusterName: clusterName,
			// Use arbitrary session ID
			SessionID: uuid.New(),
		})
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(certificate, privateKey)
	require.NoError(t, err)

	return &tls.Config{
		RootCAs:            p.rootCertPool,
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
	}
}

// makeRequest makes a request to the root cluster with the given session cookie.
func (p *pack) makeRequest(sessionCookie string, method string, endpoint string) (int, string, error) {
	req, err := http.NewRequest(method, p.assembleRootProxyURL(endpoint), nil)
	if err != nil {
		return 0, "", trace.Wrap(err)
	}

	// Only attach session cookie if passed in.
	if sessionCookie != "" {
		req.AddCookie(&http.Cookie{
			Name:  app.CookieName,
			Value: sessionCookie,
		})
	}

	return p.sendRequest(req, nil)
}

// makeRequestWithClientCert makes a request to the root cluster using the
// client certificate authentication from the provided tls config.
func (p *pack) makeRequestWithClientCert(tlsConfig *tls.Config, method, endpoint string) (int, string, error) {
	req, err := http.NewRequest(method, p.assembleRootProxyURL(endpoint), nil)
	if err != nil {
		return 0, "", trace.Wrap(err)
	}
	return p.sendRequest(req, tlsConfig)
}

// makeWebsocketRequest makes a websocket request with the given session cookie.
func (p *pack) makeWebsocketRequest(sessionCookie, endpoint string) (string, error) {
	config, err := websocket.NewConfig(
		fmt.Sprintf("wss://%s%s", net.JoinHostPort(Loopback, p.rootCluster.GetPortWeb()), endpoint),
		"https://localhost")
	if err != nil {
		return "", trace.Wrap(err)
	}
	if sessionCookie != "" {
		config.Header.Set("Cookie", (&http.Cookie{
			Name:  app.CookieName,
			Value: sessionCookie,
		}).String())
	}
	config.TlsConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	conn, err := websocket.DialConfig(config)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer conn.Close()
	data, err := ioutil.ReadAll(conn)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(data), nil
}

// assembleRootProxyURL returns the URL string of an endpoint at the root
// cluster's proxy web.
func (p *pack) assembleRootProxyURL(endpoint string) string {
	u := url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(Loopback, p.rootCluster.GetPortWeb()),
		Path:   endpoint,
	}
	return u.String()
}

// sendReqeust sends the request to the root cluster.
func (p *pack) sendRequest(req *http.Request, tlsConfig *tls.Config) (int, string, error) {
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", trace.Wrap(err)
	}
	defer resp.Body.Close()

	// Read in response body.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, "", trace.Wrap(err)
	}

	return resp.StatusCode, string(body), nil
}

// waitForLogout keeps making request with the passed in session cookie until
// they return a non-200 status.
func (p *pack) waitForLogout(appCookie string) (int, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-ticker.C:
			status, _, err := p.makeRequest(appCookie, http.MethodGet, "/")
			if err != nil {
				return 0, trace.Wrap(err)
			}
			if status != http.StatusOK {
				return status, nil
			}
		case <-timeout.C:
			return 0, trace.BadParameter("timed out waiting for logout")
		}
	}
}

var forwardedHeaderNames = []string{
	teleport.AppJWTHeader,
	teleport.AppCFHeader,
	"X-Forwarded-Proto",
	"X-Forwarded-Host",
	"X-Forwarded-Server",
	"X-Forwarded-For",
}
