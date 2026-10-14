/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

type gitHTTPProxy struct {
	*alpnproxy.LocalProxy
	middleware *gitHTTPMiddleware
}

// startGitHTTPProxy starts a local git HTTP proxy that sends the requests with
// the encrypted token to the Teleport proxy with the git cert.
func startGitHTTPProxy(gitCtx *gitContext) (*gitHTTPProxy, error) {
	// Fail early before the client starts. The token goes first, as the GitHub
	// OAuth flow reissues certs.
	if _, err := gitCtx.getUserDecryptedAccessToken(); err != nil {
		return nil, trace.Wrap(err)
	}
	certChecker := client.NewGitCertChecker(gitCtx.tc, proto.RouteToGit{
		Name: gitCtx.gitServer.GetName(),
	}, nil)
	// If a stored cert is found for the git server, try using it.
	if cert, err := loadGitCertificate(gitCtx.tc, gitCtx.gitServer.GetName()); err == nil {
		certChecker.SetCert(cert)
	}
	if _, err := certChecker.GetOrIssueCert(gitCtx.Context); err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Forward CONNECT requests (e.g. from a tsh git run by gh, connecting to
	// Teleport) to their destination, through the system proxy if configured.
	forwardProxy, err := alpnproxy.NewForwardProxy(alpnproxy.ForwardProxyConfig{
		Listener:     listener, // not used but required
		CloseContext: gitCtx.Context,
		Handlers: []alpnproxy.ConnectRequestHandler{
			alpnproxy.NewForwardToSystemProxyHandler(alpnproxy.ForwardToSystemProxyHandlerConfig{
				InsecureSystemProxy: gitCtx.InsecureSkipVerify,
			}),
			alpnproxy.NewForwardToOriginalHostHandler(),
		},
	})
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}
	middleware := &gitHTTPMiddleware{
		getUserDecryptedAccessToken: gitCtx.getUserDecryptedAccessToken,
		forwardProxy:                forwardProxy,
	}
	lp, err := alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(gitCtx.Context, gitCtx.tc, listener, gitCtx.InsecureSkipVerify),
		alpnproxy.WithALPNProtocol(alpncommon.ProtocolHTTP),
		alpnproxy.WithClusterCAsIfConnUpgrade(gitCtx.Context, gitCtx.tc.RootClusterCACertPool),
		alpnproxy.WithHTTPMiddleware(middleware),
		alpnproxy.WithMiddleware(certChecker),
	)
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}
	go func() {
		if err := lp.Start(gitCtx.Context); err != nil {
			logger.WarnContext(gitCtx.Context, "Git HTTP proxy stopped", "error", err)
		}
	}()
	logger.DebugContext(gitCtx.Context, "Started git HTTP proxy", "addr", lp.GetAddr())
	return &gitHTTPProxy{
		LocalProxy: lp,
		middleware: middleware,
	}, nil
}

// gitHTTPMiddleware is the local proxy HTTP middleware for Git over HTTPS.
type gitHTTPMiddleware struct {
	alpnproxy.DefaultLocalProxyHTTPMiddleware
	getUserDecryptedAccessToken func() (string, error)

	// unauthorized is set when GitHub rejects the token.
	unauthorized atomic.Bool

	// mu makes sure only one getUserDecryptedAccessToken runs at a time, so
	// concurrent requests do not each refresh the token or run the OAuth flow.
	mu sync.Mutex

	// forwardProxy tunnels CONNECT requests.
	forwardProxy http.Handler
}

// HandleRequest implements [alpnproxy.LocalProxyHTTPMiddleware].
func (m *gitHTTPMiddleware) HandleRequest(w http.ResponseWriter, r *http.Request) bool {
	logger.DebugContext(r.Context(), "Git HTTP proxy received request", "method", r.Method, "host", r.URL.Host, "path", r.URL.Path)
	if r.Method == http.MethodConnect {
		m.forwardProxy.ServeHTTP(w, r)
		return true
	}
	upstreamHost, ok := gitHTTPProxyHosts[r.URL.Hostname()]
	if !ok {
		http.Error(w, fmt.Sprintf("host %q is not supported, use 'tsh git clone' to clone a repository", r.URL.Hostname()), http.StatusForbidden)
		return true
	}
	m.mu.Lock()
	encryptedToken, err := m.getUserDecryptedAccessToken()
	m.mu.Unlock()
	if err != nil {
		logger.WarnContext(r.Context(), "Failed to get GitHub token for git HTTP proxy", "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return true
	}
	// The local proxy forwards the request to the Teleport proxy with the
	// original Host.
	r.Host = upstreamHost
	r.Header.Set(teleport.GitTokenHeader, encryptedToken)
	return false
}

// HandleResponse implements [alpnproxy.LocalProxyHTTPMiddleware].
func (m *gitHTTPMiddleware) HandleResponse(resp *http.Response) error {
	// Teleport access denied errors are 403, so 401 comes from GitHub:
	//
	// "If you try to use a REST API endpoint without a token or with a token
	// that has insufficient permissions, you will receive a 404 Not Found or
	// 403 Forbidden response. Authenticating with invalid credentials will
	// initially return a 401 Unauthorized response."
	//
	// https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api
	if resp.StatusCode == http.StatusUnauthorized {
		m.unauthorized.Store(true)
	}
	// gh follows the absolute pagination URLs in Link, so point them to this
	// proxy.
	for i, link := range resp.Header["Link"] {
		resp.Header["Link"][i] = strings.ReplaceAll(link, "<https://api.github.com/", "<http://api.github.localhost/")
	}
	// Make same-host redirects relative, so the client follows them through
	// this proxy.
	if loc, err := resp.Location(); err == nil && loc.Host == resp.Request.Host {
		resp.Header.Set("Location", loc.RequestURI())
	}
	return nil
}

// gitHTTPProxyHosts maps the hosts accepted by the git HTTP proxy to the
// GitHub hosts they are forwarded to.
var gitHTTPProxyHosts = map[string]string{
	"github.com":           "github.com",
	"api.github.localhost": "api.github.com",
	// Git commands run by `tsh gh`.
	"github.localhost": "github.com",
}

// printGitHubCredentialsRejected prints how to authorize again after GitHub
// rejects the credentials of the session.
func printGitHubCredentialsRejected(gitCtx *gitContext) {
	org := gitCtx.gitServer.GetGitHub().Organization
	if !sso.OpenBrowserPathAvailable(gitCtx.Browser, gitCtx.LookPath) {
		fmt.Fprintf(gitCtx.Stderr(), "\nGitHub rejected the credentials for organization %q. "+
			"Open this link in a browser logged in to Teleport as %s to authorize again, then run this command again:\n\n  %s\n\n",
			org, gitCtx.tc.Username, gitCtx.gitServerAuthorizeOnWebURL())
		return
	}
	fmt.Fprintf(gitCtx.Stderr(), "\nGitHub rejected the credentials for organization %q, run 'tsh git login --github-org %s --force' and try again.\n", org, org)
}

func loadGitCertificate(tc *client.TeleportClient, gitServerName string) (tls.Certificate, error) {
	keyRing, err := tc.LocalAgent().GetKeyRing(tc.SiteName, client.WithGitCerts{})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	cert, err := keyRing.GitTLSCert(gitServerName)
	return cert, trace.Wrap(err)
}
