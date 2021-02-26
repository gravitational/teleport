/*
Copyright 2020 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/testauthority"
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
)

// TestForward tests that requests get forwarded to the target application
// within a single cluster and trusted cluster.
func TestForward(t *testing.T) {
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

// TestForwardModes ensures that requests are forwarded to applications even
// when the cluster is in proxy recording mode.
func TestForwardModes(t *testing.T) {
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

// TestLogout verifies the session is removed from the backend when the user logs out.
func TestLogout(t *testing.T) {
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

// TestJWT ensures a JWT token is attached to requests and the JWT token can
// be validated.
func TestJWT(t *testing.T) {
	// Create cluster, user, and credentials package.
	pack := setup(t)

	// Create an application session.
	appCookie := pack.createAppSession(t, pack.jwtAppPublicAddr, pack.jwtAppClusterName)

	// Log user out of session.
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

// pack contains identity as well as initialized Teleport clusters and instances.
type pack struct {
	username string
	password string

	user services.User

	webCookie string
	webToken  string

	rootCluster   *TeleInstance
	rootAppServer *service.TeleportProcess

	rootAppName        string
	rootAppPublicAddr  string
	rootAppClusterName string
	rootMessage        string

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

		leafAppName:        "app-02",
		leafAppPublicAddr:  "app-02.example.com",
		leafAppClusterName: "leaf.example.com",
		leafMessage:        uuid.New(),

		jwtAppName:        "app-03",
		jwtAppPublicAddr:  "app-03.example.com",
		jwtAppClusterName: "example.com",
	}

	// Start a few different HTTP server that will be acting like a proxied application. The first two applications
	rootServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, p.rootMessage)
	}))
	t.Cleanup(rootServer.Close)
	leafServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, p.leafMessage)
	}))
	t.Cleanup(leafServer.Close)
	jwtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, r.Header.Get(teleport.AppJWTHeader))
	}))
	t.Cleanup(jwtServer.Close)

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
			Name:       p.jwtAppName,
			URI:        jwtServer.URL,
			PublicAddr: p.jwtAppPublicAddr,
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
	}
	p.leafAppServer, err = p.leafCluster.StartApp(laConf)
	require.NoError(t, err)
	t.Cleanup(func() { p.leafAppServer.Close() })

	// Create user for tests.
	p.createUser(t)

	// Create Web UI session.
	p.createWebSession(t)

	return p
}

// createUser will create a user within the root cluster.
func (p *pack) createUser(t *testing.T) {
	p.username = uuid.New()
	p.password = uuid.New()

	user, err := services.NewUser(p.username)
	require.NoError(t, err)

	role := services.RoleForUser(user)
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

// createWebSession creates a Web UI session within the root cluster.
func (p *pack) createWebSession(t *testing.T) {
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

// makeRequest makes a request to the root cluster with the given session cookie.
func (p *pack) makeRequest(sessionCookie string, method string, endpoint string) (int, string, error) {
	u := url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(Loopback, p.rootCluster.GetPortWeb()),
		Path:   endpoint,
	}
	req, err := http.NewRequest(method, u.String(), nil)
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

	// Issue request.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
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
