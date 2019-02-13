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
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
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
	t := asciitable.MakeTable([]string{"Role", "Allowed to login as", "Node Labels", "Access to resources"})
	for _, r := range r.roles {
		if r.GetName() == teleport.DefaultImplicitRole {
			continue
		}
		t.AddRow([]string{
			r.GetMetadata().Name,
			strings.Join(r.GetLogins(services.Allow), ","),
			printNodeLabels(r.GetNodeLabels(services.Allow)),
			printActions(r.GetRules(services.Allow))})
	}
	_, err := t.AsBuffer().WriteTo(w)
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
	return utils.WriteYAML(w, r.toMarshal())
}

type namespaceCollection struct {
	namespaces []services.Namespace
}

func (n *namespaceCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, n := range n.namespaces {
		t.AddRow([]string{n.Metadata.Name})
	}
	_, err := t.AsBuffer().WriteTo(w)
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
	return utils.WriteYAML(w, n.toMarshal())
}

func printActions(rules []services.Rule) string {
	pairs := []string{}
	for _, rule := range rules {
		pairs = append(pairs, fmt.Sprintf("%v:%v", strings.Join(rule.Resources, ","), strings.Join(rule.Verbs, ",")))
	}
	return strings.Join(pairs, ",")
}

func printNodeLabels(labels services.Labels) string {
	pairs := []string{}
	for key, values := range labels {
		if key == services.Wildcard {
			return "<all nodes>"
		}
		pairs = append(pairs, fmt.Sprintf("%v=%v", key, values))
	}
	return strings.Join(pairs, ",")
}

type serverCollection struct {
	servers []services.Server
}

func (s *serverCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Nodename", "UUID", "Address", "Labels"})
	for _, s := range s.servers {
		t.AddRow([]string{
			s.GetHostname(), s.GetName(), s.GetAddr(), s.LabelsString(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
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
	return utils.WriteYAML(w, r.toMarshal())
}

type userCollection struct {
	users []services.User
}

func (s *userCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"User"})
	for _, u := range s.users {
		t.AddRow([]string{u.GetName()})
	}
	fmt.Println(t.AsBuffer().String())
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
	return utils.WriteYAML(w, r.toMarshal())
}

type authorityCollection struct {
	cas []services.CertAuthority
}

func (a *authorityCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Cluster Name", "CA Type", "Fingerprint", "Role Map"})
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
			t.AddRow([]string{
				a.GetClusterName(),
				string(a.GetType()),
				fingerprint,
				roles,
			})
		}
	}
	_, err := t.AsBuffer().WriteTo(w)
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
	return utils.WriteYAML(w, a.toMarshal())
}

type reverseTunnelCollection struct {
	tunnels []services.ReverseTunnel
}

func (r *reverseTunnelCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Cluster Name", "Dial Addresses"})
	for _, tunnel := range r.tunnels {
		t.AddRow([]string{
			tunnel.GetClusterName(), strings.Join(tunnel.GetDialAddrs(), ","),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
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
	return utils.WriteYAML(w, r.toMarshal())
}

type oidcCollection struct {
	connectors []services.OIDCConnector
}

func (c *oidcCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Name", "Issuer URL", "Additional Scope"})
	for _, conn := range c.connectors {
		t.AddRow([]string{
			conn.GetName(), conn.GetIssuerURL(), strings.Join(conn.GetScope(), ","),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
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
	return utils.WriteYAML(w, c.toMarshal())
}

type samlCollection struct {
	connectors []services.SAMLConnector
}

func (c *samlCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Name", "SSO URL"})
	for _, conn := range c.connectors {
		t.AddRow([]string{conn.GetName(), conn.GetSSO()})
	}
	t.AsBuffer().WriteTo(w)
	return nil
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
	return utils.WriteYAML(w, c.toMarshal())
}

type connectorsCollection struct {
	oidc   []services.OIDCConnector
	saml   []services.SAMLConnector
	github []services.GithubConnector
}

func (c *connectorsCollection) writeText(w io.Writer) error {
	if len(c.oidc) > 0 {
		_, err := io.WriteString(w, "\nOIDC:\n")
		if err != nil {
			return trace.Wrap(err)
		}
		oc := &oidcCollection{connectors: c.oidc}
		err = oc.writeText(w)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if len(c.saml) > 0 {
		_, err := io.WriteString(w, "\nSAML:\n")
		sc := &samlCollection{connectors: c.saml}
		err = sc.writeText(w)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if len(c.github) > 0 {
		_, err := io.WriteString(w, "\nGitHub:\n")
		if err != nil {
			return trace.Wrap(err)
		}
		gc := &githubCollection{connectors: c.github}
		err = gc.writeText(w)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (c *connectorsCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(c.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (c *connectorsCollection) toMarshal() interface{} {
	var connectors []interface{}

	for _, o := range c.oidc {
		connectors = append(connectors, o)
	}
	for _, s := range c.saml {
		connectors = append(connectors, s)
	}
	for _, g := range c.github {
		connectors = append(connectors, g)
	}

	return connectors
}

func (c *connectorsCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.toMarshal())
}

type trustedClusterCollection struct {
	trustedClusters []services.TrustedCluster
}

func (c *trustedClusterCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{
		"Name", "Enabled", "Token", "Proxy Address", "Reverse Tunnel Address", "Role Map"})
	for _, tc := range c.trustedClusters {
		t.AddRow([]string{
			tc.GetName(),
			strconv.FormatBool(tc.GetEnabled()),
			tc.GetToken(),
			tc.GetProxyAddress(),
			tc.GetReverseTunnelAddress(),
			fmt.Sprintf("%v", tc.CombinedMapping()),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
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
	return utils.WriteYAML(w, c.toMarshal())
}

type githubCollection struct {
	connectors []services.GithubConnector
}

func (c *githubCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Name", "Teams To Logins"})
	for _, conn := range c.connectors {
		t.AddRow([]string{conn.GetName(), formatTeamsToLogins(
			conn.GetTeamsToLogins())})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (c *githubCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(c.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (c *githubCollection) toMarshal() interface{} {
	if len(c.connectors) == 1 {
		return c.connectors[0]
	}
	return c.connectors
}

func (c *githubCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.toMarshal())
}

func formatTeamsToLogins(mappings []services.TeamMapping) string {
	var result []string
	for _, m := range mappings {
		result = append(result, fmt.Sprintf("@%v/%v: %v",
			m.Organization, m.Team, strings.Join(m.Logins, ", ")))
	}
	return strings.Join(result, ", ")
}

type remoteClusterCollection struct {
	remoteClusters []services.RemoteCluster
}

func (c *remoteClusterCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Name", "Status", "Last Heartbeat"})
	for _, cluster := range c.remoteClusters {
		lastHeartbeat := cluster.GetLastHeartbeat()
		t.AddRow([]string{cluster.GetName(), cluster.GetConnectionStatus(), formatLastHeartbeat(lastHeartbeat)})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func formatLastHeartbeat(t time.Time) string {
	if t.IsZero() {
		return "not available"
	}
	return utils.HumanTimeFormat(t)
}

func (c *remoteClusterCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(c.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (c *remoteClusterCollection) toMarshal() interface{} {
	if len(c.remoteClusters) == 1 {
		return c.remoteClusters[0]
	}
	return c.remoteClusters
}

func (c *remoteClusterCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.toMarshal())
}
