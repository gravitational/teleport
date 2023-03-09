// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package servicecfg

import (
	"crypto/x509"
	"regexp"

	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/utils"
)

// WindowsDesktopConfig specifies the configuration for the Windows Desktop
// Access service.
type WindowsDesktopConfig struct {
	Enabled bool
	// ListenAddr is the address to listed on for incoming desktop connections.
	ListenAddr utils.NetAddr
	// PublicAddrs is a list of advertised public addresses of the service.
	PublicAddrs []utils.NetAddr
	// ShowDesktopWallpaper determines whether desktop sessions will show a
	// user-selected wallpaper vs a system-default, single-color wallpaper.
	ShowDesktopWallpaper bool
	// LDAP is the LDAP connection parameters.
	LDAP LDAPConfig

	// Discovery configures automatic desktop discovery via LDAP.
	Discovery LDAPDiscoveryConfig

	// Hosts is an optional list of static Windows hosts to expose through this
	// service.
	// Hosts is an optional list of static, AD-connected Windows hosts. This gives users
	// a way to specify AD-connected hosts that won't be found by the filters
	// specified in Discovery (or if Discovery is omitted).
	Hosts []utils.NetAddr

	// NonADHosts is an optional list of static Windows hosts to expose through this
	// service. These hosts are not part of Active Directory.
	NonADHosts []utils.NetAddr

	// ConnLimiter limits the connection and request rates.
	ConnLimiter limiter.Config
	// HostLabels specifies rules that are used to apply labels to Windows hosts.
	HostLabels HostLabelRules
	Labels     map[string]string
}

// LDAPDiscoveryConfig is LDAP discovery configuration for windows desktop discovery service.
type LDAPDiscoveryConfig struct {
	// BaseDN is the base DN to search for desktops.
	// Use the value '*' to search from the root of the domain,
	// or leave blank to disable desktop discovery.
	BaseDN string
	// Filters are additional LDAP filters to apply to the search.
	// See: https://ldap.com/ldap-filters/
	Filters []string
	// LabelAttributes are LDAP attributes to apply to hosts discovered
	// via LDAP. Teleport labels hosts by prefixing the attribute with
	// "ldap/" - for example, a value of "location" here would result in
	// discovered desktops having a label with key "ldap/location" and
	// the value being the value of the "location" attribute.
	LabelAttributes []string
}

// HostLabelRules is a collection of rules describing how to apply labels to hosts.
type HostLabelRules struct {
	rules  []HostLabelRule
	labels map[string]map[string]string
}

func NewHostLabelRules(rules ...HostLabelRule) HostLabelRules {
	return HostLabelRules{
		rules: rules,
	}
}

// LabelsForHost returns the set of all labels that should be applied
// to the specified host. If multiple rules match and specify the same
// label keys, the value will be that of the last matching rule.
func (h HostLabelRules) LabelsForHost(host string) map[string]string {
	labels, ok := h.labels[host]
	if ok {
		return labels
	}

	result := make(map[string]string)
	for _, rule := range h.rules {
		if rule.Regexp.MatchString(host) {
			for k, v := range rule.Labels {
				result[k] = v
			}
		}
	}

	if h.labels == nil {
		h.labels = make(map[string]map[string]string)
	}
	h.labels[host] = result

	return result
}

// HostLabelRule specifies a set of labels that should be applied to
// hosts matching the provided regexp.
type HostLabelRule struct {
	Regexp *regexp.Regexp
	Labels map[string]string
}

// LDAPConfig is the LDAP connection parameters.
type LDAPConfig struct {
	// Addr is the address:port of the LDAP server (typically port 389).
	Addr string
	// Domain is the ActiveDirectory domain name.
	Domain string
	// Username for LDAP authentication.
	Username string
	// SID is the SID for the user specified by Username.
	SID string
	// InsecureSkipVerify decides whether whether we skip verifying with the LDAP server's CA when making the LDAPS connection.
	InsecureSkipVerify bool
	// ServerName is the name of the LDAP server for TLS.
	ServerName string
	// CA is an optional CA cert to be used for verification if InsecureSkipVerify is set to false.
	CA *x509.Certificate
}
