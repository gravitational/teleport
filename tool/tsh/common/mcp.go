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

	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

func onMCPStart(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) currently assumes app login is done
	cert, err := loadAppCertificate(tc, cf.AppName)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(greedy52) use a single use listener to avoid this dial
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return trace.Wrap(err)
	}
	defer listener.Close()

	lp, err := alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(cf.Context, tc, listener, tc.InsecureSkipVerify),
		alpnproxy.WithALPNProtocol(alpncommon.ProtocolTCP),
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

	clientConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		return trace.Wrap(err)
	}

	stdioConn := utils.CombinedStdio{}
	return utils.ProxyConn(cf.Context, clientConn, stdioConn)
}
