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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/app/common"
	libmcp "github.com/gravitational/teleport/lib/srv/mcp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/app"
	websession "github.com/gravitational/teleport/lib/web/session"
)

// Pack contains identity as well as initialized Teleport clusters and instances.
type Pack struct {
	username string
	password string

	tc *client.TeleportClient

	user types.User

	webCookie string
	webToken  string

	rootCluster    *helpers.TeleInstance
	rootAppServers []*service.TeleportProcess
	rootCertPool   *x509.CertPool

	rootAppName        string
	rootAppPublicAddr  string
	rootAppClusterName string
	rootMessage        string
	rootAppURI         string

	rootWSAppName    string
	rootWSPublicAddr string
	rootWSMessage    string
	rootWSAppURI     string

	rootWSSAppName    string
	rootWSSPublicAddr string
	rootWSSMessage    string
	rootWSSAppURI     string

	rootTCPAppName    string
	rootTCPPublicAddr string
	rootTCPMessage    string
	rootTCPAppURI     string

	rootTCPTwoWayAppName    string
	rootTCPTwoWayPublicAddr string
	rootTCPTwoWayMessage    string
	rootTCPTwoWayAppURI     string

	rootTCPMultiPortAppName      string
	rootTCPMultiPortPublicAddr   string
	rootTCPMultiPortMessageAlpha string
	rootTCPMultiPortMessageBeta  string
	rootTCPMultiPortAppURI       string
	rootTCPMultiPortAppPortAlpha int
	rootTCPMultiPortAppPortBeta  int

	jwtAppName        string
	jwtAppPublicAddr  string
	jwtAppClusterName string
	jwtAppURI         string

	dumperAppURI string

	leafCluster    *helpers.TeleInstance
	leafAppServers []*service.TeleportProcess

	leafAppName        string
	leafAppPublicAddr  string
	leafAppClusterName string
	leafMessage        string
	leafAppURI         string

	leafWSAppName    string
	leafWSPublicAddr string
	leafWSMessage    string
	leafWSAppURI     string

	leafWSSAppName    string
	leafWSSPublicAddr string
	leafWSSMessage    string
	leafWSSAppURI     string

	leafTCPAppName    string
	leafTCPPublicAddr string
	leafTCPMessage    string
	leafTCPAppURI     string

	leafTCPMultiPortAppName      string
	leafTCPMultiPortPublicAddr   string
	leafTCPMultiPortMessageAlpha string
	leafTCPMultiPortMessageBeta  string
	leafTCPMultiPortAppURI       string
	leafTCPMultiPortAppPortAlpha int
	leafTCPMultiPortAppPortBeta  int

	headerAppName        string
	headerAppPublicAddr  string
	headerAppClusterName string
	headerAppURI         string

	wsHeaderAppName        string
	wsHeaderAppPublicAddr  string
	wsHeaderAppClusterName string
	wsHeaderAppURI         string

	flushAppName        string
	flushAppPublicAddr  string
	flushAppClusterName string
	flushAppURI         string
}

func (p *Pack) RootWebAddr() string {
	return p.rootCluster.Web
}

func (p *Pack) RootAppName() string {
	return p.rootAppName
}

func (p *Pack) RootAppClusterName() string {
	return p.rootAppClusterName
}

func (p *Pack) RootAppPublicAddr() string {
	return p.rootAppPublicAddr
}

func (p *Pack) RootTCPAppName() string {
	return p.rootTCPAppName
}

func (p *Pack) RootTCPMessage() string {
	return p.rootTCPMessage
}

func (p *Pack) RootTCPMultiPortAppName() string {
	return p.rootTCPMultiPortAppName
}

func (p *Pack) RootTCPMultiPortAppPortAlpha() int {
	return p.rootTCPMultiPortAppPortAlpha
}

func (p *Pack) RootTCPMultiPortMessageAlpha() string {
	return p.rootTCPMultiPortMessageAlpha
}

func (p *Pack) RootTCPMultiPortAppPortBeta() int {
	return p.rootTCPMultiPortAppPortBeta
}

func (p *Pack) RootTCPMultiPortMessageBeta() string {
	return p.rootTCPMultiPortMessageBeta
}

func (p *Pack) RootAuthServer() *auth.Server {
	return p.rootCluster.Process.GetAuthServer()
}

func (p *Pack) LeafAppName() string {
	return p.leafAppName
}

func (p *Pack) LeafAppClusterName() string {
	return p.leafAppClusterName
}

func (p *Pack) LeafAppPublicAddr() string {
	return p.leafAppPublicAddr
}

func (p *Pack) LeafTCPAppName() string {
	return p.leafTCPAppName
}

func (p *Pack) LeafTCPMessage() string {
	return p.leafTCPMessage
}

func (p *Pack) LeafTCPMultiPortAppName() string {
	return p.leafTCPMultiPortAppName
}

func (p *Pack) LeafTCPMultiPortAppPortAlpha() int {
	return p.leafTCPMultiPortAppPortAlpha
}

func (p *Pack) LeafTCPMultiPortMessageAlpha() string {
	return p.leafTCPMultiPortMessageAlpha
}

func (p *Pack) LeafTCPMultiPortAppPortBeta() int {
	return p.leafTCPMultiPortAppPortBeta
}

func (p *Pack) LeafTCPMultiPortMessageBeta() string {
	return p.leafTCPMultiPortMessageBeta
}

func (p *Pack) LeafAuthServer() *auth.Server {
	return p.leafCluster.Process.GetAuthServer()
}

// initUser will create a user within the root cluster.
func (p *Pack) initUser(t *testing.T) {
	p.user, p.password = p.CreateUser(t)
	p.username = p.user.GetName()
}

// CreateUser creates and upserts a new user into the root cluster, and returns the new user and password.
func (p *Pack) CreateUser(t *testing.T) (types.User, string) {
	username := uuid.New().String()
	password := uuid.New().String()

	user, err := types.NewUser(username)
	require.NoError(t, err)

	role := services.RoleForUser(user)
	role.SetLogins(types.Allow, []string{username, "root", "ubuntu"})
	role, err = p.rootCluster.Process.GetAuthServer().UpsertRole(context.Background(), role)
	require.NoError(t, err)

	user.AddRole(role.GetName())
	user.SetTraits(map[string][]string{"env": {"production"}, "empty": {}, "nil": nil})
	user, err = p.rootCluster.Process.GetAuthServer().CreateUser(context.Background(), user)
	require.NoError(t, err)

	err = p.rootCluster.Process.GetAuthServer().UpsertPassword(user.GetName(), []byte(password))
	require.NoError(t, err)
	return user, password
}

// initWebSession creates a Web UI session within the root cluster.
func (p *Pack) initWebSession(t *testing.T) {
	csReq, err := json.Marshal(web.CreateSessionReq{
		User: p.username,
		Pass: p.password,
	})
	require.NoError(t, err)

	// Create POST request to create session.
	u := url.URL{
		Scheme: "https",
		Host:   p.rootCluster.Web,
		Path:   "/v1/webapi/sessions/web",
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(csReq))
	require.NoError(t, err)

	// Set Content-Type header, otherwise Teleport's CSRF protection will
	// reject the request.
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

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
	require.Equal(t, websession.CookieName, cookie.Name)

	p.webCookie = cookie.Value
	p.webToken = csResp.Token
}

// initTeleportClient initializes a Teleport client with this pack's user
// credentials.
func (p *Pack) initTeleportClient(t *testing.T) {
	p.tc = p.MakeTeleportClient(t, p.username)
}

func (p *Pack) MakeTeleportClient(t *testing.T, user string) *client.TeleportClient {
	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  p.rootCluster.Process,
		Username: user,
	})
	require.NoError(t, err)

	tc, err := p.rootCluster.NewClientWithCreds(helpers.ClientConfig{
		Login:   user,
		Cluster: p.rootCluster.Secrets.SiteName,
		Host:    helpers.Loopback,
		Port:    helpers.Port(t, p.rootCluster.SSH),
	}, *creds)
	require.NoError(t, err)
	return tc
}

// GenerateAndSetupUserCreds is useful in situations where we need to manually manipulate user
// certs, for example when we want to force a TeleportClient to operate using expired certs.
//
// ttl equals to 0 means that the certs will have the default TTL used by helpers.GenerateUserCreds.
func (p *Pack) GenerateAndSetupUserCreds(t *testing.T, tc *client.TeleportClient, ttl time.Duration) {
	creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
		Process:  p.rootCluster.Process,
		Username: tc.Username,
		TTL:      ttl,
	})
	require.NoError(t, err)

	err = helpers.SetupUserCreds(tc, p.rootCluster.Process.Config.Proxy.SSHAddr.Addr, *creds)
	require.NoError(t, err)
}

// CreateAppSessionCookies creates an application session with the root cluster through the web
// API and returns the app session cookies. The application that the user connects to may be
// running in a leaf cluster.
func (p *Pack) CreateAppSessionCookies(t *testing.T, publicAddr, clusterName string) []*http.Cookie {
	require.NotEmpty(t, p.webCookie)
	require.NotEmpty(t, p.webToken)

	casReq, err := json.Marshal(web.CreateAppSessionRequest{
		ResolveAppParams: web.ResolveAppParams{
			FQDNHint:    publicAddr,
			PublicAddr:  publicAddr,
			ClusterName: clusterName,
		},
	})
	require.NoError(t, err)
	statusCode, body, err := p.makeWebapiRequest(http.MethodPost, "sessions/app", casReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, statusCode)

	var casResp *web.CreateAppSessionResponse
	err = json.Unmarshal(body, &casResp)
	require.NoError(t, err)

	return []*http.Cookie{
		{
			Name:  app.CookieName,
			Value: casResp.CookieValue,
		},
		{
			Name:  app.SubjectCookieName,
			Value: casResp.SubjectCookieValue,
		},
	}
}

// CreateAppSessionWithClientCert creates an application session with the root
// cluster and returns the client cert that can be used for an application
// request.
func (p *Pack) CreateAppSessionWithClientCert(t *testing.T) []tls.Certificate {
	session := p.CreateAppSession(t, CreateAppSessionParams{
		Username:      p.username,
		ClusterName:   p.rootAppClusterName,
		AppPublicAddr: p.rootAppPublicAddr,
	})
	config := p.makeTLSConfig(t, tlsConfigParams{
		sessionID:   session.GetName(),
		username:    session.GetUser(),
		publicAddr:  p.rootAppPublicAddr,
		clusterName: p.rootAppClusterName,
	})
	return config.Certificates
}

type CreateAppSessionParams struct {
	Username      string
	ClusterName   string
	AppPublicAddr string
	AppTargetPort int
}

func (p *Pack) CreateAppSession(t *testing.T, params CreateAppSessionParams) types.WebSession {
	ctx := context.Background()
	userState, err := p.rootCluster.Process.GetAuthServer().GetUserOrLoginState(ctx, params.Username)
	require.NoError(t, err)
	accessInfo := services.AccessInfoFromUserState(userState)

	ws, err := p.rootCluster.Process.GetAuthServer().CreateAppSessionFromReq(ctx, auth.NewAppSessionRequest{
		NewWebSessionRequest: auth.NewWebSessionRequest{
			User:       params.Username,
			Roles:      accessInfo.Roles,
			Traits:     accessInfo.Traits,
			SessionTTL: time.Hour,
		},
		PublicAddr:    params.AppPublicAddr,
		ClusterName:   params.ClusterName,
		AppTargetPort: params.AppTargetPort,
	})
	require.NoError(t, err)

	return ws
}

// LockUser will lock the configured user for this pack.
func (p *Pack) LockUser(t *testing.T) {
	err := p.rootCluster.Process.GetAuthServer().UpsertLock(context.Background(), &types.LockV2{
		Spec: types.LockSpecV2{
			Target: types.LockTarget{
				User: p.username,
			},
		},
		Metadata: types.Metadata{
			Name: "test-lock",
		},
	})
	require.NoError(t, err)
}

// makeWebapiRequest makes a request to the root cluster Web API.
func (p *Pack) makeWebapiRequest(method, endpoint string, payload []byte) (int, []byte, error) {
	u := url.URL{
		Scheme: "https",
		Host:   p.rootCluster.Web,
		Path:   fmt.Sprintf("/v1/webapi/%s", endpoint),
	}

	req, err := http.NewRequest(method, u.String(), bytes.NewBuffer(payload))
	if err != nil {
		return 0, nil, trace.Wrap(err)
	}

	req.AddCookie(&http.Cookie{
		Name:  websession.CookieName,
		Value: p.webCookie,
	})
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", p.webToken))
	req.Header.Add("Content-Type", "application/json")

	statusCode, body, err := p.sendRequest(req, nil)
	return statusCode, []byte(body), trace.Wrap(err)
}

func (p *Pack) ensureAuditEvent(t *testing.T, eventType string, checkEvent func(event apievents.AuditEvent)) {
	ctx := context.Background()
	require.Eventuallyf(t, func() bool {
		events, _, err := p.rootCluster.Process.GetAuthServer().SearchEvents(ctx, events.SearchEventsRequest{
			From:       time.Now().Add(-time.Hour),
			To:         time.Now().Add(time.Hour),
			EventTypes: []string{eventType},
			Limit:      1,
			Order:      types.EventOrderDescending,
		})
		require.NoError(t, err)
		if len(events) == 0 {
			return false
		}

		checkEvent(events[0])
		return true
	}, 500*time.Millisecond, 50*time.Millisecond, "failed to fetch audit event \"%s\"", eventType)
}

// initCertPool initializes root cluster CA pool.
func (p *Pack) initCertPool(t *testing.T) {
	authClient := p.rootCluster.GetSiteAPI(p.rootCluster.Secrets.SiteName)
	ca, err := authClient.GetCertAuthority(context.Background(), types.CertAuthID{
		Type:       types.HostCA,
		DomainName: p.rootCluster.Secrets.SiteName,
	}, false)
	require.NoError(t, err)

	pool, err := services.CertPool(ca)
	require.NoError(t, err)

	p.rootCertPool = pool
}

// startLocalProxy starts a local ALPN proxy for the specified application.
func (p *Pack) startLocalProxy(t *testing.T, tlsConfig *tls.Config) string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	proxy, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:    p.rootCluster.Web,
		Protocols:          []alpncommon.Protocol{alpncommon.ProtocolTCP},
		InsecureSkipVerify: true,
		Listener:           listener,
		ParentContext:      context.Background(),
		Cert:               tlsConfig.Certificates[0],
	})
	require.NoError(t, err)
	t.Cleanup(func() { proxy.Close() })

	go proxy.Start(context.Background())

	return proxy.GetAddr()
}

type tlsConfigParams struct {
	sessionID   string
	username    string
	publicAddr  string
	clusterName string
	pinnedIP    string
	targetPort  int
}

// makeTLSConfig returns TLS config suitable for making an app access request.
func (p *Pack) makeTLSConfig(t *testing.T, params tlsConfigParams) *tls.Config {
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	privateKeyPEM, err := keys.MarshalPrivateKey(key)
	require.NoError(t, err)
	publicKeyPEM, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err)

	// Make sure the session ID can be seen in the backend before we continue onward.
	require.Eventually(t, func() bool {
		_, err := p.rootCluster.Process.GetAuthServer().GetAppSession(context.Background(), types.GetAppSessionRequest{
			SessionID: params.sessionID,
		})
		return err == nil
	}, 5*time.Second, 100*time.Millisecond)
	certificate, err := p.rootCluster.Process.GetAuthServer().GenerateUserAppTestCert(
		auth.AppTestCertRequest{
			PublicKey:   publicKeyPEM,
			Username:    params.username,
			TTL:         time.Hour,
			PublicAddr:  params.publicAddr,
			TargetPort:  params.targetPort,
			ClusterName: params.clusterName,
			SessionID:   params.sessionID,
			PinnedIP:    params.pinnedIP,
		})
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(certificate, privateKeyPEM)
	require.NoError(t, err)

	return &tls.Config{
		RootCAs:            p.rootCertPool,
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
	}
}

// makeTLSConfigNoSession returns TLS config for application access without
// creating session to simulate nonexistent session scenario.
func (p *Pack) makeTLSConfigNoSession(t *testing.T, publicAddr, clusterName string) *tls.Config {
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	privateKeyPEM, err := keys.MarshalPrivateKey(key)
	require.NoError(t, err)
	publicKeyPEM, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err)

	certificate, err := p.rootCluster.Process.GetAuthServer().GenerateUserAppTestCert(
		auth.AppTestCertRequest{
			PublicKey:   publicKeyPEM,
			Username:    p.user.GetName(),
			TTL:         time.Hour,
			PublicAddr:  publicAddr,
			ClusterName: clusterName,
			// Use arbitrary session ID
			SessionID: uuid.New().String(),
		})
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(certificate, privateKeyPEM)
	require.NoError(t, err)

	return &tls.Config{
		RootCAs:            p.rootCertPool,
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
	}
}

// MakeRequest makes a request to the root cluster with the given session cookie.
func (p *Pack) MakeRequest(cookies []*http.Cookie, method string, endpoint string, headers ...servicecfg.Header) (int, string, error) {
	req, err := http.NewRequest(method, p.assembleRootProxyURL(endpoint), nil)
	if err != nil {
		return 0, "", trace.Wrap(err)
	}

	// attach session cookies if passed in.
	for _, c := range cookies {
		req.AddCookie(c)
	}

	for _, h := range headers {
		req.Header.Add(h.Name, h.Value)
	}

	return p.sendRequest(req, nil)
}

// makeRequestWithClientCert makes a request to the root cluster using the
// client certificate authentication from the provided tls config.
func (p *Pack) makeRequestWithClientCert(tlsConfig *tls.Config, method, endpoint string) (int, string, error) {
	req, err := http.NewRequest(method, p.assembleRootProxyURL(endpoint), nil)
	if err != nil {
		return 0, "", trace.Wrap(err)
	}
	return p.sendRequest(req, tlsConfig)
}

// makeWebsocketRequest makes a websocket request with the given session cookie.
func (p *Pack) makeWebsocketRequest(cookies []*http.Cookie, endpoint string) (string, error) {
	header := http.Header{}
	dialer := websocket.Dialer{}

	for _, c := range cookies {
		header.Add("Cookie", c.String())
	}
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	conn, resp, err := dialer.Dial(fmt.Sprintf("wss://%s%s", p.rootCluster.Web, endpoint), header)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	defer resp.Body.Close()
	stream := &web.WebsocketIO{Conn: conn}
	data, err := io.ReadAll(stream)
	if err != nil && websocket.IsUnexpectedCloseError(err, websocket.CloseAbnormalClosure) {
		return "", err
	}
	return string(data), nil
}

// assembleRootProxyURL returns the URL string of an endpoint at the root
// cluster's proxy web.
func (p *Pack) assembleRootProxyURL(endpoint string) string {
	u := url.URL{
		Scheme: "https",
		Host:   p.rootCluster.Web,
		Path:   endpoint,
	}
	return u.String()
}

// sendReqeust sends the request to the root cluster.
func (p *Pack) sendRequest(req *http.Request, tlsConfig *tls.Config) (int, string, error) {
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", trace.Wrap(err)
	}

	return resp.StatusCode, string(body), nil
}

// waitForLogout keeps making request with the passed in session cookie until
// they return a non-200 status.
func (p *Pack) waitForLogout(appCookies []*http.Cookie) (int, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-ticker.C:
			status, _, err := p.MakeRequest(appCookies, http.MethodGet, "/")
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

func (p *Pack) startRootAppServers(t *testing.T, count int, opts AppTestOptions) []*service.TeleportProcess {
	configs := make([]*servicecfg.Config, count)

	for i := range count {
		raConf := servicecfg.MakeDefaultConfig()
		raConf.Clock = opts.Clock
		raConf.Logger = utils.NewSlogLoggerForTests()
		raConf.DataDir = t.TempDir()
		raConf.SetToken("static-token-value")
		raConf.SetAuthServerAddress(utils.NetAddr{
			AddrNetwork: "tcp",
			Addr:        p.rootCluster.Web,
		})
		raConf.Auth.Enabled = false
		raConf.Proxy.Enabled = false
		raConf.SSH.Enabled = false
		raConf.Apps.Enabled = true
		raConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
		raConf.Apps.MonitorCloseChannel = opts.MonitorCloseChannel
		raConf.Apps.Apps = append([]servicecfg.App{
			{
				Name:       p.rootAppName,
				URI:        p.rootAppURI,
				PublicAddr: p.rootAppPublicAddr,
			},
			{
				Name:       p.rootWSAppName,
				URI:        p.rootWSAppURI,
				PublicAddr: p.rootWSPublicAddr,
			},
			{
				Name:       p.rootWSSAppName,
				URI:        p.rootWSSAppURI,
				PublicAddr: p.rootWSSPublicAddr,
			},
			{
				Name:       p.rootTCPAppName,
				URI:        p.rootTCPAppURI,
				PublicAddr: p.rootTCPPublicAddr,
			},
			{
				Name:       p.rootTCPTwoWayAppName,
				URI:        p.rootTCPTwoWayAppURI,
				PublicAddr: p.rootTCPTwoWayPublicAddr,
			},
			{
				Name:       p.rootTCPMultiPortAppName,
				URI:        p.rootTCPMultiPortAppURI,
				PublicAddr: p.rootTCPMultiPortPublicAddr,
				TCPPorts: []servicecfg.PortRange{
					servicecfg.PortRange{
						Port: p.rootTCPMultiPortAppPortAlpha,
					},
					servicecfg.PortRange{
						Port: p.rootTCPMultiPortAppPortBeta,
					},
				},
			},
			{
				Name:       p.jwtAppName,
				URI:        p.jwtAppURI,
				PublicAddr: p.jwtAppPublicAddr,
			},
			{
				Name:       p.headerAppName,
				URI:        p.headerAppURI,
				PublicAddr: p.headerAppPublicAddr,
			},
			{
				Name:       p.wsHeaderAppName,
				URI:        p.wsHeaderAppURI,
				PublicAddr: p.wsHeaderAppPublicAddr,
			},
			{
				Name:       p.flushAppName,
				URI:        p.flushAppURI,
				PublicAddr: p.flushAppPublicAddr,
			},
			{
				Name:       "dumper-root",
				URI:        p.dumperAppURI,
				PublicAddr: "dumper-root.example.com",
				Rewrite: &servicecfg.Rewrite{
					Headers: []servicecfg.Header{
						{
							Name:  "X-Teleport-Cluster",
							Value: "root",
						},
						{
							Name:  "X-External-Env",
							Value: "{{external.env}}",
						},
						// Make sure can rewrite Host header.
						{
							Name:  "Host",
							Value: "example.com",
						},
						// Make sure can rewrite existing header.
						{
							Name:  "X-Existing",
							Value: "rewritten-existing-header",
						},
						// Make sure can't rewrite Teleport headers.
						{
							Name:  teleport.AppJWTHeader,
							Value: "rewritten-app-jwt-header",
						},
						{
							Name:  common.TeleportAPIErrorHeader,
							Value: "rewritten-x-teleport-api-error",
						},
						{
							Name:  reverseproxy.XForwardedFor,
							Value: "rewritten-x-forwarded-for-header",
						},
						{
							Name:  reverseproxy.XForwardedHost,
							Value: "rewritten-x-forwarded-host-header",
						},
						{
							Name:  reverseproxy.XForwardedProto,
							Value: "rewritten-x-forwarded-proto-header",
						},
						{
							Name:  reverseproxy.XForwardedServer,
							Value: "rewritten-x-forwarded-server-header",
						},
						{
							Name:  common.XForwardedSSL,
							Value: "rewritten-x-forwarded-ssl-header",
						},
						{
							Name:  reverseproxy.XForwardedPort,
							Value: "rewritten-x-forwarded-port-header",
						},
						// Make sure we can insert JWT token in custom header.
						{
							Name:  "X-JWT",
							Value: teleport.TraitInternalJWTVariable,
						},
					},
				},
			},
		}, opts.ExtraRootApps...)

		configs[i] = raConf
	}

	servers, err := p.rootCluster.StartApps(configs)
	require.NoError(t, err)
	require.Len(t, configs, len(servers))

	for i, appServer := range servers {
		srv := appServer
		t.Cleanup(func() {
			require.NoError(t, srv.Close())
		})
		waitForAppServer(t, p.rootCluster.Tunnel, p.rootAppClusterName, srv.Config.HostUUID, configs[i].Apps.Apps)
	}

	return servers
}

func waitForAppServer(t *testing.T, tunnel reversetunnelclient.Server, name string, hostUUID string, apps []servicecfg.App) {
	// Make sure that the app server is ready to accept connections.
	// The remote site cache needs to be filled with new registered application services.
	waitForAppRegInRemoteSiteCache(t, tunnel, name, apps, hostUUID)
}

func (p *Pack) startLeafAppServers(t *testing.T, count int, opts AppTestOptions) []*service.TeleportProcess {
	configs := make([]*servicecfg.Config, count)

	for i := range count {
		laConf := servicecfg.MakeDefaultConfig()
		laConf.Clock = opts.Clock
		laConf.Logger = utils.NewSlogLoggerForTests()
		laConf.DataDir = t.TempDir()
		laConf.SetToken("static-token-value")
		laConf.SetAuthServerAddress(utils.NetAddr{
			AddrNetwork: "tcp",
			Addr:        p.leafCluster.Web,
		})
		laConf.Auth.Enabled = false
		laConf.Proxy.Enabled = false
		laConf.SSH.Enabled = false
		laConf.Apps.Enabled = true
		laConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()
		laConf.Apps.MonitorCloseChannel = opts.MonitorCloseChannel
		laConf.Apps.Apps = append([]servicecfg.App{
			{
				Name:       p.leafAppName,
				URI:        p.leafAppURI,
				PublicAddr: p.leafAppPublicAddr,
			},
			{
				Name:       p.leafWSAppName,
				URI:        p.leafWSAppURI,
				PublicAddr: p.leafWSPublicAddr,
			},
			{
				Name:       p.leafWSSAppName,
				URI:        p.leafWSSAppURI,
				PublicAddr: p.leafWSSPublicAddr,
			},
			{
				Name:       p.leafTCPAppName,
				URI:        p.leafTCPAppURI,
				PublicAddr: p.leafTCPPublicAddr,
			},
			{
				Name:       p.leafTCPMultiPortAppName,
				URI:        p.leafTCPMultiPortAppURI,
				PublicAddr: p.leafTCPMultiPortPublicAddr,
				TCPPorts: []servicecfg.PortRange{
					servicecfg.PortRange{
						Port: p.leafTCPMultiPortAppPortAlpha,
					},
					servicecfg.PortRange{
						Port: p.leafTCPMultiPortAppPortBeta,
					},
				},
			},
			{
				Name:       "dumper-leaf",
				URI:        p.dumperAppURI,
				PublicAddr: "dumper-leaf.example.com",
				Rewrite: &servicecfg.Rewrite{
					Headers: []servicecfg.Header{
						{
							Name:  "X-Teleport-Cluster",
							Value: "leaf",
						},
						// In leaf clusters internal.logins variable is
						// populated with the user's root role logins.
						{
							Name:  "X-Teleport-Login",
							Value: "{{internal.logins}}",
						},
						{
							Name:  "X-External-Env",
							Value: "{{external.env}}",
						},
						// Make sure can rewrite Host header.
						{
							Name:  "Host",
							Value: "example.com",
						},
						// Make sure can rewrite existing header.
						{
							Name:  "X-Existing",
							Value: "rewritten-existing-header",
						},
						// Make sure can't rewrite Teleport headers.
						{
							Name:  teleport.AppJWTHeader,
							Value: "rewritten-app-jwt-header",
						},
						{
							Name:  common.TeleportAPIErrorHeader,
							Value: "rewritten-x-teleport-api-error",
						},
						{
							Name:  reverseproxy.XForwardedFor,
							Value: "rewritten-x-forwarded-for-header",
						},
						{
							Name:  reverseproxy.XForwardedHost,
							Value: "rewritten-x-forwarded-host-header",
						},
						{
							Name:  reverseproxy.XForwardedProto,
							Value: "rewritten-x-forwarded-proto-header",
						},
						{
							Name:  reverseproxy.XForwardedServer,
							Value: "rewritten-x-forwarded-server-header",
						},
						{
							Name:  common.XForwardedSSL,
							Value: "rewritten-x-forwarded-ssl-header",
						},
						{
							Name:  reverseproxy.XForwardedPort,
							Value: "rewritten-x-forwarded-port-header",
						},
					},
				},
			},
		}, opts.ExtraLeafApps...)

		configs[i] = laConf
	}

	servers, err := p.leafCluster.StartApps(configs)
	require.NoError(t, err)
	require.Len(t, configs, len(servers))

	for i, appServer := range servers {
		srv := appServer
		t.Cleanup(func() {
			require.NoError(t, srv.Close())
		})
		waitForAppServer(t, p.leafCluster.Tunnel, p.leafAppClusterName, srv.Config.HostUUID, configs[i].Apps.Apps)
	}

	return servers
}

func waitForAppRegInRemoteSiteCache(t *testing.T, tunnel reversetunnelclient.Server, clusterName string, cfgApps []servicecfg.App, hostUUID string) {
	if os.Getenv(libmcp.InMemoryServerEnvVar) == "true" {
		cfgApps = append(cfgApps, servicecfg.App{
			Name: libmcp.InMemoryServerName,
		})
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		site, err := tunnel.GetSite(clusterName)
		assert.NoError(t, err)

		ap, err := site.CachingAccessPoint()
		assert.NoError(t, err)

		apps, err := ap.GetApplicationServers(context.Background(), apidefaults.Namespace)
		assert.NoError(t, err)

		counter := 0
		for _, v := range apps {
			if v.GetHostID() == hostUUID {
				counter++
			}
		}
		assert.Len(t, cfgApps, counter)
	}, time.Minute*2, time.Millisecond*200)
}
