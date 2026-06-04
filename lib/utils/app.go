// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package utils

import (
	"cmp"
	"fmt"
	"net"
	"strings"

	"github.com/gravitational/teleport/api/types"
)

// AssembleAppFQDN returns the application's FQDN.
//
// If the application is running within the local cluster and it has a public
// address specified, the application's public address is used.
//
// In all other cases, i.e. if the public address is not set or the application
// is running in a remote cluster, the FQDN is formatted as
// <appName>.<localProxyDNSName>
func AssembleAppFQDN(localClusterName string, localProxyDNSName string, appClusterName string, app types.Application) string {
	isLocalCluster := localClusterName == appClusterName
	if isLocalCluster && app.GetPublicAddr() != "" && !app.GetUseAnyProxyPublicAddr() {
		return app.GetPublicAddr()
	}
	return DefaultAppPublicAddr(app.GetName(), localProxyDNSName)
}

// DefaultAppPublicAddr returns "<appName>.<localProxyDNSName>",
// stripping a trailing port and lowercasing the host so the result
// satisfies ValidatePublicAddr.
func DefaultAppPublicAddr(appName, localProxyDNSName string) string {
	if host, _, err := net.SplitHostPort(localProxyDNSName); err == nil {
		localProxyDNSName = host
	}
	return fmt.Sprintf("%s.%s", appName, strings.ToLower(localProxyDNSName))
}

// DefaultAppFQDN returns the default routing FQDN for an app.
// proxyPublicAddrHost takes precedence; clusterName is the fallback
// when it is empty. An IP-valued proxy public_addr is used as-is,
// not replaced by clusterName.
func DefaultAppFQDN(appName, proxyPublicAddrHost, clusterName string) string {
	return DefaultAppPublicAddr(appName, cmp.Or(proxyPublicAddrHost, clusterName))
}
