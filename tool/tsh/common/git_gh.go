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
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	proto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

const (
	gitServerEnvVar = "TSH_GIT_SERVER"
	ghCLIBinaryName = "gh"
)

// gitGHCommand implements `tsh gh`.
type gitGHCommand struct {
	*kingpin.CmdClause

	gitServerName string
	execCmd       string
	ghArgs        []string
}

func newGitGHCommand(parent *kingpin.CmdClause) *gitGHCommand {
	cmd := &gitGHCommand{
		CmdClause: parent.Command("gh", "Run gh CLI commands through Teleport."),
	}
	cmd.Flag("git-server", "Name of the git server. Can also be set via TSH_GIT_SERVER env var.").
		Envar(gitServerEnvVar).
		StringVar(&cmd.gitServerName)
	cmd.Flag("exec", "Run a custom command instead of gh.").StringVar(&cmd.execCmd)
	cmd.Arg("gh-args", "Arguments to pass to gh CLI.").StringsVar(&cmd.ghArgs)
	return cmd
}

func newTopLevelGHCommand(app *kingpin.Application) *gitGHCommand {
	cmd := &gitGHCommand{
		CmdClause: app.Command("gh", "Run gh CLI commands through Teleport."),
	}
	cmd.Flag("git-server", "Name of the git server. Can also be set via TSH_GIT_SERVER env var.").
		Envar(gitServerEnvVar).
		StringVar(&cmd.gitServerName)
	cmd.Flag("exec", "Run a custom command instead of gh.").StringVar(&cmd.execCmd)
	cmd.Arg("gh-args", "Arguments to pass to gh CLI.").StringsVar(&cmd.ghArgs)
	return cmd
}

func (c *gitGHCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	gitServer, err := c.resolveGitServer(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	github := gitServer.GetGitHub()
	if github == nil {
		return trace.BadParameter("git server %v is not a GitHub server", gitServer.GetName())
	}
	if !types.GitServerHTTPEnabled(github) {
		return trace.BadParameter("git server %v does not have HTTP proxying enabled", gitServer.GetName())
	}

	valid, _ := hasValidGitCert(tc, gitServer.GetName())
	if !valid {
		if err := ensureGitCredentialsAndCert(cf, tc, gitServer); err != nil {
			return trace.Wrap(err)
		}
	}

	proxy, err := startGitProxy(cf, tc, gitServer.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxy.Close()

	commandToRun := ghCLIBinaryName
	if c.execCmd != "" {
		commandToRun = c.execCmd
	}
	cmd := exec.Command(commandToRun, c.ghArgs...)
	cmd.Stdout = cf.Stdout()
	cmd.Stderr = cf.Stderr()
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	for k, v := range proxy.GetEnvVars() {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	logger.DebugContext(cf.Context, "Running gh command",
		"http_proxy", proxy.httpProxyAddr,
		"args", c.ghArgs,
	)

	if err := cf.RunCommand(cmd); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *gitGHCommand) resolveGitServer(cf *CLIConf, tc *client.TeleportClient) (types.Server, error) {
	if c.gitServerName != "" {
		return findGitServerByName(cf, tc, c.gitServerName)
	}

	var servers []types.Server
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		servers, _, err = clusterClient.AuthClient.GitServerReadOnlyClient().ListGitServers(cf.Context, 0, "")
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var httpServers []types.Server
	for _, s := range servers {
		if github := s.GetGitHub(); github != nil && types.GitServerHTTPEnabled(github) {
			httpServers = append(httpServers, s)
		}
	}

	switch len(httpServers) {
	case 0:
		return nil, trace.NotFound("no git servers with HTTP proxying enabled found")
	case 1:
		return httpServers[0], nil
	default:
		var names []string
		for _, s := range httpServers {
			names = append(names, s.GetName())
		}
		return nil, trace.BadParameter("multiple git servers found, specify one with --git-server: %v", names)
	}
}

// gitProxyCommand implements `tsh proxy git`.
type gitProxyCommand struct {
	*kingpin.CmdClause

	gitServerName string
}

func newGitProxyCommand(parent *kingpin.CmdClause) *gitProxyCommand {
	cmd := &gitProxyCommand{
		CmdClause: parent.Command("git", "Start a local proxy for Git HTTPS and GitHub API access."),
	}
	cmd.Arg("git-server", "Name of the git server.").Required().StringVar(&cmd.gitServerName)
	return cmd
}

func (c *gitProxyCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	proxy, err := startGitProxy(cf, tc, c.gitServerName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxy.Close()

	fmt.Fprintf(cf.Stdout(), "Started git proxy for %q\n\n", c.gitServerName)
	fmt.Fprintf(cf.Stdout(), "Use with gh CLI:\n")
	fmt.Fprintf(cf.Stdout(), "  export GH_HOST=github.localhost\n")
	fmt.Fprintf(cf.Stdout(), "  export GH_TOKEN=teleport\n")
	fmt.Fprintf(cf.Stdout(), "  export HTTP_PROXY=http://%s\n", proxy.httpProxyAddr)
	fmt.Fprintf(cf.Stdout(), "  gh api /user\n\n")
	fmt.Fprintf(cf.Stdout(), "Use with curl (HTTPS via forward proxy):\n")
	fmt.Fprintf(cf.Stdout(), "  HTTPS_PROXY=http://%s curl --cacert <ca-path> https://api.github.com/user\n\n", proxy.forwardProxy.GetAddr())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
	case <-cf.Context.Done():
	}
	return nil
}

// gitProxy holds the running git proxy infrastructure.
type gitProxy struct {
	certChecker    *client.CertChecker
	localCAPath    string
	alpnProxy      *alpnproxy.LocalProxy
	forwardProxy   *alpnproxy.ForwardProxy
	httpProxy      *http.Server
	httpProxyLn    net.Listener
	httpProxyAddr  string
}

func (p *gitProxy) Close() error {
	var errs []error
	if p.httpProxy != nil {
		errs = append(errs, p.httpProxy.Close())
	}
	if p.httpProxyLn != nil {
		errs = append(errs, p.httpProxyLn.Close())
	}
	if p.forwardProxy != nil {
		errs = append(errs, p.forwardProxy.Close())
	}
	if p.alpnProxy != nil {
		errs = append(errs, p.alpnProxy.Close())
	}
	return trace.NewAggregate(errs...)
}

// GetEnvVars returns environment variables for gh CLI.
// Uses GH_HOST=github.localhost so gh sends plain HTTP (no TLS verification
// needed). The HTTP_PROXY routes requests through our local proxy which
// rewrites github.localhost to github.com and forwards to the Teleport proxy
// with mTLS.
//
// TODO(greedy52) once Go supports SSL_CERT_FILE on macOS
// (https://github.com/golang/go/issues/77865), switch to using HTTPS_PROXY
// with a self-signed CA instead of the github.localhost workaround.
func (p *gitProxy) GetEnvVars() map[string]string {
	return map[string]string{
		"GH_TOKEN":   "teleport",
		"GH_HOST":    "github.localhost",
		"HTTP_PROXY": fmt.Sprintf("http://%s", p.httpProxyAddr),
		"http_proxy": fmt.Sprintf("http://%s", p.httpProxyAddr),
	}
}

func startGitProxy(cf *CLIConf, tc *client.TeleportClient, gitServerName string) (*gitProxy, error) {
	routeToGit := proto.RouteToGit{
		GitServerName: gitServerName,
	}

	gitCertChecker := client.NewGitCertChecker(tc, routeToGit, nil, client.WithTTL(tc.KeyTTL))

	if cert, err := loadAppCertificate(tc, gitServerName); err == nil {
		gitCertChecker.SetCert(cert)
	}

	// Create a local CA for TLS termination (used by git clone HTTPS via the
	// forward proxy). Git (LibreSSL) respects GIT_SSL_CAINFO.
	profile, err := cf.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appLocalCAPath := profile.AppLocalCAPath(tc.SiteName, gitServerName)
	localCertGen, err := client.NewLocalCertGenerator(cf.Context, gitCertChecker, appLocalCAPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Start ALPN proxy with TLS listener (terminates git's HTTPS with
	// self-signed CA, then connects to Teleport proxy with mTLS git cert).
	tlsListener, err := tls.Listen("tcp", "localhost:0", &tls.Config{
		GetCertificate: localCertGen.GetCertificate,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg := makeBasicLocalProxyConfig(cf.Context, tc, tlsListener, false)
	cfg.Protocols = []alpncommon.Protocol{alpncommon.ProtocolHTTP}

	alpnProxy, err := alpnproxy.NewLocalProxy(
		cfg,
		alpnproxy.WithClusterCAsIfConnUpgrade(cf.Context, tc.RootClusterCACertPool),
		alpnproxy.WithMiddleware(gitCertChecker),
	)
	if err != nil {
		tlsListener.Close()
		return nil, trace.Wrap(err)
	}

	go func() {
		if err := alpnProxy.Start(cf.Context); err != nil {
			logger.ErrorContext(cf.Context, "Failed to start git ALPN proxy", "error", err)
		}
	}()

	proxy := &gitProxy{
		certChecker: gitCertChecker,
		localCAPath: appLocalCAPath,
		alpnProxy:   alpnProxy,
	}

	// Start forward proxy (for curl / HTTPS_PROXY usage with SSL_CERT_FILE on Linux).
	forwardProxy, err := startGitForwardProxy(cf, alpnProxy.GetAddr())
	if err != nil {
		alpnProxy.Close()
		return nil, trace.Wrap(err)
	}
	proxy.forwardProxy = forwardProxy

	// Start HTTP proxy for gh CLI with GH_HOST=github.localhost.
	// gh sends plain HTTP through HTTP_PROXY. We rewrite github.localhost to
	// github.com and forward to the Teleport proxy with mTLS.
	httpProxyLn, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		proxy.Close()
		return nil, trace.Wrap(err)
	}
	proxy.httpProxyLn = httpProxyLn
	proxy.httpProxyAddr = httpProxyLn.Addr().String()

	httpProxyServer := &http.Server{
		Handler: newGitHTTPProxyHandler(gitCertChecker, tc.WebProxyAddr),
	}
	proxy.httpProxy = httpProxyServer

	go func() {
		if err := httpProxyServer.Serve(httpProxyLn); err != nil && err != http.ErrServerClosed {
			logger.ErrorContext(cf.Context, "Failed to start git HTTP proxy", "error", err)
		}
	}()

	logger.DebugContext(cf.Context, "Git proxy started",
		"alpn_proxy", alpnProxy.GetAddr(),
		"forward_proxy", forwardProxy.GetAddr(),
		"http_proxy", proxy.httpProxyAddr,
	)

	return proxy, nil
}

func matchGitHubRequests(req *http.Request) bool {
	host := req.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	switch host {
	case "github.com", "api.github.com":
		return true
	default:
		return false
	}
}

func startGitForwardProxy(cf *CLIConf, alpnProxyAddr string) (*alpnproxy.ForwardProxy, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxy, err := alpnproxy.NewForwardProxy(alpnproxy.ForwardProxyConfig{
		Listener:     listener,
		CloseContext: cf.Context,
		Handlers: []alpnproxy.ConnectRequestHandler{
			alpnproxy.NewForwardToHostHandler(alpnproxy.ForwardToHostHandlerConfig{
				MatchFunc: matchGitHubRequests,
				Host:      alpnProxyAddr,
			}),
			alpnproxy.NewForwardToSystemProxyHandler(alpnproxy.ForwardToSystemProxyHandlerConfig{}),
			alpnproxy.NewForwardToOriginalHostHandler(),
		},
	})
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}

	go func() {
		if err := proxy.Start(); err != nil {
			logger.ErrorContext(cf.Context, "Failed to start git forward proxy", "error", err)
		}
	}()
	return proxy, nil
}

// gitHTTPProxyHandler receives plain HTTP proxy requests from gh when using
// GH_HOST=github.localhost. It rewrites github.localhost URLs to github.com
// and forwards to the Teleport proxy with mTLS.
type gitHTTPProxyHandler struct {
	certChecker *client.CertChecker
	proxyAddr   string
}

func newGitHTTPProxyHandler(certChecker *client.CertChecker, proxyAddr string) *gitHTTPProxyHandler {
	return &gitHTTPProxyHandler{
		certChecker: certChecker,
		proxyAddr:   proxyAddr,
	}
}

func (h *gitHTTPProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.DebugContext(r.Context(), "Git HTTP proxy received request",
		"method", r.Method,
		"host", r.Host,
		"url", r.URL.String(),
	)

	cert, err := h.certChecker.GetOrIssueCert(r.Context())
	if err != nil {
		logger.ErrorContext(r.Context(), "Failed to get git cert", "error", err)
		http.Error(w, "failed to get certificate", http.StatusInternalServerError)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "https"
			req.URL.Host = strings.Replace(req.URL.Host, "github.localhost", "github.com", 1)
			req.Host = strings.Replace(req.Host, "github.localhost", "github.com", 1)
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
				NextProtos:         []string{string(alpncommon.ProtocolHTTP)},
			},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("tcp", h.proxyAddr)
			},
		},
		ModifyResponse: gitHTTPProxyModifyResponse,
	}
	proxy.ServeHTTP(w, r)
}

// gitHTTPProxyModifyResponse intercepts responses from the Teleport proxy and
// adds a hint when GitHub credentials have expired.
func gitHTTPProxyModifyResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		resp.Header.Set("X-Teleport-Git-Error", "GitHub authorization required. Run \"tsh git login\" and retry.")
		logger.WarnContext(resp.Request.Context(), "GitHub authorization required, run \"tsh git login\" and retry")
	}
	return nil
}

// ensureGitCredentialsAndCert checks for stored credentials and triggers OAuth
// if needed, then issues a git cert. Called only when no valid git cert exists
// (first-time use).
func ensureGitCredentialsAndCert(cf *CLIConf, tc *client.TeleportClient, gitServer types.Server) error {
	github := gitServer.GetGitHub()
	if github == nil {
		return trace.BadParameter("git server %v is not a GitHub server", gitServer.GetName())
	}

	hasCredentials, err := checkGitHubCredentials(cf, tc, gitServer.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	if !hasCredentials {
		fmt.Fprintln(cf.Stderr(), "GitHub authorization required. Starting OAuth flow...")
		if _, err := getGitHubIdentity(cf, github.Organization, withForceOAuthFlow(true)); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := issueGitCert(cf, tc, gitServer.GetName()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
