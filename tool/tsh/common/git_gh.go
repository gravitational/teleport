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

	if !isBeamsEnvironment() {
		valid, _ := hasValidGitCert(tc, gitServer.GetName())
		if !valid {
			if err := ensureGitCredentialsAndCert(cf, tc, gitServer); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	proxy, err := startGitProxy(cf, tc, gitProxyConfig{
		gitServerName: gitServer.GetName(),
	})
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
	for k, v := range proxy.GetGHEnvVars() {
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

	gitServerName      string
	gitHubOrganization string
	port               string
}

func newGitProxyCommand(parent *kingpin.CmdClause) *gitProxyCommand {
	cmd := &gitProxyCommand{
		CmdClause: parent.Command("git", "Start a local proxy for GitHub API access."),
	}
	cmd.Arg("git-server", "Name of the git server.").StringVar(&cmd.gitServerName)
	cmd.Flag("github-org", "GitHub organization.").StringVar(&cmd.gitHubOrganization)
	cmd.Flag("port", "Specifies the source port used by the proxy.").Short('p').StringVar(&cmd.port)
	return cmd
}

func (c *gitProxyCommand) run(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	gitServer, err := resolveGitServer(cf, tc, c.gitServerName, c.gitHubOrganization)
	if err != nil {
		return trace.Wrap(err)
	}

	proxy, err := startGitProxy(cf, tc, gitProxyConfig{
		gitServerName: gitServer.GetName(),
		port:          c.port,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxy.Close()

	github := gitServer.GetGitHub()
	if github == nil {
		return trace.BadParameter("git server %v is not a GitHub server", gitServer.GetName())
	}

	fmt.Fprintf(cf.Stdout(), "Started git proxy for GitHub organization %q\n\n", github.Organization)
	fmt.Fprintf(cf.Stdout(), "Export the following environment variables to use the proxy:\n\n")
	fmt.Fprintf(cf.Stdout(), "  export HTTP_PROXY=http://%s\n", proxy.httpProxyAddr)
	fmt.Fprintf(cf.Stdout(), "  export http_proxy=http://%s\n\n", proxy.httpProxyAddr)
	fmt.Fprintf(cf.Stdout(), "Use the GitHub API at http://api.github.localhost:\n\n")
	fmt.Fprintf(cf.Stdout(), "  curl http://api.github.localhost/user\n")
	fmt.Fprintf(cf.Stdout(), "  curl http://api.github.localhost/repos/%s/<repo>/issues\n\n", github.Organization)
	fmt.Fprintf(cf.Stdout(), "Use with gh CLI (additional env vars required):\n\n")
	fmt.Fprintf(cf.Stdout(), "  export GH_HOST=github.localhost\n")
	fmt.Fprintf(cf.Stdout(), "  export GH_TOKEN=teleport\n")
	fmt.Fprintf(cf.Stdout(), "  gh api /user\n\n")

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
	httpProxy     *http.Server
	httpProxyLn   net.Listener
	httpProxyAddr string
}

func (p *gitProxy) Close() error {
	var errs []error
	if p.httpProxy != nil {
		errs = append(errs, p.httpProxy.Close())
	}
	if p.httpProxyLn != nil {
		errs = append(errs, p.httpProxyLn.Close())
	}
	return trace.NewAggregate(errs...)
}

// GetGHEnvVars returns environment variables for gh CLI.
// Uses GH_HOST=github.localhost so gh sends plain HTTP (no TLS verification
// needed). The HTTP_PROXY routes requests through our local proxy which
// rewrites github.localhost to github.com and forwards to the Teleport proxy
// with mTLS via ALPN.
func (p *gitProxy) GetGHEnvVars() map[string]string {
	return map[string]string{
		"GH_TOKEN":   "teleport",
		"GH_HOST":    "github.localhost",
		"HTTP_PROXY": fmt.Sprintf("http://%s", p.httpProxyAddr),
		"http_proxy": fmt.Sprintf("http://%s", p.httpProxyAddr),
	}
}

// GetGitEnvVars returns environment variables for git remote-http.
// git remote-http is called with http:// URL and routed through this proxy.
func (p *gitProxy) GetGitEnvVars() map[string]string {
	return map[string]string{
		"HTTP_PROXY": fmt.Sprintf("http://%s", p.httpProxyAddr),
		"http_proxy": fmt.Sprintf("http://%s", p.httpProxyAddr),
	}
}

type gitProxyConfig struct {
	gitServerName string
	port          string
}

func startGitProxy(cf *CLIConf, tc *client.TeleportClient, cfg gitProxyConfig) (*gitProxy, error) {
	routeToGit := proto.RouteToGit{
		GitServerName: cfg.gitServerName,
	}

	gitCertChecker := client.NewGitCertChecker(tc, routeToGit, nil, client.WithTTL(tc.KeyTTL))

	if cert, err := loadAppCertificate(tc, cfg.gitServerName); err == nil {
		gitCertChecker.SetCert(cert)
	}

	listenAddr := "localhost:0"
	if cfg.port != "" {
		listenAddr = "localhost:" + cfg.port
	}
	httpProxyLn, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxy := &gitProxy{
		httpProxyLn:   httpProxyLn,
		httpProxyAddr: httpProxyLn.Addr().String(),
	}

	httpProxyServer := &http.Server{
		Handler: newGitHTTPProxyHandler(gitCertChecker, tc, cfg.gitServerName),
	}
	proxy.httpProxy = httpProxyServer

	go func() {
		if err := httpProxyServer.Serve(httpProxyLn); err != nil && err != http.ErrServerClosed {
			logger.ErrorContext(cf.Context, "Failed to start git HTTP proxy", "error", err)
		}
	}()

	logger.DebugContext(cf.Context, "Git proxy started",
		"http_proxy", proxy.httpProxyAddr,
	)

	return proxy, nil
}

// gitHTTPProxyHandler receives plain HTTP proxy requests from gh (via
// GH_HOST=github.localhost) and git remote-http (via http://github.com).
// It rewrites the host as needed and dials the Teleport proxy directly via
// ALPN with mTLS git cert.
type gitHTTPProxyHandler struct {
	certChecker   *client.CertChecker
	tc            *client.TeleportClient
	gitServerName string
}

func newGitHTTPProxyHandler(certChecker *client.CertChecker, tc *client.TeleportClient, gitServerName string) *gitHTTPProxyHandler {
	return &gitHTTPProxyHandler{
		certChecker:   certChecker,
		tc:            tc,
		gitServerName: gitServerName,
	}
}

func (h *gitHTTPProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger.DebugContext(r.Context(), "Git HTTP proxy received request",
		"method", r.Method,
		"host", r.Host,
		"url", r.URL.String(),
	)

	dialFunc := h.dialALPN
	if isBeamsEnvironment() {
		dialFunc = h.dialVNet
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = strings.Replace(req.URL.Host, "github.localhost", "github.com", 1)
			req.Host = strings.Replace(req.Host, "github.localhost", "github.com", 1)
		},
		Transport: &http.Transport{
			DialContext: dialFunc,
		},
		ModifyResponse: gitHTTPProxyModifyResponse,
	}
	proxy.ServeHTTP(w, r)
}

func (h *gitHTTPProxyHandler) dialALPN(ctx context.Context, network, addr string) (net.Conn, error) {
	cert, err := h.certChecker.GetOrIssueCert(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting git cert")
	}
	return h.tc.DialALPN(ctx, cert, alpncommon.ProtocolHTTP)
}

// dialVNet dials the git server through VNet. VNet handles ALPN tunneling and
// cert management, so tsh just makes a plain TCP connection.
func (h *gitHTTPProxyHandler) dialVNet(ctx context.Context, network, addr string) (net.Conn, error) {
	proxyHost, _, _ := net.SplitHostPort(h.tc.WebProxyAddr)
	vnetFQDN := fmt.Sprintf("%s.git.%s", h.gitServerName, proxyHost)
	logger.DebugContext(ctx, "Dialing git server through VNet", "fqdn", vnetFQDN)
	var d net.Dialer
	return d.DialContext(ctx, "tcp", net.JoinHostPort(vnetFQDN, "443"))
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
