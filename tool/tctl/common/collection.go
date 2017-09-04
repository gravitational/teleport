/*
Copyright 2015-2017 Gravitational, Inc.

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

package common

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"

	"github.com/buger/goterm"
	"github.com/ghodss/yaml"
)

type ResourceCollection interface {
	writeText(w io.Writer) error
	writeJSON(w io.Writer) error
	writeYAML(w io.Writer) error
}

type roleCollection struct {
	roles []services.Role
}

func (r *roleCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	PrintHeader(t, []string{"Role", "Allowed to login as", "Node Labels", "Access to resources"})
	if len(r.roles) == 0 {
		_, err := io.WriteString(w, t.String())
		return trace.Wrap(err)
	}
	for _, r := range r.roles {
		if r.GetName() == teleport.DefaultImplicitRole {
			continue
		}
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\n",
			r.GetMetadata().Name,
			strings.Join(r.GetLogins(services.Allow), ","),
			printNodeLabels(r.GetNodeLabels(services.Allow)),
			printActions(r.GetRules(services.Allow)))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (r *roleCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(r.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (r *roleCollection) toMarshal() interface{} {
	if len(r.roles) == 1 {
		return r.roles[0]
	}
	return r.roles
}

func (r *roleCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(r.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type namespaceCollection struct {
	namespaces []services.Namespace
}

func (n *namespaceCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	PrintHeader(t, []string{"Name"})
	if len(n.namespaces) == 0 {
		_, err := io.WriteString(w, t.String())
		return trace.Wrap(err)
	}
	for _, n := range n.namespaces {
		fmt.Fprintf(t, "%v\n", n.Metadata.Name)
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (n *namespaceCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(n.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (n *namespaceCollection) toMarshal() interface{} {
	if len(n.namespaces) == 1 {
		return n.namespaces[0]
	}
	return n.namespaces
}

func (n *namespaceCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(n.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func printActions(rules []services.Rule) string {
	pairs := []string{}
	for _, rule := range rules {
		pairs = append(pairs, fmt.Sprintf("%v:%v", strings.Join(rule.Resources, ","), strings.Join(rule.Verbs, ",")))
	}
	return strings.Join(pairs, ",")
}

func printNodeLabels(labels map[string]string) string {
	pairs := []string{}
	for key, val := range labels {
		if key == services.Wildcard {
			return "<all nodes>"
		}
		pairs = append(pairs, fmt.Sprintf("%v=%v", key, val))
	}
	return strings.Join(pairs, ",")
}

type serverCollection struct {
	servers []services.Server
}

func (s *serverCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	PrintHeader(t, []string{"Hostname", "UUID", "Address", "Labels"})
	if len(s.servers) == 0 {
		_, err := io.WriteString(w, t.String())
		return trace.Wrap(err)
	}
	for _, s := range s.servers {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", s.GetHostname(), s.GetName(), s.GetAddr(), s.LabelsString())
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (s *serverCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(s.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (s *serverCollection) toMarshal() interface{} {
	if len(s.servers) == 1 {
		return s.servers[0]
	}
	return s.servers
}

func (r *serverCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(r.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type userCollection struct {
	users []services.User
}

func (s *userCollection) writeText(w io.Writer) error {
	// a user collection does not need this method
	return nil
}

func (s *userCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(s.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (s *userCollection) toMarshal() interface{} {
	if len(s.users) == 1 {
		return s.users[0]
	}
	return s.users
}

func (r *userCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(r.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type authorityCollection struct {
	cas []services.CertAuthority
}

func (a *authorityCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	PrintHeader(t, []string{"Cluster Name", "CA Type", "Fingerprint", "Role Map"})
	for _, a := range a.cas {
		for _, keyBytes := range a.GetCheckingKeys() {
			fingerprint, err := sshutils.AuthorizedKeyFingerprint(keyBytes)
			if err != nil {
				fingerprint = fmt.Sprintf("<bad key: %v>", err)
			}
			var roles string
			if a.GetType() == services.HostCA {
				roles = "N/A"
			} else {
				roles = fmt.Sprintf("%v", a.CombinedMapping())
			}
			fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", a.GetClusterName(), a.GetType(), fingerprint, roles)
		}
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (a *authorityCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(a.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (a *authorityCollection) toMarshal() interface{} {
	if len(a.cas) == 1 {
		return a.cas[0]
	}
	return a.cas
}

func (a *authorityCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(a.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type reverseTunnelCollection struct {
	tunnels []services.ReverseTunnel
}

func (r *reverseTunnelCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	PrintHeader(t, []string{"Cluster Name", "Dial Addresses"})
	for _, tunnel := range r.tunnels {
		fmt.Fprintf(t, "%v\t%v\n", tunnel.GetClusterName(), strings.Join(tunnel.GetDialAddrs(), ","))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (r *reverseTunnelCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(r.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (r *reverseTunnelCollection) toMarshal() interface{} {
	if len(r.tunnels) == 1 {
		return r.tunnels[0]
	}
	return r.tunnels
}

func (r *reverseTunnelCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(r.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type oidcCollection struct {
	connectors []services.OIDCConnector
}

func (c *oidcCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	PrintHeader(t, []string{"Name", "Issuer URL", "Additional Scope"})
	for _, conn := range c.connectors {
		fmt.Fprintf(t, "%v\t%v\t%v\n", conn.GetName(), conn.GetIssuerURL(), strings.Join(conn.GetScope(), ","))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (c *oidcCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(c.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (c *oidcCollection) toMarshal() interface{} {
	if len(c.connectors) == 1 {
		return c.connectors[0]
	}
	return c.connectors
}

func (c *oidcCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(c.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type samlCollection struct {
	connectors []services.SAMLConnector
}

func (c *samlCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	PrintHeader(t, []string{"Name", "SSO URL"})
	for _, conn := range c.connectors {
		fmt.Fprintf(t, "%v\t%v\n", conn.GetName(), conn.GetSSO())
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (c *samlCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(c.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (c *samlCollection) toMarshal() interface{} {
	if len(c.connectors) == 1 {
		return c.connectors[0]
	}
	return c.connectors
}

func (c *samlCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(c.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type trustedClusterCollection struct {
	trustedClusters []services.TrustedCluster
}

func (c *trustedClusterCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	PrintHeader(t, []string{"Name", "Enabled", "Token", "Proxy Address", "Reverse Tunnel Address", "Role Map"})
	for _, tc := range c.trustedClusters {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\t%v\t%v\n", tc.GetName(), tc.GetEnabled(), tc.GetToken(), tc.GetProxyAddress(), tc.GetReverseTunnelAddress(), tc.CombinedMapping())
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (c *trustedClusterCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(c.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (c *trustedClusterCollection) toMarshal() interface{} {
	if len(c.trustedClusters) == 1 {
		return c.trustedClusters[0]
	}
	return c.trustedClusters
}

func (c *trustedClusterCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(c.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type authPreferenceCollection struct {
	services.AuthPreference
}

func (c *authPreferenceCollection) writeText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	PrintHeader(t, []string{"Type", "Second Factor"})
	fmt.Fprintf(t, "%v\t%v\n", c.GetType(), c.GetSecondFactor())
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func (c *authPreferenceCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(c.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (c *authPreferenceCollection) toMarshal() interface{} {
	return c
}

func (c *authPreferenceCollection) writeYAML(w io.Writer) error {
	data, err := yaml.Marshal(c.toMarshal())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}
