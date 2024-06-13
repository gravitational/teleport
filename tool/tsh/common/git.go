/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/config/openssh"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

func onGitClone(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	app, err := getRegisteredApp(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	if !app.IsGitHub() {
		return trace.BadParameter("app %v of type %v is not supported for %v", app.GetName(), app.GetProtocol(), cf.command)
	}

	org, repo, ok := parseGitURL(cf.GitURL)
	if !ok {
		return trace.BadParameter("bad git URL %s", cf.GitURL)
	}

	if app.GetGitHubOrganization() != org {
		return trace.BadParameter("app %s is intended for organziation %s but got %s", app.GetName(), app.GetGitHubOrganization(), org)
	}

	cf.GitSaveSSHConfig = true
	if err := onGitSSHConfig(cf); err != nil {
		return trace.Wrap(err)
	}

	localGitURL := fmt.Sprintf("git@%s.teleport-git-app.%s:%s/%s.git", app.GetName(), tc.WebProxyHost(), org, repo)
	slog.DebugContext(cf.Context, "Calling git clone.", "url", localGitURL)

	cmd := exec.Command("git", "clone", localGitURL)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return trace.Wrap(cmd.Run())
}

func shouldProxyGitSSH(cf *CLIConf, tc *client.TeleportClient) bool {
	if tc.HostLogin != "git" {
		return false
	}

	targetHost, _, err := net.SplitHostPort(tc.Host)
	if err != nil {
		return false
	}
	wantSuffix := fmt.Sprintf(".teleport-git-app.%s", tc.WebProxyHost())
	return strings.HasSuffix(targetHost, wantSuffix)
}

func onProxyCommandGitSSH(cf *CLIConf, tc *client.TeleportClient) error {
	appName, _, ok := strings.Cut(tc.Host, ".teleport-git-app.")
	if !ok {
		return trace.BadParameter("bad host %s", tc.Host)
	}
	cf.AppName = appName

	slog.DebugContext(cf.Context, "Proxy git SSH.", "app", cf.AppName, "host", cf.UserHost)

	appCert, needLogin, err := loadAppCertificate(tc, cf.AppName)
	if err != nil {
		return trace.Wrap(err)
	}
	if needLogin {
		return trace.AccessDenied("app session for %q is expired. Please login the app with `tsh apps login %v", cf.AppName, cf.AppName)
	}

	// TODO make this a helper?
	dialer := apiclient.NewALPNDialer(apiclient.ALPNDialerConfig{
		ALPNConnUpgradeRequired: tc.TLSRoutingConnUpgradeRequired,
		GetClusterCAs:           tc.RootClusterCACertPool,
		TLSConfig: &tls.Config{
			// TODO should we use -ping?
			NextProtos:         []string{string(alpncommon.ProtocolGitSSH)},
			InsecureSkipVerify: tc.InsecureSkipVerify,
			Certificates:       []tls.Certificate{appCert},
		},
	})
	serverConn, err := dialer.DialContext(cf.Context, "tcp", tc.WebProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(utils.ProxyConn(cf.Context, utils.NewCombinedStdioWithProperClose(cf.Context), serverConn))
}

func onGitSSHConfig(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	sshConf := openssh.NewSSHConfig(nil, log)
	keysDir := profile.FullProfilePath(tc.Config.KeysDir)
	knownHostsPath := keypaths.KnownHostsPath(keysDir)
	params := &openssh.SSHConfigParameters{
		AppName:        openssh.TshGitApp,
		ClusterNames:   []string{tc.WebProxyHost()},
		KnownHostsPath: knownHostsPath,
		ProxyHost:      tc.WebProxyHost(),
		ProxyPort:      fmt.Sprintf("%d", tc.WebProxyPort()),
		ExecutablePath: cf.executablePath,
	}
	if !cf.GitSaveSSHConfig {
		var sb strings.Builder
		if err := sshConf.GetSSHConfig(&sb, params); err != nil {
			return trace.Wrap(err)
		}

		fmt.Fprint(cf.Stdout(), sb.String())
		return nil
	}

	return trace.Wrap(sshConf.SaveToUserConfig(params))
}

func parseGitURL(input string) (string, string, bool) {
	if strings.HasSuffix(input, ".git") {
		return parseGitURL(strings.TrimSuffix(input, ".git"))
	}

	switch {
	case strings.HasPrefix(input, "https://"):
		httpURL, err := url.Parse(input)
		if err != nil {
			return "", "", false
		}
		return parseGitURL(strings.TrimPrefix(httpURL.Path, "/"))

	case strings.Contains(input, "@") && strings.Contains(input, ":"):
		_, orgAndRepo, ok := strings.Cut(input, ":")
		if !ok {
			return "", "", false
		}
		return parseGitURL(orgAndRepo)

	case strings.Count(input, "/") == 1:
		org, repo, ok := strings.Cut(input, "/")
		if !ok {
			return "", "", false
		}
		return org, repo, true

	default:
		return "", "", false
	}
}
