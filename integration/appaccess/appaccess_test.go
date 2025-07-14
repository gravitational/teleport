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
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv/app/common"
	libmcp "github.com/gravitational/teleport/lib/srv/mcp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/app"
)

// TestAppAccess runs the full application access integration test suite.
//
// It allows to make the entire cluster set up once, instead of per test,
// which speeds things up significantly.
func TestAppAccess(t *testing.T) {
	t.Setenv(libmcp.InMemoryServerEnvVar, "true")

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

	t.Run("MCP", bind(pack, testMCP))

	// This test should go last because it stops/starts app servers.
	t.Run("TestAppServersHA", bind(pack, testServersHA))
}

// testForward tests that requests get forwarded to the target application
// within a single cluster and trusted cluster.
func testForward(p *Pack, t *testing.T) {
	rootCookies := helpers.ParseCookies(t, p.CreateAppSessionCookies(t, p.rootAppPublicAddr, p.rootAppClusterName))
	leafCookies := helpers.ParseCookies(t, p.CreateAppSessionCookies(t, p.leafAppPublicAddr, p.leafAppClusterName))
	tests := []struct {
		desc          string
		inCookies     []*http.Cookie
		outStatusCode int
		outMessage    string
	}{
		{
			desc:          "root cluster, valid application session cookie, success",
			inCookies:     rootCookies.ToSlice(),
			outStatusCode: http.StatusOK,
			outMessage:    p.rootMessage,
		},
		{
			desc:          "leaf cluster, valid application session cookie, success",
			inCookies:     leafCookies.ToSlice(),
			outStatusCode: http.StatusOK,
			outMessage:    p.leafMessage,
		},
		{
			desc:          "missing root subject session cookie, redirect to login",
			inCookies:     rootCookies.WithSubjectCookie(nil).ToSlice(),
			outStatusCode: http.StatusFound,
			outMessage:    "",
		},
		{
			desc: "root subject session cookie invalid, redirect to login",
			inCookies: rootCookies.WithSubjectCookie(&http.Cookie{
				Name:  app.SubjectCookieName,
				Value: "letmeinplease",
			}).ToSlice(),
			outStatusCode: http.StatusFound,
		},
		{
			desc:          "missing leaf subject session cookie, redirect to login",
			inCookies:     leafCookies.WithSubjectCookie(nil).ToSlice(),
			outStatusCode: http.StatusFound,
		},
		{
			desc: "leaf subject session cookie invalid, redirect to login",
			inCookies: leafCookies.WithSubjectCookie(&http.Cookie{
				Name:  app.SubjectCookieName,
				Value: "letmeinplease",
			}).ToSlice(),
			outStatusCode: http.StatusFound,
		},
		{
			desc: "invalid application session cookie, redirect to login",
			inCookies: []*http.Cookie{
				{
					Name:  app.CookieName,
					Value: "D25C463CD27861559CC6A0A6AE54818079809AA8731CB18037B4B37A80C4FC6C",
				},
			},
			outStatusCode: http.StatusFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			status, body, err := p.MakeRequest(tt.inCookies, http.MethodGet, "/")
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
		inCookies  []*http.Cookie
		outMessage string
		err        error
	}{
		{
			desc:       "root cluster, valid application session cookie, successful websocket (ws://) request",
			inCookies:  p.CreateAppSessionCookies(t, p.rootWSPublicAddr, p.rootAppClusterName),
			outMessage: p.rootWSMessage,
		},
		{
			desc:       "root cluster, valid application session cookie, successful secure websocket (wss://) request",
			inCookies:  p.CreateAppSessionCookies(t, p.rootWSSPublicAddr, p.rootAppClusterName),
			outMessage: p.rootWSSMessage,
		},
		{
			desc:       "leaf cluster, valid application session cookie, successful websocket (ws://) request",
			inCookies:  p.CreateAppSessionCookies(t, p.leafWSPublicAddr, p.leafAppClusterName),
			outMessage: p.leafWSMessage,
		},
		{
			desc:       "leaf cluster, valid application session cookie, successful secure websocket (wss://) request",
			inCookies:  p.CreateAppSessionCookies(t, p.leafWSSPublicAddr, p.leafAppClusterName),
			outMessage: p.leafWSSMessage,
		},
		{
			desc: "valid application session cookie, invalid subject session cookie, websocket request fails to dial",
			inCookies: helpers.ParseCookies(t, p.CreateAppSessionCookies(t, p.rootWSPublicAddr, p.rootAppClusterName)).WithSubjectCookie(
				&http.Cookie{
					Name:  app.SubjectCookieName,
					Value: "foobarbaz",
				},
			).ToSlice(),
			err: errors.New(""),
		},
		{
			desc: "invalid application session cookie, websocket request fails to dial",
			inCookies: []*http.Cookie{
				{
					Name:  app.CookieName,
					Value: "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
				},
			},
			err: errors.New(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			body, err := p.makeWebsocketRequest(tt.inCookies, "/")
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
	_, err = p.rootCluster.Process.GetAuthServer().UpsertSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)
	_, err = p.leafCluster.Process.GetAuthServer().UpsertSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// Requests to root and leaf cluster are successful.
	tests := []struct {
		desc          string
		inCookies     []*http.Cookie
		outStatusCode int
		outMessage    string
	}{
		{
			desc:          "root cluster, valid application session cookie, success",
			inCookies:     p.CreateAppSessionCookies(t, p.rootAppPublicAddr, p.rootAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    p.rootMessage,
		},
		{
			desc:          "leaf cluster, valid application session cookie, success",
			inCookies:     p.CreateAppSessionCookies(t, p.leafAppPublicAddr, p.leafAppClusterName),
			outStatusCode: http.StatusOK,
			outMessage:    p.leafMessage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			status, body, err := p.MakeRequest(tt.inCookies, http.MethodGet, "/")
			require.NoError(t, err)
			require.Equal(t, tt.outStatusCode, status)
			require.Contains(t, body, tt.outMessage)
		})
	}
}

// testClientCert tests mutual TLS authentication flow with application
// access typically used in CLI by curl and other clients.
func testClientCert(p *Pack, t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.App: {Enabled: true},
			},
		},
	})
	evilUser, _ := p.CreateUser(t)
	rootWs := p.CreateAppSession(t, CreateAppSessionParams{
		Username:      p.username,
		ClusterName:   p.rootAppClusterName,
		AppPublicAddr: p.rootAppPublicAddr,
	})
	leafWs := p.CreateAppSession(t, CreateAppSessionParams{
		Username:      p.username,
		ClusterName:   p.leafAppClusterName,
		AppPublicAddr: p.leafAppPublicAddr,
	})

	tests := []struct {
		desc          string
		inTLSConfig   *tls.Config
		outStatusCode int
		outMessage    string
		wantErr       bool
	}{
		{
			desc: "root cluster, valid TLS config, success",
			inTLSConfig: p.makeTLSConfig(t, tlsConfigParams{
				sessionID:   rootWs.GetName(),
				username:    rootWs.GetUser(),
				publicAddr:  p.rootAppPublicAddr,
				clusterName: p.rootAppClusterName,
			}),
			outStatusCode: http.StatusOK,
			outMessage:    p.rootMessage,
		},
		{
			desc: "leaf cluster, valid TLS config, success",
			inTLSConfig: p.makeTLSConfig(t, tlsConfigParams{
				sessionID:   leafWs.GetName(),
				username:    leafWs.GetUser(),
				publicAddr:  p.leafAppPublicAddr,
				clusterName: p.leafAppClusterName,
			}),
			outStatusCode: http.StatusOK,
			outMessage:    p.leafMessage,
		},
		{
			desc:          "root cluster, invalid session ID",
			inTLSConfig:   p.makeTLSConfigNoSession(t, p.rootAppPublicAddr, p.rootAppClusterName),
			outStatusCode: http.StatusForbidden,
		},
		{
			desc: "root cluster, invalid session owner",
			inTLSConfig: p.makeTLSConfig(t, tlsConfigParams{
				sessionID:   rootWs.GetName(),
				username:    evilUser.GetName(),
				publicAddr:  p.rootAppPublicAddr,
				clusterName: p.rootAppClusterName,
			}),
			outStatusCode: http.StatusForbidden,
			outMessage:    "",
		},
		{
			desc: "leaf cluster, invalid session owner",
			inTLSConfig: p.makeTLSConfig(t, tlsConfigParams{
				sessionID:   leafWs.GetName(),
				username:    evilUser.GetName(),
				publicAddr:  p.leafAppPublicAddr,
				clusterName: p.leafAppClusterName,
			}),
			outStatusCode: http.StatusForbidden,
			outMessage:    "",
		},
		{
			desc: "root cluster, valid TLS config with pinned IP, success",
			inTLSConfig: p.makeTLSConfig(t, tlsConfigParams{
				sessionID:   rootWs.GetName(),
				username:    rootWs.GetUser(),
				publicAddr:  p.rootAppPublicAddr,
				clusterName: p.rootAppClusterName,
				pinnedIP:    "127.0.0.1",
			}),
			outStatusCode: http.StatusOK,
			outMessage:    p.rootMessage,
		},
		{
			desc: "root cluster, valid TLS config with wrong pinned IP",
			inTLSConfig: p.makeTLSConfig(t, tlsConfigParams{
				sessionID:   rootWs.GetName(),
				username:    rootWs.GetUser(),
				publicAddr:  p.rootAppPublicAddr,
				clusterName: p.rootAppClusterName,
				pinnedIP:    "127.0.0.2",
			}),
			outStatusCode: http.StatusForbidden,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			status, body, err := p.makeRequestWithClientCert(tt.inTLSConfig, http.MethodGet, "/")
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.outStatusCode, status)
				require.Contains(t, body, tt.outMessage)
			}
		})
	}
}

// appAccessFlush makes sure that application access periodically flushes
// buffered data to the response.
func testFlush(p *Pack, t *testing.T) {
	req, err := http.NewRequest("GET", p.assembleRootProxyURL("/"), nil)
	require.NoError(t, err)

	cookies := p.CreateAppSessionCookies(t, p.flushAppPublicAddr, p.flushAppClusterName)
	for _, c := range cookies {
		req.AddCookie(c)
	}

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
	appCookies := p.CreateAppSessionCookies(t, "dumper-root.example.com", "example.com")

	// Get headers response and make sure headers were passed.
	status, resp, err := p.MakeRequest(appCookies, http.MethodGet, "/", servicecfg.Header{
		Name: "X-Existing", Value: "existing",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Dumper app just dumps HTTP request so we should be able to read it back.
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(resp)))
	require.NoError(t, err)
	require.Equal(t, "example.com", req.Host)
	require.Equal(t, "root", req.Header.Get("X-Teleport-Cluster"))
	require.Equal(t, "production", req.Header.Get("X-External-Env"))
	require.Equal(t, "rewritten-existing-header", req.Header.Get("X-Existing"))

	// verify these headers were not rewritten.
	require.NotEqual(t, "rewritten-app-jwt-header", req.Header.Get(teleport.AppJWTHeader))
	require.NotEqual(t, "rewritten-x-teleport-api-error", req.Header.Get(common.TeleportAPIErrorHeader))
	require.NotEqual(t, "rewritten-x-forwarded-for-header", req.Header.Get(reverseproxy.XForwardedFor))
	require.NotEqual(t, "rewritten-x-forwarded-host-header", req.Header.Get(reverseproxy.XForwardedHost))
	require.NotEqual(t, "rewritten-x-forwarded-proto-header", req.Header.Get(reverseproxy.XForwardedProto))
	require.NotEqual(t, "rewritten-x-forwarded-server-header", req.Header.Get(reverseproxy.XForwardedServer))
	require.NotEqual(t, "rewritten-x-forwarded-ssl", req.Header.Get(common.XForwardedSSL))

	// Verify JWT tokens.
	for _, header := range []string{teleport.AppJWTHeader, "X-JWT"} {
		verifyJWT(t, p, req.Header.Get(header), p.dumperAppURI)
	}
}

// testRewriteHeadersLeaf validates that http headers from application
// rewrite configuration are correctly passed to proxied applications in leaf.
func testRewriteHeadersLeaf(p *Pack, t *testing.T) {
	// Create an application session for dumper app in leaf cluster.
	appCookie := p.CreateAppSessionCookies(t, "dumper-leaf.example.com", "leaf.example.com")

	// Get headers response and make sure headers were passed.
	status, resp, err := p.MakeRequest(appCookie, http.MethodGet, "/", servicecfg.Header{
		Name: "X-Existing", Value: "existing",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Dumper app just dumps HTTP request so we should be able to read it back.
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(resp)))
	require.NoError(t, err)
	require.Equal(t, "example.com", req.Host)
	require.Equal(t, "leaf", req.Header.Get("X-Teleport-Cluster"))
	require.ElementsMatch(t, []string{"root", "ubuntu", "-teleport-internal-join"}, req.Header.Values("X-Teleport-Login"))
	require.Equal(t, "production", req.Header.Get("X-External-Env"))
	require.Equal(t, "rewritten-existing-header", req.Header.Get("X-Existing"))

	// verify these headers were not rewritten.
	require.NotEqual(t, "rewritten-app-jwt-header", req.Header.Get(teleport.AppJWTHeader))
	require.NotEqual(t, "rewritten-x-teleport-api-error", req.Header.Get(common.TeleportAPIErrorHeader))
	require.NotEqual(t, "rewritten-x-forwarded-ssl", req.Header.Get(common.XForwardedSSL))
	require.NotEqual(t, "rewritten-x-forwarded-for-header", req.Header.Get(reverseproxy.XForwardedFor))
	require.NotEqual(t, "rewritten-x-forwarded-host-header", req.Header.Get(reverseproxy.XForwardedHost))
	require.NotEqual(t, "rewritten-x-forwarded-proto-header", req.Header.Get(reverseproxy.XForwardedProto))
	require.NotEqual(t, "rewritten-x-forwarded-server-header", req.Header.Get(reverseproxy.XForwardedServer))
}

// testLogout verifies the session is removed from the backend when the user logs out.
func testLogout(p *Pack, t *testing.T) {
	// Create an application session.
	appCookies := p.CreateAppSessionCookies(t, p.rootAppPublicAddr, p.rootAppClusterName)

	// Log user out of session.
	status, _, err := p.MakeRequest(appCookies, http.MethodGet, "/teleport-logout")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Wait until requests using the session cookie have failed.
	status, err = p.waitForLogout(appCookies)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, status)
}

// testJWT ensures a JWT token is attached to requests and the JWT token can
// be validated.
func testJWT(p *Pack, t *testing.T) {
	// Create an application session.
	appCookies := p.CreateAppSessionCookies(t, p.jwtAppPublicAddr, p.jwtAppClusterName)

	// Get JWT.
	status, token, err := p.MakeRequest(appCookies, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Verify JWT token.
	verifyJWT(t, p, token, p.jwtAppURI)

	// Connect to websocket application that dumps the upgrade request.
	wsCookies := p.CreateAppSessionCookies(t, p.wsHeaderAppPublicAddr, p.wsHeaderAppClusterName)
	body, err := p.makeWebsocketRequest(wsCookies, "/")
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
	appCookies := p.CreateAppSessionCookies(t, p.headerAppPublicAddr, p.headerAppClusterName)

	// Get HTTP headers forwarded to the application.
	status, origHeaderResp, err := p.MakeRequest(appCookies, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	origHeaders := strings.Split(origHeaderResp, "\n")
	require.Len(t, origHeaders, len(forwardedHeaderNames)+1)

	// Construct HTTP request with custom headers.
	req, err := http.NewRequest(http.MethodGet, p.assembleRootProxyURL("/"), nil)
	require.NoError(t, err)
	for _, c := range appCookies {
		req.AddCookie(c)
	}
	for _, headerName := range forwardedHeaderNames {
		req.Header.Set(headerName, uuid.New().String())
	}

	// Issue the request.
	status, newHeaderResp, err := p.sendRequest(req, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	newHeaders := strings.Split(newHeaderResp, "\n")
	require.Len(t, newHeaders, len(forwardedHeaderNames)+1)

	// Headers sent to the application should not be affected.
	for i := range forwardedHeaderNames {
		require.Equal(t, origHeaders[i], newHeaders[i])
	}
}

func testAuditEvents(p *Pack, t *testing.T) {
	inCookies := p.CreateAppSessionCookies(t, p.rootAppPublicAddr, p.rootAppClusterName)

	status, body, err := p.MakeRequest(inCookies, http.MethodGet, "/")
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
			cmpopts.IgnoreFields(apievents.Metadata{}, "ID", "Time", "Index"),
			cmpopts.IgnoreFields(apievents.AppSessionChunk{}, "SessionChunkID"),
		))
	})
}

func TestInvalidateAppSessionsOnLogout(t *testing.T) {
	p := Setup(t)

	// Create an application session.
	appCookies := p.CreateAppSessionCookies(t, p.rootAppPublicAddr, p.rootAppClusterName)
	sessID := helpers.ParseCookies(t, appCookies).SessionCookie.Value

	// Issue a request to the application to guarantee everything is working correctly.
	status, _, err := p.MakeRequest(appCookies, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Generates TLS config for making app requests.
	reqTLS := p.makeTLSConfig(t, tlsConfigParams{
		sessionID:   sessID,
		username:    p.username,
		publicAddr:  p.rootAppPublicAddr,
		clusterName: p.rootAppClusterName,
	})
	require.NotNil(t, reqTLS)

	// Issue a request to the application to guarantee everything is working correctly.
	status, _, err = p.makeRequestWithClientCert(reqTLS, http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// Logout from Teleport.
	status, _, err = p.makeWebapiRequest(http.MethodDelete, "sessions/web", []byte{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	// As deleting WebSessions might not happen immediately, run the next request
	// in an `Eventually` block.
	require.Eventually(t, func() bool {
		// Issue another request to the application. Now, it should receive a
		// redirect because the application sessions are gone.
		status, _, err = p.MakeRequest(appCookies, http.MethodGet, "/")
		require.NoError(t, err)
		return status == http.StatusFound
	}, time.Second, 250*time.Millisecond)

	// Check the same for the client certificate.
	require.Eventually(t, func() bool {
		// Issue another request to the application. Now, it should receive a
		// redirect because the application sessions are gone.
		status, _, err = p.makeRequestWithClientCert(reqTLS, http.MethodGet, "/")
		require.NoError(t, err)
		return status == http.StatusForbidden
	}, time.Second, 250*time.Millisecond)
}

// TestTCP tests proxying of plain TCP applications through app access.
func TestTCP(t *testing.T) {
	pack := Setup(t)
	evilUser, _ := pack.CreateUser(t)
	sessionUsername := pack.tc.Username

	rootTCPAppAddr, err := utils.ParseAddr(pack.rootTCPAppURI)
	require.NoError(t, err)
	rootTCPAppPort := rootTCPAppAddr.Port(0)

	tests := []struct {
		description string
		// tlsConfigParams carries information needed to create TLS config for a local proxy.
		// tlsConfigParams.sessionID is automatically set from the session created within the test.
		tlsConfigParams tlsConfigParams
		outMessage      string
		wantReadErr     error
	}{
		{
			description: "TCP app in root cluster",
			tlsConfigParams: tlsConfigParams{
				username:    sessionUsername,
				publicAddr:  pack.rootTCPPublicAddr,
				clusterName: pack.rootAppClusterName,
			},
			outMessage: pack.rootTCPMessage,
		},
		{
			description: "TCP app in leaf cluster",
			tlsConfigParams: tlsConfigParams{
				username:    sessionUsername,
				publicAddr:  pack.leafTCPPublicAddr,
				clusterName: pack.leafAppClusterName,
			},
			outMessage: pack.leafTCPMessage,
		},
		{
			description: "TCP app in root cluster, invalid session owner",
			tlsConfigParams: tlsConfigParams{
				username:    evilUser.GetName(),
				publicAddr:  pack.rootTCPPublicAddr,
				clusterName: pack.rootAppClusterName,
			},
			wantReadErr: io.EOF, // access denied errors should close the tcp conn
		},
		{
			description: "TCP app in leaf cluster, invalid session owner",
			tlsConfigParams: tlsConfigParams{
				username:    evilUser.GetName(),
				publicAddr:  pack.leafTCPPublicAddr,
				clusterName: pack.leafAppClusterName,
			},
			wantReadErr: io.EOF, // access denied errors should close the tcp conn
		},
		// The following two situation can happen when a multi-port app is updated to be a singe-port
		// app but after the user already generated a cert for the multi-port variant.
		{
			description: "TCP app, target port matches port in URI",
			tlsConfigParams: tlsConfigParams{
				username:    sessionUsername,
				publicAddr:  pack.rootTCPPublicAddr,
				clusterName: pack.rootAppClusterName,
				targetPort:  rootTCPAppPort,
			},
			outMessage: pack.rootTCPMessage,
		},
		{
			description: "TCP app, target port does not match port in URI",
			tlsConfigParams: tlsConfigParams{
				username:    sessionUsername,
				publicAddr:  pack.rootTCPPublicAddr,
				clusterName: pack.rootAppClusterName,
				targetPort:  rootTCPAppPort - 1,
			},
			wantReadErr: io.EOF,
		},
		{
			description: "multi-port TCP app in root cluster",
			tlsConfigParams: tlsConfigParams{
				username:    sessionUsername,
				publicAddr:  pack.rootTCPMultiPortPublicAddr,
				clusterName: pack.rootAppClusterName,
				targetPort:  pack.rootTCPMultiPortAppPortAlpha,
			},
			outMessage: pack.rootTCPMultiPortMessageAlpha,
		},
		{
			description: "multi-port TCP app in root cluster, other port",
			tlsConfigParams: tlsConfigParams{
				username:    sessionUsername,
				publicAddr:  pack.rootTCPMultiPortPublicAddr,
				clusterName: pack.rootAppClusterName,
				targetPort:  pack.rootTCPMultiPortAppPortBeta,
			},
			outMessage: pack.rootTCPMultiPortMessageBeta,
		},
		{
			description: "multi-port TCP app in leaf cluster",
			tlsConfigParams: tlsConfigParams{
				username:    sessionUsername,
				publicAddr:  pack.leafTCPMultiPortPublicAddr,
				clusterName: pack.leafAppClusterName,
				targetPort:  pack.leafTCPMultiPortAppPortAlpha,
			},
			outMessage: pack.leafTCPMultiPortMessageAlpha,
		},
		{
			description: "multi-port TCP app in leaf cluster, other port",
			tlsConfigParams: tlsConfigParams{
				username:    sessionUsername,
				publicAddr:  pack.leafTCPMultiPortPublicAddr,
				clusterName: pack.leafAppClusterName,
				targetPort:  pack.leafTCPMultiPortAppPortBeta,
			},
			outMessage: pack.leafTCPMultiPortMessageBeta,
		},
		{
			// This simulates an older client with no TargetPort connecting to a newer app agent.
			description: "multi-port TCP app in root cluster, no target port",
			tlsConfigParams: tlsConfigParams{
				username:    sessionUsername,
				publicAddr:  pack.rootTCPMultiPortPublicAddr,
				clusterName: pack.rootAppClusterName,
			},
			// Such client should still be proxied to the first port found in TCP ports of the app.
			outMessage: pack.rootTCPMultiPortMessageAlpha,
		},
		{
			description: "multi-port TCP app, port not in spec",
			tlsConfigParams: tlsConfigParams{
				username:    sessionUsername,
				publicAddr:  pack.rootTCPMultiPortPublicAddr,
				clusterName: pack.rootAppClusterName,
				// 42 should not be handed out to a non-root user when creating a listener on port 0, so
				// it's unlikely that 42 is going to end up in the app spec.
				targetPort: 42,
			},
			wantReadErr: io.EOF,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			ws := pack.CreateAppSession(t, CreateAppSessionParams{
				Username:      sessionUsername,
				ClusterName:   test.tlsConfigParams.clusterName,
				AppPublicAddr: test.tlsConfigParams.publicAddr,
				AppTargetPort: test.tlsConfigParams.targetPort,
			})

			test.tlsConfigParams.sessionID = ws.GetName()

			localProxyAddress := pack.startLocalProxy(t, pack.makeTLSConfig(t, test.tlsConfigParams))

			conn, err := net.Dial("tcp", localProxyAddress)
			require.NoError(t, err)
			defer conn.Close()

			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if test.wantReadErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, test.wantReadErr)
				return
			}
			require.NoError(t, err)

			resp := strings.TrimSpace(string(buf[:n]))
			require.Equal(t, test.outMessage, resp)
		})
	}
}

// TestTCPLock tests locking TCP applications.
func TestTCPLock(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Now())
	mCloseChannel := make(chan struct{}, 1)
	pack := SetupWithOptions(t, AppTestOptions{
		Clock:               clock,
		MonitorCloseChannel: mCloseChannel,
	})

	msg := []byte(uuid.New().String())

	// Start the proxy to the two way communication app.
	rootWs := pack.CreateAppSession(t, CreateAppSessionParams{
		Username:      pack.tc.Username,
		ClusterName:   pack.rootAppClusterName,
		AppPublicAddr: pack.rootTCPTwoWayPublicAddr,
	})
	tlsConfig := pack.makeTLSConfig(t, tlsConfigParams{
		sessionID:   rootWs.GetName(),
		username:    rootWs.GetUser(),
		publicAddr:  pack.rootTCPTwoWayPublicAddr,
		clusterName: pack.rootAppClusterName,
	})

	address := pack.startLocalProxy(t, tlsConfig)

	var conn net.Conn
	var err error
	var n int
	buf := make([]byte, 1024)

	// Try to read for a short amount of time.
	require.Eventually(t, func() bool {
		conn, err = net.Dial("tcp", address)
		if err != nil {
			return false
		}

		// Try to read
		n, err = conn.Read(buf)
		return err == nil
	}, time.Second*5, time.Millisecond*100)

	resp := strings.TrimSpace(string(buf[:n]))
	require.Equal(t, pack.rootTCPTwoWayMessage, resp)

	// Once we've read successfully, write, and then read again to verify the connection
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
	mCloseChannel := make(chan struct{}, 1)
	pack := SetupWithOptions(t, AppTestOptions{
		Clock:               clock,
		MonitorCloseChannel: mCloseChannel,
	})

	msg := []byte(uuid.New().String())

	// Start the proxy to the two way communication app.
	rootWs := pack.CreateAppSession(t, CreateAppSessionParams{
		Username:      pack.tc.Username,
		ClusterName:   pack.rootAppClusterName,
		AppPublicAddr: pack.rootTCPTwoWayPublicAddr,
	})
	tlsConfig := pack.makeTLSConfig(t, tlsConfigParams{
		sessionID:   rootWs.GetName(),
		username:    rootWs.GetUser(),
		publicAddr:  pack.rootTCPTwoWayPublicAddr,
		clusterName: pack.rootAppClusterName,
	})

	address := pack.startLocalProxy(t, tlsConfig)

	var conn net.Conn
	var err error
	var n int
	buf := make([]byte, 1024)

	// Try to read for a short amount of time.
	require.Eventually(t, func() bool {
		conn, err = net.Dial("tcp", address)
		if err != nil {
			return false
		}

		// Read, write, and read again to make sure its working as expected.
		n, err = conn.Read(buf)
		return err == nil
	}, time.Second*5, time.Millisecond*100)

	resp := strings.TrimSpace(string(buf[:n]))
	require.Equal(t, pack.rootTCPTwoWayMessage, resp)

	// Once we've read successfully, write, and then read again to verify the connection
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

	require.Eventually(t, func() bool {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return false
		}

		// Try to read again, expect a failure.
		_, err = conn.Read(buf)
		return err != nil
	}, time.Second*5, time.Millisecond*100)
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
			require.Equal(t, http.StatusFound, status)
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

	makeRequests := func(t *testing.T, pack *Pack, httpCookies, wsCookies []*http.Cookie, responseAssertion func(*testing.T, int, error)) {
		status, _, err := pack.MakeRequest(httpCookies, http.MethodGet, "/")
		responseAssertion(t, status, err)

		_, err = pack.makeWebsocketRequest(wsCookies, "/")
		responseAssertion(t, 0, err)
	}

	for name, test := range testCases {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			info := test.packInfo(p)
			httpCookies := p.CreateAppSessionCookies(t, info.publicHTTPAddr, info.clusterName)
			wsCookies := p.CreateAppSessionCookies(t, info.publicWSAddr, info.clusterName)

			makeRequests(t, p, httpCookies, wsCookies, responseWithoutError)

			// Stop all root app servers.
			for i, appServer := range info.appServers {
				require.NoError(t, appServer.Close())
				require.NoError(t, appServer.Wait())

				if i == len(info.appServers)-1 {
					// fails only when the last one is closed.
					makeRequests(t, p, httpCookies, wsCookies, responseWithError)
				} else {
					// otherwise the request should be handled by another
					// server.
					makeRequests(t, p, httpCookies, wsCookies, responseWithoutError)
				}
			}

			servers := test.startAppServers(p, 1)
			test.waitForTunnelConn(t, p, 1)
			makeRequests(t, p, httpCookies, wsCookies, responseWithoutError)

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
				makeRequests(t, p, httpCookies, wsCookies, responseWithoutError)
			}
		})
	}
}
