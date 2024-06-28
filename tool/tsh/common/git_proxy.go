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
	"strings"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/client"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

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
