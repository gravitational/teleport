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
	"fmt"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// AssembleAppFQDN returns the application's FQDN.
//
// If the application is running within the local cluster and it has a public
// address specified, the application's public address is used, unless it is specified
// that the app should always use the proxy public addr.
//
// In all other cases, i.e. if the public address is not set or the application
// is running in a remote cluster, the FQDN is formatted as
// <appName>.<localProxyDNSName>
func AssembleAppFQDN(localClusterName string, localProxyDNSName string, appClusterName string, app types.Application) string {
	isLocalCluster := localClusterName == appClusterName
	if isLocalCluster && app.GetPublicAddr() != "" && !app.GetAlwaysUseProxyPublicAddr() {
		return app.GetPublicAddr()
	}
	return DefaultAppPublicAddr(app.GetName(), localProxyDNSName)
}

// DefaultAppPublicAddr returns the default publicAddr for an app.
// Format: <appName>.<localProxyDNSName>
func DefaultAppPublicAddr(appName, localProxyDNSName string) string {
	return fmt.Sprintf("%v.%v", appName, localProxyDNSName)
}

// ExtractAppAndProxyName tries to extract an app name and proxyDNSName from a given fqdn. If a proxyDNS name is not
// found, an empty app name and the default param will be returned.
func ExtractAppAndProxyName(fqdn string, proxyDNSNames []string, defaultPublicAddr string) (string, string, error) {
	// Split the FQDN into its components.
	fqdnParts := strings.Split(fqdn, ".")
	proxyDNSName := ""
	appName := ""

	// check each part to find the proxyDNSName.
	for i := 0; i < len(fqdnParts); i++ {
		potentialDNS := strings.Join(fqdnParts[i:], ".")
		if slices.Contains(proxyDNSNames, potentialDNS) {
			proxyDNSName = potentialDNS
			appName = strings.Join(fqdnParts[:i], ".")
			break
		}
	}

	if proxyDNSName == "" {
		return "", defaultPublicAddr, trace.BadParameter("FQDN %q does not match any known proxy DNS names", fqdn)
	}

	return appName, proxyDNSName, nil
}
