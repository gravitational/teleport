/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
)

func onMCPStartDB(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	keyRing, err := tc.LocalAgent().GetCoreKeyRing()
	if err != nil {
		return trace.Wrap(err)
	}
	cred := client.TLSCredential{
		PrivateKey: keyRing.TLSPrivateKey,
		Cert:       keyRing.TLSCert,
	}
	cert, err := cred.TLSCertificate()
	if err != nil {
		return trace.Wrap(err)
	}

	in, out := net.Pipe()
	listener := listenerutils.NewSingleUseListener(out)
	defer listener.Close()

	lp, err := alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(cf.Context, tc, listener, tc.InsecureSkipVerify),
		alpnproxy.WithALPNProtocol(alpncommon.ProtocolMCP),
		alpnproxy.WithClientCert(cert),
		alpnproxy.WithClusterCAsIfConnUpgrade(cf.Context, tc.RootClusterCACertPool),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		defer lp.Close()
		if err = lp.Start(cf.Context); err != nil {
			logger.ErrorContext(cf.Context, "Failed to start local ALPN proxy", "error", err)
		}
	}()

	stdioConn := utils.CombinedStdio{}
	return utils.ProxyConn(cf.Context, in, stdioConn)
}
