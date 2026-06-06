// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package join

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// ProxyServerHTTPListenPortHint returns user-facing text suggesting proxy_server
// be set to host:443 when the address uses the default non-TLS proxy listen
// port (3080), or when the port was omitted and the host is a Teleport Cloud
// hostname (utils.NetAddr then defaults to that port). Returns "" when no hint
// applies.
func ProxyServerHTTPListenPortHint(proxyAddr string) string {
	host, portStr, splitErr := net.SplitHostPort(proxyAddr)
	if splitErr == nil && portStr == strconv.Itoa(defaults.HTTPListenPort) {
		httpsAddr := net.JoinHostPort(host, strconv.Itoa(teleport.StandardHTTPSPort))
		return fmt.Sprintf(
			"If %d is blocked or unreachable, set proxy_server to %s",
			defaults.HTTPListenPort,
			httpsAddr,
		)
	}
	if splitErr != nil {
		na := utils.NetAddr{Addr: proxyAddr}
		if na.Port(defaults.HTTPListenPort) == defaults.HTTPListenPort {
			h := na.Host()
			if strings.HasSuffix(h, "."+defaults.CloudDomainSuffix) {
				httpsAddr := net.JoinHostPort(h, strconv.Itoa(teleport.StandardHTTPSPort))
				return fmt.Sprintf(
					"If %d is blocked or unreachable, set proxy_server to %s",
					defaults.HTTPListenPort,
					httpsAddr,
				)
			}
		}
	}
	return ""
}
