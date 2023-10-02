/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package awsoidc

import (
	"context"
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// ProxyGetter is a service that gets proxies.
type ProxiesGetter interface {
	// GetProxies returns a list of registered proxies.
	GetProxies() ([]types.Server, error)
}

// IssuerForCluster returns the issuer URL using the Cluster state.
func IssuerForCluster(ctx context.Context, clt ProxiesGetter) (string, error) {
	proxies, err := clt.GetProxies()
	if err != nil {
		return "", trace.Wrap(err)
	}

	for _, p := range proxies {
		proxyPublicAddress := p.GetPublicAddr()
		if proxyPublicAddress != "" {
			return IssuerFromPublicAddress(proxyPublicAddress)
		}
	}

	return "", trace.BadParameter("failed to get Proxy Public Address")
}

// IssuerFromPublicAddress is the address for the AWS OIDC Provider.
// It must match exactly what was introduced in AWS IAM console when adding the Identity Provider.
// PublicProxyAddr from `teleport.yaml/proxy` does not come with the desired format: it misses the protocol and has a port
// This method adds the `https` protocol and removes the port if it is the default one for https (443)
func IssuerFromPublicAddress(addr string) (string, error) {
	// Add protocol if not present.
	if !strings.HasPrefix(addr, "https://") && !strings.HasPrefix(addr, "http://") {
		addr = "https://" + addr
	}

	result, err := url.Parse(addr)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if result.Port() == "443" {
		// Cut off redundant :443
		result.Host = result.Hostname()
	}
	return result.String(), nil
}
