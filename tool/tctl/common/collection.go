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

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

type ResourceCollection interface {
	writeText(w io.Writer) error
	resources() []types.Resource
}

type roleCollection struct {
	roles   []types.Role
	verbose bool
}

func (r *roleCollection) resources() (res []types.Resource) {
	for _, resource := range r.roles {
		res = append(res, resource)
	}
	return res
}

func (r *roleCollection) writeText(w io.Writer) error {
	var rows [][]string
	for _, r := range r.roles {
		if r.GetName() == constants.DefaultImplicitRole {
			continue
		}
		rows = append(rows, []string{
			r.GetMetadata().Name,
			strings.Join(r.GetLogins(types.Allow), ","),
			printNodeLabels(r.GetNodeLabels(types.Allow)),
			printActions(r.GetRules(types.Allow))})
	}

	headers := []string{"Role", "Allowed to login as", "Node Labels", "Access to resources"}
	var t asciitable.Table
	if r.verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Access to resources")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type namespaceCollection struct {
	namespaces []types.Namespace
}

func (n *namespaceCollection) resources() (r []types.Resource) {
	for _, resource := range n.namespaces {
		r = append(r, &resource)
	}
	return r
}

func (n *namespaceCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, n := range n.namespaces {
		t.AddRow([]string{n.Metadata.Name})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func printActions(rules []types.Rule) string {
	pairs := []string{}
	for _, rule := range rules {
		pairs = append(pairs, fmt.Sprintf("%v:%v", strings.Join(rule.Resources, ","), strings.Join(rule.Verbs, ",")))
	}
	return strings.Join(pairs, ",")
}

func printMetadataLabels(labels map[string]string) string {
	pairs := []string{}
	for key, value := range labels {
		pairs = append(pairs, fmt.Sprintf("%v=%v", key, value))
	}
	return strings.Join(pairs, ",")
}

func printNodeLabels(labels types.Labels) string {
	pairs := []string{}
	for key, values := range labels {
		if key == types.Wildcard {
			return "<all nodes>"
		}
		pairs = append(pairs, fmt.Sprintf("%v=%v", key, values))
	}
	return strings.Join(pairs, ",")
}

type serverCollection struct {
	servers []types.Server
	verbose bool
}

func (s *serverCollection) resources() (r []types.Resource) {
	for _, resource := range s.servers {
		r = append(r, resource)
	}
	return r
}

func (s *serverCollection) writeText(w io.Writer) error {
	var rows [][]string
	for _, se := range s.servers {
		labels := stripInternalTeleportLabels(s.verbose, se.GetAllLabels())
		rows = append(rows, []string{
			se.GetHostname(), se.GetName(), se.GetAddr(), labels, se.GetTeleportVersion(),
		})
	}
	headers := []string{"Host", "UUID", "Public Address", "Labels", "Version"}
	var t asciitable.Table
	if s.verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")

	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (s *serverCollection) writeYaml(w io.Writer) error {
	return utils.WriteYAML(w, s.servers)
}

func (s *serverCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(s.resources(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type userCollection struct {
	users []types.User
}

func (u *userCollection) resources() (r []types.Resource) {
	for _, resource := range u.users {
		r = append(r, resource)
	}
	return r
}

func (u *userCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"User"})
	for _, user := range u.users {
		t.AddRow([]string{user.GetName()})
	}
	fmt.Println(t.AsBuffer().String())
	return nil
}

type authorityCollection struct {
	cas []types.CertAuthority
}

func (a *authorityCollection) resources() (r []types.Resource) {
	for _, resource := range a.cas {
		r = append(r, resource)
	}
	return r
}

func (a *authorityCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Cluster Name", "CA Type", "Fingerprint", "Role Map"})
	for _, a := range a.cas {
		for _, key := range a.GetTrustedSSHKeyPairs() {
			fingerprint, err := sshutils.AuthorizedKeyFingerprint(key.PublicKey)
			if err != nil {
				fingerprint = fmt.Sprintf("<bad key: %v>", err)
			}
			var roles string
			if a.GetType() == types.HostCA {
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

type reverseTunnelCollection struct {
	tunnels []types.ReverseTunnel
}

func (r *reverseTunnelCollection) resources() (res []types.Resource) {
	for _, resource := range r.tunnels {
		res = append(res, resource)
	}
	return res
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

type oidcCollection struct {
	connectors []types.OIDCConnector
}

func (c *oidcCollection) resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
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

type samlCollection struct {
	connectors []types.SAMLConnector
}

func (c *samlCollection) resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
}

func (c *samlCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Name", "SSO URL"})
	for _, conn := range c.connectors {
		t.AddRow([]string{conn.GetName(), conn.GetSSO()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type connectorsCollection struct {
	oidc   []types.OIDCConnector
	saml   []types.SAMLConnector
	github []types.GithubConnector
}

func (c *connectorsCollection) resources() (r []types.Resource) {
	for _, resource := range c.oidc {
		r = append(r, resource)
	}
	for _, resource := range c.saml {
		r = append(r, resource)
	}
	for _, resource := range c.github {
		r = append(r, resource)
	}
	return r
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
		if err != nil {
			return trace.Wrap(err)
		}
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

type trustedClusterCollection struct {
	trustedClusters []types.TrustedCluster
}

func (c *trustedClusterCollection) resources() (r []types.Resource) {
	for _, resource := range c.trustedClusters {
		r = append(r, resource)
	}
	return r
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

type githubCollection struct {
	connectors []types.GithubConnector
}

func (c *githubCollection) resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
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

func formatTeamsToLogins(mappings []types.TeamMapping) string {
	var result []string
	for _, m := range mappings {
		result = append(result, fmt.Sprintf("@%v/%v: %v",
			m.Organization, m.Team, strings.Join(m.Logins, ", ")))
	}
	return strings.Join(result, ", ")
}

type remoteClusterCollection struct {
	remoteClusters []types.RemoteCluster
}

func (c *remoteClusterCollection) resources() (r []types.Resource) {
	for _, resource := range c.remoteClusters {
		r = append(r, resource)
	}
	return r
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
	return apiutils.HumanTimeFormat(t)
}

func writeJSON(c ResourceCollection, w io.Writer) error {
	data, err := json.MarshalIndent(c.resources(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func writeYAML(c ResourceCollection, w io.Writer) error {
	return utils.WriteYAML(w, c.resources())
}

type semaphoreCollection struct {
	sems []types.Semaphore
}

func (c *semaphoreCollection) resources() (r []types.Resource) {
	for _, resource := range c.sems {
		r = append(r, resource)
	}
	return r
}

func (c *semaphoreCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Kind", "Name", "LeaseID", "Holder", "Expires"})
	for _, sem := range c.sems {
		for _, ref := range sem.LeaseRefs() {
			t.AddRow([]string{
				sem.GetSubKind(), sem.GetName(), ref.LeaseID, ref.Holder, ref.Expires.Format(time.RFC822),
			})
		}
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type appServerCollection struct {
	servers []types.AppServer
	verbose bool
}

func (a *appServerCollection) resources() (r []types.Resource) {
	for _, resource := range a.servers {
		r = append(r, resource)
	}
	return r
}

func (a *appServerCollection) writeText(w io.Writer) error {
	var rows [][]string
	for _, server := range a.servers {
		app := server.GetApp()
		labels := stripInternalTeleportLabels(a.verbose, app.GetAllLabels())
		rows = append(rows, []string{
			server.GetHostname(), app.GetName(), app.GetProtocol(), app.GetPublicAddr(), app.GetURI(), labels, server.GetTeleportVersion()})
	}
	var t asciitable.Table
	headers := []string{"Host", "Name", "Type", "Public Address", "URI", "Labels", "Version"}
	if a.verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (a *appServerCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(a.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (a *appServerCollection) toMarshal() interface{} {
	return a.servers
}

func (a *appServerCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, a.toMarshal())
}

type appCollection struct {
	apps    []types.Application
	verbose bool
}

func (c *appCollection) resources() (r []types.Resource) {
	for _, resource := range c.apps {
		r = append(r, resource)
	}
	return r
}

func (c *appCollection) writeText(w io.Writer) error {
	var rows [][]string
	for _, app := range c.apps {
		labels := stripInternalTeleportLabels(c.verbose, app.GetAllLabels())
		rows = append(rows, []string{
			app.GetName(), app.GetDescription(), app.GetURI(), app.GetPublicAddr(), labels, app.GetVersion()})
	}
	headers := []string{"Name", "Description", "URI", "Public Address", "Labels", "Version"}
	var t asciitable.Table
	if c.verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type authPrefCollection struct {
	authPref types.AuthPreference
}

func (c *authPrefCollection) resources() (r []types.Resource) {
	return []types.Resource{c.authPref}
}

func (c *authPrefCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Type", "Second Factor"})
	t.AddRow([]string{c.authPref.GetType(), string(c.authPref.GetSecondFactor())})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type netConfigCollection struct {
	netConfig types.ClusterNetworkingConfig
}

func (c *netConfigCollection) resources() (r []types.Resource) {
	return []types.Resource{c.netConfig}
}

func (c *netConfigCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Client Idle Timeout", "Keep Alive Interval", "Keep Alive Count Max", "Session Control Timeout"})
	t.AddRow([]string{
		c.netConfig.GetClientIdleTimeout().String(),
		c.netConfig.GetKeepAliveInterval().String(),
		strconv.FormatInt(c.netConfig.GetKeepAliveCountMax(), 10),
		c.netConfig.GetSessionControlTimeout().String(),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type recConfigCollection struct {
	recConfig types.SessionRecordingConfig
}

func (c *recConfigCollection) resources() (r []types.Resource) {
	return []types.Resource{c.recConfig}
}

func (c *recConfigCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Mode", "Proxy Checks Host Keys"})
	t.AddRow([]string{c.recConfig.GetMode(), strconv.FormatBool(c.recConfig.GetProxyChecksHostKeys())})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type netRestrictionsCollection struct {
	netRestricts types.NetworkRestrictions
}

type writer struct {
	w   io.Writer
	err error
}

func (w *writer) write(s string) {
	if w.err == nil {
		_, w.err = w.w.Write([]byte(s))
	}
}

func (c *netRestrictionsCollection) resources() (r []types.Resource) {
	r = append(r, c.netRestricts)
	return
}

func (c *netRestrictionsCollection) writeList(as []types.AddressCondition, w *writer) {
	for _, a := range as {
		w.write(a.CIDR)
		w.write("\n")
	}
}

func (c *netRestrictionsCollection) writeText(w io.Writer) error {
	out := &writer{w: w}
	out.write("ALLOW\n")
	c.writeList(c.netRestricts.GetAllow(), out)

	out.write("\nDENY\n")
	c.writeList(c.netRestricts.GetDeny(), out)
	return trace.Wrap(out.err)
}

type databaseServerCollection struct {
	servers []types.DatabaseServer
	verbose bool
}

func (c *databaseServerCollection) resources() (r []types.Resource) {
	for _, resource := range c.servers {
		r = append(r, resource)
	}
	return r
}

func (c *databaseServerCollection) writeText(w io.Writer) error {
	var rows [][]string
	for _, server := range c.servers {
		labels := stripInternalTeleportLabels(c.verbose, server.GetDatabase().GetAllLabels())
		rows = append(rows, []string{
			server.GetHostname(),
			server.GetDatabase().GetName(),
			server.GetDatabase().GetProtocol(),
			server.GetDatabase().GetURI(),
			labels,
			server.GetTeleportVersion(),
		})
	}
	headers := []string{"Host", "Name", "Protocol", "URI", "Labels", "Version"}
	var t asciitable.Table
	if c.verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (c *databaseServerCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(c.toMarshal(), "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

func (c *databaseServerCollection) toMarshal() interface{} {
	return c.servers
}

func (c *databaseServerCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.toMarshal())
}

type databaseCollection struct {
	databases []types.Database
	verbose   bool
}

func (c *databaseCollection) resources() (r []types.Resource) {
	for _, resource := range c.databases {
		r = append(r, resource)
	}
	return r
}

func (c *databaseCollection) writeText(w io.Writer) error {
	var rows [][]string
	for _, database := range c.databases {
		labels := stripInternalTeleportLabels(c.verbose, database.GetAllLabels())
		rows = append(rows, []string{
			database.GetName(), database.GetProtocol(), database.GetURI(), labels,
		})
	}
	headers := []string{"Name", "Protocol", "URI", "Labels"}
	var t asciitable.Table
	if c.verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type lockCollection struct {
	locks []types.Lock
}

func (c *lockCollection) resources() (r []types.Resource) {
	for _, resource := range c.locks {
		r = append(r, resource)
	}
	return r
}

func (c *lockCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"ID", "Target", "Message", "Expires"})
	for _, lock := range c.locks {
		target := lock.Target()
		expires := "never"
		if lock.LockExpiry() != nil {
			expires = apiutils.HumanTimeFormat(*lock.LockExpiry())
		}
		t.AddRow([]string{lock.GetName(), target.String(), lock.Message(), expires})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type windowsDesktopServiceCollection struct {
	services []types.WindowsDesktopService
}

func (c *windowsDesktopServiceCollection) resources() (r []types.Resource) {
	for _, resource := range c.services {
		r = append(r, resource)
	}
	return r
}

func (c *windowsDesktopServiceCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"Name", "Address", "Version"})
	for _, service := range c.services {
		addr := service.GetAddr()
		if addr == reversetunnel.LocalWindowsDesktop {
			addr = "<proxy tunnel>"
		}
		t.AddRow([]string{service.GetName(), addr, service.GetTeleportVersion()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type windowsDesktopCollection struct {
	desktops []types.WindowsDesktop
}

func (c *windowsDesktopCollection) resources() (r []types.Resource) {
	for _, resource := range c.desktops {
		r = append(r, resource)
	}
	return r
}

func (c *windowsDesktopCollection) writeText(w io.Writer) error {
	t := asciitable.MakeTable([]string{"UUID", "Address"})
	for _, desktop := range c.desktops {
		t.AddRow([]string{desktop.GetName(), desktop.GetAddr()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type windowsDesktopAndService struct {
	desktop types.WindowsDesktop
	service types.WindowsDesktopService
}

type windowsDesktopAndServiceCollection struct {
	desktops []windowsDesktopAndService
	verbose  bool
}

func stripInternalTeleportLabels(verbose bool, labels map[string]string) string {
	if verbose { // remove teleport.dev labels unless we're in verbose mode.
		return types.LabelsAsString(labels, nil)
	}
	for key := range labels {
		if strings.HasPrefix(key, types.TeleportNamespace+"/") {
			delete(labels, key)
		}
	}
	return types.LabelsAsString(labels, nil)
}

func (c *windowsDesktopAndServiceCollection) writeText(w io.Writer) error {
	var rows [][]string
	for _, d := range c.desktops {
		labels := stripInternalTeleportLabels(c.verbose, d.desktop.GetAllLabels())
		rows = append(rows, []string{d.service.GetHostname(), d.desktop.GetAddr(),
			d.desktop.GetDomain(), labels, d.service.GetTeleportVersion()})
	}
	headers := []string{"Host", "Address", "AD Domain", "Labels", "Version"}
	var t asciitable.Table
	if c.verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (c *windowsDesktopAndServiceCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.desktops)
}

func (c *windowsDesktopAndServiceCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(c.desktops, "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}

type tokenCollection struct {
	tokens []types.ProvisionToken
}

func (c *tokenCollection) resources() (r []types.Resource) {
	for _, resource := range c.tokens {
		r = append(r, resource)
	}
	return r
}

func (c *tokenCollection) writeText(w io.Writer) error {
	for _, token := range c.tokens {
		_, err := w.Write([]byte(token.String()))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

type kubeServerCollection struct {
	servers []types.Server
	verbose bool
}

func (c *kubeServerCollection) writeText(w io.Writer) error {
	var rows [][]string
	for _, server := range c.servers {
		kubes := server.GetKubernetesClusters()
		for _, kube := range kubes {
			labels := stripInternalTeleportLabels(c.verbose,
				types.CombineLabels(kube.StaticLabels, kube.DynamicLabels))
			rows = append(rows, []string{
				kube.Name,
				labels,
				server.GetTeleportVersion(),
			})
		}
	}
	headers := []string{"Cluster", "Labels", "Version"}
	var t asciitable.Table
	if c.verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (c *kubeServerCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.servers)
}

func (c *kubeServerCollection) writeJSON(w io.Writer) error {
	data, err := json.MarshalIndent(c.servers, "", "    ")
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}
