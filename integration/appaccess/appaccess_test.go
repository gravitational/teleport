// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package appaccess

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/web/app"
)

// TestAppAccess runs the full application access integration test suite.
//
// It allows to make the entire cluster set up once, instead of per test,
// which speeds things up significantly.
func TestAppAccess(t *testing.T) {
	pack := Setup(t)

	t.Run("Forward", bind(pack, testForward))
	t.Run("Websockets", bind(pack, testWebsockets))
	t.Run("ClientCert", bind(pack, testClientCert))
	t.Run("Flush", bind(pack, testFlush))
	t.Run("ForwardModes", bind(pack, testForwardModes))
	t.Run("RewriteHeadersRoot", bind(pack, testRewriteHeadersRoot))
	t.Run("RewriteHeadersLeaf", bind(pack, testRewriteHeadersLeaf))
	t.Run("Logout", bind(pack, testLogout))
	t.Run("JWT", bind(pack, testJWT))
	t.Run("NoHeaderOverrides", bind(pack, testNoHeaderOverrides))
	t.Run("AuditEvents", bind(pack, testAuditEvents))
	t.Run("TestAppInvalidateAppSessionsOnLogout", bind(pack, testInvalidateAppSessionsOnLogout))

	// This test should go last because it stops/starts app servers.
	t.Run("TestAppServersHA", bind(pack, testServersHA))
}

// testForward tests that requests get forwarded to the target application
// within a single cluster and trusted cluster.
func testForward(p *Pack, t *testing.T) {
	tests := []struct {
		desc          string
		inCookie      string
		outStatusCode int
		outMessage    string
	}{
		{
			desc:          "root cluster, valid application session cookie, success",
			inCookie:      p.CreateAppSession(t, p.rootAppPublicAddr, p.rootAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    p.rootMessage,
		},
		{
			desc:          "leaf cluster, valid application session cookie, success",
			inCookie:      p.CreateAppSession(t, p.leafAppPublicAddr, p.leafAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    p.leafMessage,
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
			status, body, err := p.MakeRequest(tt.inCookie, http.MethodGet, "/")
			require.NoError(t, err)
			require.Equal(t, tt.outStatusCode, status)
			require.Contains(t, body, tt.outMessage)
		})
	}
}

// TestWebsockets makes sure that websocket requests get forwarded.
func testWebsockets(p *Pack, t *testing.T) {
	tests := []struct {
		desc       string
		inCookie   string
		outMessage string
		err        error
	}{
		{
			desc:       "root cluster, valid application session cookie, successful websocket (ws://) request",
			inCookie:   p.CreateAppSession(t, p.rootWSPublicAddr, p.rootAppClusterName),
			outMessage: p.rootWSMessage,
		},
		{
			desc:       "root cluster, valid application session cookie, successful secure websocket (wss://) request",
			inCookie:   p.CreateAppSession(t, p.rootWSSPublicAddr, p.rootAppClusterName),
			outMessage: p.rootWSSMessage,
		},
		{
			desc:       "leaf cluster, valid application session cookie, successful websocket (ws://) request",
			inCookie:   p.CreateAppSession(t, p.leafWSPublicAddr, p.leafAppClusterName),
			outMessage: p.leafWSMessage,
		},
		{
			desc:       "leaf cluster, valid application session cookie, successful secure websocket (wss://) request",
			inCookie:   p.CreateAppSession(t, p.leafWSSPublicAddr, p.leafAppClusterName),
			outMessage: p.leafWSSMessage,
		},
		{
			desc:     "invalid application session cookie, websocket request fails to dial",
			inCookie: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			err:      errors.New(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tt := tt
			body, err := p.makeWebsocketRequest(tt.inCookie, "/")
			if tt.err != nil {
				require.IsType(t, tt.err, trace.Unwrap(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.outMessage, body)
			}
		})
	}
}

// testForwardModes ensures that requests are forwarded to applications
// even when the cluster is in proxy recording mode.
func testForwardModes(p *Pack, t *testing.T) {
	// Create cluster, user, sessions, and credentials package.
	ctx := context.Background()

	// Update root and leaf clusters to record sessions at the proxy.
	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	require.NoError(t, err)
	err = p.rootCluster.Process.GetAuthServer().SetSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)
	err = p.leafCluster.Process.GetAuthServer().SetSessionRecordingConfig(ctx, recConfig)
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
			inCookie:      p.CreateAppSession(t, p.rootAppPublicAddr, p.rootAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    p.rootMessage,
		},
		{
			desc:          "leaf cluster, valid application session cookie, success",
			inCookie:      p.CreateAppSession(t, p.leafAppPublicAddr, p.leafAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    p.leafMessage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tt := tt
			status, body, err := p.MakeRequest(tt.inCookie, http.MethodGet, "/")
			require.NoError(t, err)
			require.Equal(t, tt.outStatusCode, status)
			require.Contains(t, body, tt.outMessage)
		})
	}
}

// testClientCert tests mutual TLS authentication flow with application
// access typically used in CLI by curl and other clients.
func testClientCert(p *Pack, t *testing.T) {
	tests := []struct {
		desc          string
		inTLSConfig   *tls.Config
		outStatusCode int
		outMessage    string
	}{
		{
			desc:          "root cluster, valid TLS config, success",
			inTLSConfig:   p.makeTLSConfig(t, p.rootAppPublicAddr, p.rootAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    p.rootMessage,
		},
		{
			desc:          "leaf cluster, valid TLS config, success",
			inTLSConfig:   p.makeTLSConfig(t, p.leafAppPublicAddr, p.leafAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    p.leafMessage,
		},
		{
			desc:          "root cluster, invalid session ID",
			inTLSConfig:   p.makeTLSConfigNoSession(t, p.rootAppPublicAddr, p.rootAppClusterName),
			outStatusCode: http.StatusFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tt := tt
			status, body, err := p.makeRequestWithClientCert(tt.inTLSConfig, http.MethodGet, "/")
			require.NoError(t, err)
			require.Equal(t, tt.outStatusCode, status)
			require.Contains(t, body, tt.outMessage)
		})
	}
}

// appAccessFlush makes sure that application access periodically flushes
// buffered data to the response.
func testFlush(p *Pack, t *testing.T) {
	req, err := http.NewRequest("GET", p.assembleRootProxyURL("/"), nil)
	require.NoError(t, err)

	cookie := p.CreateAppSession(t, p.flushAppPublicAddr, p.flushAppClusterName)
	req.AddCookie(&http.Cookie{
		Name:  app.CookieName,
		Value: cookie,
	})

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

	// The "flush server" will send 2 messages, "hello" and "world", with a
	// 500ms delay between them. They should arrive as 2 different frames
	// due to the periodic flushing.
	frames := []string{"hello", "world"}
	for _, frame := range frames {
		buffer := make([]byte, 1024)
		n, err := resp.Body.Read(buffer)
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
		}
		require.Equal(t, frame, strings.TrimSpace(string(buffer[:n])))
	}
}

// testRewriteHeadersRoot validates that http headers from application
// rewrite configuration are correctly passed to proxied applications in root.
func testRewriteHeadersRoot(p *Pack, t *testing.T) {
	// Create an application session for dumper app in root cluster.
	appCookie := p.CreateAppSession(t, "dumper-root.example.com", "example.com")

	// Get headers response and make sure headers were passed.
	status, resp, err := p.MakeRequest(appCookie, http.MethodGet, "/", service.Header{
		Name: "X-Existing", Value: "existing",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Dumper app just dumps HTTP request so we should be able to read it back.
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(resp)))
	require.NoError(t, err)
	require.Equal(t, req.Host, "example.com")
	require.Equal(t, req.Header.Get("X-Teleport-Cluster"), "root")
	require.Equal(t, req.Header.Get("X-External-Env"), "production")
	require.Equal(t, req.Header.Get("X-Existing"), "rewritten-existing-header")
	require.NotEqual(t, req.Header.Get(teleport.AppJWTHeader), "rewritten-app-jwt-header")
	require.NotEqual(t, req.Header.Get(teleport.AppCFHeader), "rewritten-app-cf-header")
	require.NotEqual(t, req.Header.Get(forward.XForwardedFor), "rewritten-x-forwarded-for-header")
	require.NotEqual(t, req.Header.Get(forward.XForwardedHost), "rewritten-x-forwarded-host-header")
	require.NotEqual(t, req.Header.Get(forward.XForwardedProto), "rewritten-x-forwarded-proto-header")
	require.NotEqual(t, req.Header.Get(forward.XForwardedServer), "rewritten-x-forwarded-server-header")

	// Verify JWT tokens.
	for _, header := range []string{teleport.AppJWTHeader, teleport.AppCFHeader, "X-JWT"} {
		verifyJWT(t, p, req.Header.Get(header), p.dumperAppURI)
	}
}

// testRewriteHeadersLeaf validates that http headers from application
// rewrite configuration are correctly passed to proxied applications in leaf.
func testRewriteHeadersLeaf(p *Pack, t *testing.T) {
	// Create an application session for dumper app in leaf cluster.
	appCookie := p.CreateAppSession(t, "dumper-leaf.example.com", "leaf.example.com")

	// Get headers response and make sure headers were passed.
	status, resp, err := p.MakeRequest(appCookie, http.MethodGet, "/", service.Header{
		Name: "X-Existing", Value: "existing",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, resp, "X-Teleport-Cluster: leaf")
	require.Contains(t, resp, "X-Teleport-Login: root")
	require.Contains(t, resp, "X-Teleport-Login: ubuntu")
	require.Contains(t, resp, "X-External-Env: production")
	require.Contains(t, resp, "Host: example.com")
	require.Contains(t, resp, "X-Existing: rewritten-existing-header")
	require.NotContains(t, resp, "X-Existing: existing")
	require.NotContains(t, resp, "rewritten-app-jwt-header")
	require.NotContains(t, resp, "rewritten-app-cf-header")
	require.NotContains(t, resp, "rewritten-x-forwarded-for-header")
	require.NotContains(t, resp, "rewritten-x-forwarded-host-header")
	require.NotContains(t, resp, "rewritten-x-forwarded-proto-header")
	require.NotContains(t, resp, "rewritten-x-forwarded-server-header")
}

// testLogout verifies the session is removed from the backend when the user logs out.
func testLogout(p *Pack, t *testing.T) {
	// Create an application session.
	appCookie := p.CreateAppSession(t, p.rootAppPublicAddr, p.rootAppClusterName)

	// Log user out of session.
	status, _, err := p.MakeRequest(appCookie, http.MethodGet, "/teleport-logout")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Wait until requests using the session cookie have failed.
	status, err = p.waitForLogout(appCookie)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, status)
}

// testJWT ensures a JWT token is attached to requests and the JWT token can
// be validated.
func testJWT(p *Pack, t *testing.T) {
	// Create an application session.
	appCookie := p.CreateAppSession(t, p.jwtAppPublicAddr, p.jwtAppClusterName)

	// Get JWT.
	status, token, err := p.MakeRequest(appCookie, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Verify JWT token.
	verifyJWT(t, p, token, p.jwtAppURI)

	// Connect to websocket application that dumps the upgrade request.
	wsCookie := p.CreateAppSession(t, p.wsHeaderAppPublicAddr, p.wsHeaderAppClusterName)
	body, err := p.makeWebsocketRequest(wsCookie, "/")
	require.NoError(t, err)

	// Parse the upgrade request the websocket application received.
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(body)))
	require.NoError(t, err)

	// Extract JWT token from header and verify it.
	wsToken := req.Header.Get(teleport.AppJWTHeader)
	require.NotEmpty(t, wsToken, "websocket upgrade request doesn't contain JWT header")
	verifyJWT(t, p, wsToken, p.wsHeaderAppURI)
}

// testNoHeaderOverrides ensures that AAP-specific headers cannot be overridden
// by values passed in by the user.
func testNoHeaderOverrides(p *Pack, t *testing.T) {
	// Create an application session.
	appCookie := p.CreateAppSession(t, p.headerAppPublicAddr, p.headerAppClusterName)

	// Get HTTP headers forwarded to the application.
	status, origHeaderResp, err := p.MakeRequest(appCookie, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	origHeaders := strings.Split(origHeaderResp, "\n")
	require.Equal(t, len(origHeaders), len(forwardedHeaderNames)+1)

	// Construct HTTP request with custom headers.
	req, err := http.NewRequest(http.MethodGet, p.assembleRootProxyURL("/"), nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{
		Name:  app.CookieName,
		Value: appCookie,
	})
	for _, headerName := range forwardedHeaderNames {
		req.Header.Set(headerName, uuid.New().String())
	}

	// Issue the request.
	status, newHeaderResp, err := p.sendRequest(req, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	newHeaders := strings.Split(newHeaderResp, "\n")
	require.Equal(t, len(newHeaders), len(forwardedHeaderNames)+1)

	// Headers sent to the application should not be affected.
	for i := range forwardedHeaderNames {
		require.Equal(t, origHeaders[i], newHeaders[i])
	}
}

func testAuditEvents(p *Pack, t *testing.T) {
	inCookie := p.CreateAppSession(t, p.rootAppPublicAddr, p.rootAppClusterName)

	status, body, err := p.MakeRequest(inCookie, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, p.rootMessage)

	// session start event
	p.ensureAuditEvent(t, events.AppSessionStartEvent, func(event apievents.AuditEvent) {
		expectedEvent := &apievents.AppSessionStart{
			Metadata: apievents.Metadata{
				Type:        events.AppSessionStartEvent,
				Code:        events.AppSessionStartCode,
				ClusterName: p.rootAppClusterName,
			},
			AppMetadata: apievents.AppMetadata{
				AppURI:        p.rootAppURI,
				AppPublicAddr: p.rootAppPublicAddr,
				AppName:       p.rootAppName,
			},
			PublicAddr: p.rootAppPublicAddr,
		}
		require.Empty(t, cmp.Diff(
			expectedEvent,
			event,
			cmpopts.IgnoreTypes(apievents.ServerMetadata{}, apievents.SessionMetadata{}, apievents.UserMetadata{}, apievents.ConnectionMetadata{}),
			cmpopts.IgnoreFields(apievents.Metadata{}, "ID", "Time"),
		))
	})

	// session chunk event
	p.ensureAuditEvent(t, events.AppSessionChunkEvent, func(event apievents.AuditEvent) {
		expectedEvent := &apievents.AppSessionChunk{
			Metadata: apievents.Metadata{
				Type:        events.AppSessionChunkEvent,
				Code:        events.AppSessionChunkCode,
				ClusterName: p.rootAppClusterName,
			},
			AppMetadata: apievents.AppMetadata{
				AppURI:        p.rootAppURI,
				AppPublicAddr: p.rootAppPublicAddr,
				AppName:       p.rootAppName,
			},
		}
		require.Empty(t, cmp.Diff(
			expectedEvent,
			event,
			cmpopts.IgnoreTypes(apievents.ServerMetadata{}, apievents.SessionMetadata{}, apievents.UserMetadata{}, apievents.ConnectionMetadata{}),
			cmpopts.IgnoreFields(apievents.Metadata{}, "ID", "Time"),
			cmpopts.IgnoreFields(apievents.AppSessionChunk{}, "SessionChunkID"),
		))
	})
}

func testInvalidateAppSessionsOnLogout(p *Pack, t *testing.T) {
	t.Cleanup(func() {
		// This test will invalidate the web session so init it again after the
		// test, otherwise tests that run after this one will be getting 403's.
		p.initWebSession(t)
	})

	// Create an application session.
	appCookie := p.CreateAppSession(t, p.rootAppPublicAddr, p.rootAppClusterName)

	// Issue a request to the application to guarantee everything is working correctly.
	status, _, err := p.MakeRequest(appCookie, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Generates TLS config for making app requests.
	reqTLS := p.makeTLSConfig(t, p.rootAppPublicAddr, p.rootAppClusterName)
	require.NotNil(t, reqTLS)

	// Issue a request to the application to guarantee everything is working correctly.
	status, _, err = p.makeRequestWithClientCert(reqTLS, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Logout from Teleport.
	status, _, err = p.makeWebapiRequest(http.MethodDelete, "sessions", []byte{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// As deleting WebSessions might not happen immediately, run the next request
	// in an `Eventually` block.
	require.Eventually(t, func() bool {
		// Issue another request to the application. Now, it should receive a
		// redirect because the application sessions are gone.
		status, _, err = p.MakeRequest(appCookie, http.MethodGet, "/")
		require.NoError(t, err)
		return status == http.StatusFound
	}, time.Second, 250*time.Millisecond)

	// Check the same for the client certificate.
	require.Eventually(t, func() bool {
		// Issue another request to the application. Now, it should receive a
		// redirect because the application sessions are gone.
		status, _, err = p.makeRequestWithClientCert(reqTLS, http.MethodGet, "/")
		require.NoError(t, err)
		return status == http.StatusFound
	}, time.Second, 250*time.Millisecond)
}

// TestTCP tests proxying of plain TCP applications through app access.
func TestTCP(t *testing.T) {
	pack := Setup(t)

	tests := []struct {
		description string
		address     string
		outMessage  string
	}{
		{
			description: "TCP app in root cluster",
			address:     pack.startLocalProxy(t, pack.rootTCPPublicAddr, pack.rootAppClusterName),
			outMessage:  pack.rootTCPMessage,
		},
		{
			description: "TCP app in leaf cluster",
			address:     pack.startLocalProxy(t, pack.leafTCPPublicAddr, pack.leafAppClusterName),
			outMessage:  pack.leafTCPMessage,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			conn, err := net.Dial("tcp", test.address)
			require.NoError(t, err)

			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			require.NoError(t, err)

			resp := strings.TrimSpace(string(buf[:n]))
			require.Equal(t, test.outMessage, resp)
		})
	}
}

// TestTCPLock tests locking TCP applications.
func TestTCPLock(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Now())
	mCloseChannel := make(chan struct{})
	pack := SetupWithOptions(t, AppTestOptions{
		Clock:               clock,
		MonitorCloseChannel: mCloseChannel,
	})

	msg := []byte(uuid.New().String())

	// Start the proxy to the two way communication app.
	address := pack.startLocalProxy(t, pack.rootTCPTwoWayPublicAddr, pack.rootAppClusterName)
	conn, err := net.Dial("tcp", address)
	require.NoError(t, err)

	// Read, write, and read again to make sure its working as expected.
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	require.NoError(t, err, buf)

	resp := strings.TrimSpace(string(buf[:n]))
	require.Equal(t, pack.rootTCPTwoWayMessage, resp)

	_, err = conn.Write(msg)
	require.NoError(t, err)

	n, err = conn.Read(buf)
	require.NoError(t, err, buf)

	resp = strings.TrimSpace(string(buf[:n]))
	require.Equal(t, pack.rootTCPTwoWayMessage, resp)

	// Lock the user and try to write
	pack.LockUser(t)
	clock.Advance(10 * time.Second)

	// Wait for the channel closure signal
	select {
	case <-mCloseChannel:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout waiting for monitor channel signal")
	}
	_, err = conn.Write(msg)
	require.NoError(t, err)

	_, err = conn.Read(buf)
	require.Error(t, err)

	// Close and re-open the connection
	require.NoError(t, conn.Close())

	conn, err = net.Dial("tcp", address)
	require.NoError(t, err)

	// Try to read again, expect a failure.
	_, err = conn.Read(buf)
	require.Error(t, err, buf)
}

// TestTCPCertExpiration tests TCP application with certs expiring.
func TestTCPCertExpiration(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Now())
	mCloseChannel := make(chan struct{})
	pack := SetupWithOptions(t, AppTestOptions{
		Clock:               clock,
		MonitorCloseChannel: mCloseChannel,
	})

	msg := []byte(uuid.New().String())

	// Start the proxy to the two way communication app.
	address := pack.startLocalProxy(t, pack.rootTCPTwoWayPublicAddr, pack.rootAppClusterName)
	conn, err := net.Dial("tcp", address)
	require.NoError(t, err)

	// Read, write, and read again to make sure its working as expected.
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	require.NoError(t, err, buf)

	resp := strings.TrimSpace(string(buf[:n]))
	require.Equal(t, pack.rootTCPTwoWayMessage, resp)

	_, err = conn.Write(msg)
	require.NoError(t, err)

	n, err = conn.Read(buf)
	require.NoError(t, err, buf)

	resp = strings.TrimSpace(string(buf[:n]))
	require.Equal(t, pack.rootTCPTwoWayMessage, resp)

	// Let the cert expire. We'll choose 24 hours to make sure we go above
	// any cert durations that could be chosen here.
	clock.Advance(24 * time.Hour)
	// Wait for the channel closure signal
	select {
	case <-mCloseChannel:
	case <-time.After(time.Second * 10):
		require.Fail(t, "timeout waiting for monitor channel signal")
	}
	_, err = conn.Write(msg)
	require.NoError(t, err)

	_, err = conn.Read(buf)
	require.Error(t, err)

	// Close and re-open the connection
	require.NoError(t, conn.Close())

	conn, err = net.Dial("tcp", address)
	require.NoError(t, err)

	// Try to read again, expect a failure.
	_, err = conn.Read(buf)
	require.Error(t, err, buf)
}

func testServersHA(p *Pack, t *testing.T) {
	type packInfo struct {
		clusterName    string
		publicHTTPAddr string
		publicWSAddr   string
		appServers     []*service.TeleportProcess
	}

	testCases := map[string]struct {
		packInfo          func(pack *Pack) packInfo
		startAppServers   func(pack *Pack, count int) []*service.TeleportProcess
		waitForTunnelConn func(t *testing.T, pack *Pack, count int)
	}{
		"RootServer": {
			packInfo: func(pack *Pack) packInfo {
				return packInfo{
					clusterName:    pack.rootAppClusterName,
					publicHTTPAddr: pack.rootAppPublicAddr,
					publicWSAddr:   pack.rootWSPublicAddr,
					appServers:     pack.rootAppServers,
				}
			},
			startAppServers: func(pack *Pack, count int) []*service.TeleportProcess {
				return pack.startRootAppServers(t, count, AppTestOptions{})
			},
			waitForTunnelConn: func(t *testing.T, pack *Pack, count int) {
				helpers.WaitForActiveTunnelConnections(t, pack.rootCluster.Tunnel, pack.rootCluster.Secrets.SiteName, count)
			},
		},
		"LeafServer": {
			packInfo: func(pack *Pack) packInfo {
				return packInfo{
					clusterName:    pack.leafAppClusterName,
					publicHTTPAddr: pack.leafAppPublicAddr,
					publicWSAddr:   pack.leafWSPublicAddr,
					appServers:     pack.leafAppServers,
				}
			},
			startAppServers: func(pack *Pack, count int) []*service.TeleportProcess {
				return pack.startLeafAppServers(t, count, AppTestOptions{})
			},
			waitForTunnelConn: func(t *testing.T, pack *Pack, count int) {
				helpers.WaitForActiveTunnelConnections(t, pack.leafCluster.Tunnel, pack.leafCluster.Secrets.SiteName, count)
			},
		},
	}

	// asserts that the response has error.
	responseWithError := func(t *testing.T, status int, err error) {
		if status > 0 {
			require.NoError(t, err)
			require.Equal(t, http.StatusInternalServerError, status)
			return
		}

		require.Error(t, err)
	}
	// asserts that the response has no errors.
	responseWithoutError := func(t *testing.T, status int, err error) {
		if status > 0 {
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status)
			return
		}

		require.NoError(t, err)
	}

	makeRequests := func(t *testing.T, pack *Pack, httpCookie, wsCookie string, responseAssertion func(*testing.T, int, error)) {
		status, _, err := pack.MakeRequest(httpCookie, http.MethodGet, "/")
		responseAssertion(t, status, err)

		_, err = pack.makeWebsocketRequest(wsCookie, "/")
		responseAssertion(t, 0, err)
	}

	for name, test := range testCases {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			info := test.packInfo(p)
			httpCookie := p.CreateAppSession(t, info.publicHTTPAddr, info.clusterName)
			wsCookie := p.CreateAppSession(t, info.publicWSAddr, info.clusterName)

			makeRequests(t, p, httpCookie, wsCookie, responseWithoutError)

			// Stop all root app servers.
			for i, appServer := range info.appServers {
				require.NoError(t, appServer.Close())
				require.NoError(t, appServer.Wait())

				if i == len(info.appServers)-1 {
					// fails only when the last one is closed.
					makeRequests(t, p, httpCookie, wsCookie, responseWithError)
				} else {
					// otherwise the request should be handled by another
					// server.
					makeRequests(t, p, httpCookie, wsCookie, responseWithoutError)
				}
			}

			servers := test.startAppServers(p, 1)
			test.waitForTunnelConn(t, p, 1)
			makeRequests(t, p, httpCookie, wsCookie, responseWithoutError)

			// Start an additional app server and stop all current running
			// ones.
			test.startAppServers(p, 1)
			test.waitForTunnelConn(t, p, 2)

			for _, appServer := range servers {
				require.NoError(t, appServer.Close())
				require.NoError(t, appServer.Wait())

				// Everytime an app server stops we issue a request to
				// guarantee that the requests are going to be resolved by
				// the remaining app servers.
				makeRequests(t, p, httpCookie, wsCookie, responseWithoutError)
			}
		})
	}
}
