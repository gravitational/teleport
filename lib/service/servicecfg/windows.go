/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package servicecfg

import (
	"crypto/x509"
	"maps"
	"regexp"

	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
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
	// PKIDomain optionally configures a separate Active Directory domain
	// for PKI operations. If empty, the domain from the LDAP config is used.
	// This can be useful for cases where PKI is configured in a root domain
	// but Teleport is used to provide access to users and computers in a child
	// domain.
	PKIDomain string
	// KDCAddr optionally configure the address of the Kerberos Key Distribution Center,
	// which is used to support RDP Network Level Authentication (NLA).
	// If empty, the LDAP address will be used instead.
	// Note: NLA is only supported in Active Directory environments - this field has
	// no effect when connecting to desktops as local Windows users.
	KDCAddr string

	// Discovery configures automatic desktop discovery via LDAP.
	Discovery LDAPDiscoveryConfig

	// StaticHosts is an optional list of static Windows hosts to expose through this
	// service.
	StaticHosts []WindowsHost

	// ConnLimiter limits the connection and request rates.
	ConnLimiter limiter.Config
	// HostLabels specifies rules that are used to apply labels to Windows hosts.
	HostLabels HostLabelRules
	Labels     map[string]string
	// ResourceMatchers match dynamic Windows desktop resources.
	ResourceMatchers []services.ResourceMatcher
}

// WindowsHost is configuration for single Windows desktop host
type WindowsHost struct {
	// Name that will be used in the Teleport UI
	Name string
	// Address of the remote Windows host
	Address utils.NetAddr
	// AD is true if the host is part of the Active Directory domain
	AD bool
	// Labels to be applied to the host
	Labels map[string]string
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
			maps.Copy(result, rule.Labels)
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
