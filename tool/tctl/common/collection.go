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

package common

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	accessmonitoringrulesv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/api/types/secreports"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
	clusterconfigrec "github.com/gravitational/teleport/tool/tctl/common/clusterconfig"
	"github.com/gravitational/teleport/tool/tctl/common/databaseobject"
	"github.com/gravitational/teleport/tool/tctl/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/tool/tctl/common/loginrule"
	"github.com/gravitational/teleport/tool/tctl/common/oktaassignment"
	"github.com/gravitational/teleport/tool/tctl/common/resources"
)

// namedResourceCollection is an implementation of [resources.Collection] that
// displays resources in a table as a list of names and nothing else.
type namedResourceCollection []types.Resource

// Resources implements [resources.Collection].
func (c namedResourceCollection) Resources() []types.Resource {
	return c
}

// WriteText implements [resources.Collection].
func (c namedResourceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, override := range c {
		t.AddRow([]string{override.GetName()})
	}
	return trace.Wrap(t.WriteTo(w))
}

type namespaceCollection struct {
	namespaces []types.Namespace
}

func (n *namespaceCollection) Resources() (r []types.Resource) {
	for i := range n.namespaces {
		r = append(r, &n.namespaces[i])
	}
	return r
}

func (n *namespaceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, n := range n.namespaces {
		t.AddRow([]string{n.Metadata.Name})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func printMetadataLabels(labels map[string]string) string {
	pairs := []string{}
	for key, value := range labels {
		pairs = append(pairs, fmt.Sprintf("%v=%v", key, value))
	}
	return strings.Join(pairs, ",")
}

type authorityCollection struct {
	cas []types.CertAuthority
}

func (a *authorityCollection) Resources() (r []types.Resource) {
	for _, resource := range a.cas {
		r = append(r, resource)
	}
	return r
}

func (a *authorityCollection) WriteText(w io.Writer, verbose bool) error {
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

func (r *reverseTunnelCollection) Resources() (res []types.Resource) {
	for _, resource := range r.tunnels {
		res = append(res, resource)
	}
	return res
}

func (r *reverseTunnelCollection) WriteText(w io.Writer, verbose bool) error {
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

func (c *oidcCollection) Resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
}

func (c *oidcCollection) WriteText(w io.Writer, verbose bool) error {
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

func (c *samlCollection) Resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
}

func (c *samlCollection) WriteText(w io.Writer, verbose bool) error {
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

func (c *connectorsCollection) Resources() (r []types.Resource) {
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

func (c *connectorsCollection) WriteText(w io.Writer, verbose bool) error {
	if len(c.oidc) > 0 {
		_, err := io.WriteString(w, "\nOIDC:\n")
		if err != nil {
			return trace.Wrap(err)
		}
		oc := &oidcCollection{connectors: c.oidc}
		err = oc.WriteText(w, verbose)
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
		err = sc.WriteText(w, verbose)
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
		err = gc.WriteText(w, verbose)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

type trustedClusterCollection struct {
	trustedClusters []types.TrustedCluster
}

func (c *trustedClusterCollection) Resources() (r []types.Resource) {
	for _, resource := range c.trustedClusters {
		r = append(r, resource)
	}
	return r
}

func (c *trustedClusterCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{
		"Name", "Enabled", "Token", "Proxy Address", "Reverse Tunnel Address", "Role Map",
	})
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

func (c *githubCollection) Resources() (r []types.Resource) {
	for _, resource := range c.connectors {
		r = append(r, resource)
	}
	return r
}

func (c *githubCollection) WriteText(w io.Writer, verbose bool) error {
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

func (c *remoteClusterCollection) Resources() (r []types.Resource) {
	for _, resource := range c.remoteClusters {
		r = append(r, resource)
	}
	return r
}

func (c *remoteClusterCollection) WriteText(w io.Writer, verbose bool) error {
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

func writeJSON(c resources.Collection, w io.Writer) error {
	return utils.WriteJSONArray(w, c.Resources())
}

func writeYAML(c resources.Collection, w io.Writer) error {
	return utils.WriteYAML(w, c.Resources())
}

type semaphoreCollection struct {
	sems []types.Semaphore
}

func (c *semaphoreCollection) Resources() (r []types.Resource) {
	for _, resource := range c.sems {
		r = append(r, resource)
	}
	return r
}

func (c *semaphoreCollection) WriteText(w io.Writer, verbose bool) error {
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

type authPrefCollection struct {
	authPref types.AuthPreference
}

func (c *authPrefCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.authPref}
}

func (c *authPrefCollection) WriteText(w io.Writer, verbose bool) error {
	var secondFactorStrings []string
	for _, sf := range c.authPref.GetSecondFactors() {
		sfString, err := sf.Encode()
		if err != nil {
			return trace.Wrap(err)
		}
		secondFactorStrings = append(secondFactorStrings, sfString)
	}

	t := asciitable.MakeTable([]string{"Type", "Second Factors"})
	t.AddRow([]string{c.authPref.GetType(), strings.Join(secondFactorStrings, ", ")})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type uiConfigCollection struct {
	uiconfig types.UIConfig
}

func (c *uiConfigCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.uiconfig}
}

func (c *uiConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Scrollback Lines", "Show Resources"})
	t.AddRow([]string{strconv.FormatInt(int64(c.uiconfig.GetScrollbackLines()), 10), string(c.uiconfig.GetShowResources())})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type netConfigCollection struct {
	netConfig types.ClusterNetworkingConfig
}

func (c *netConfigCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.netConfig}
}

func (c *netConfigCollection) WriteText(w io.Writer, verbose bool) error {
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

type maintenanceWindowCollection struct {
	cmc types.ClusterMaintenanceConfig
}

func (c *maintenanceWindowCollection) Resources() (r []types.Resource) {
	if c.cmc == nil {
		return nil
	}
	return []types.Resource{c.cmc}
}

func (c *maintenanceWindowCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Type", "Params"})

	agentUpgradeParams := "none"

	if c.cmc != nil {
		if win, ok := c.cmc.GetAgentUpgradeWindow(); ok {
			agentUpgradeParams = fmt.Sprintf("utc_start_hour=%d", win.UTCStartHour)
			if len(win.Weekdays) != 0 {
				agentUpgradeParams = fmt.Sprintf("%s, weekdays=%s", agentUpgradeParams, strings.Join(win.Weekdays, ","))
			}
		}
	}

	t.AddRow([]string{"Agent Upgrades", agentUpgradeParams})

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type recConfigCollection struct {
	recConfig types.SessionRecordingConfig
}

func (c *recConfigCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.recConfig}
}

func (c *recConfigCollection) WriteText(w io.Writer, verbose bool) error {
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

func (c *netRestrictionsCollection) Resources() (r []types.Resource) {
	r = append(r, c.netRestricts)
	return
}

func (c *netRestrictionsCollection) writeList(as []types.AddressCondition, w *writer) {
	for _, a := range as {
		w.write(a.CIDR)
		w.write("\n")
	}
}

func (c *netRestrictionsCollection) WriteText(w io.Writer, verbose bool) error {
	out := &writer{w: w}
	out.write("ALLOW\n")
	c.writeList(c.netRestricts.GetAllow(), out)

	out.write("\nDENY\n")
	c.writeList(c.netRestricts.GetDeny(), out)
	return trace.Wrap(out.err)
}

type databaseServerCollection struct {
	servers []types.DatabaseServer
}

func (c *databaseServerCollection) Resources() (r []types.Resource) {
	for _, resource := range c.servers {
		r = append(r, resource)
	}
	return r
}

func (c *databaseServerCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, server := range c.servers {
		labels := common.FormatLabels(server.GetDatabase().GetAllLabels(), verbose)
		rows = append(rows, []string{
			server.GetHostname(),
			common.FormatResourceName(server.GetDatabase(), verbose),
			server.GetDatabase().GetProtocol(),
			server.GetDatabase().GetURI(),
			labels,
			server.GetTeleportVersion(),
		})
	}
	headers := []string{"Host", "Name", "Protocol", "URI", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by hostname then by name.
	t.SortRowsBy([]int{0, 1}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (c *databaseServerCollection) writeJSON(w io.Writer) error {
	return utils.WriteJSONArray(w, c.servers)
}

func (c *databaseServerCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.servers)
}

type windowsDesktopServiceCollection struct {
	services []types.WindowsDesktopService
}

func (c *windowsDesktopServiceCollection) Resources() (r []types.Resource) {
	for _, resource := range c.services {
		r = append(r, resource)
	}
	return r
}

func (c *windowsDesktopServiceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Address", "Version"})
	for _, service := range c.services {
		addr := service.GetAddr()
		if addr == reversetunnelclient.LocalWindowsDesktop {
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

func (c *windowsDesktopCollection) Resources() (r []types.Resource) {
	for _, resource := range c.desktops {
		r = append(r, resource)
	}
	return r
}

func (c *windowsDesktopCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, d := range c.desktops {
		labels := common.FormatLabels(d.GetAllLabels(), verbose)
		rows = append(rows, []string{d.GetName(), d.GetAddr(), d.GetDomain(), labels})
	}
	headers := []string{"Name", "Address", "AD Domain", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (c *windowsDesktopCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.desktops)
}

func (c *windowsDesktopCollection) writeJSON(w io.Writer) error {
	return utils.WriteJSONArray(w, c.desktops)
}

type dynamicWindowsDesktopCollection struct {
	desktops []types.DynamicWindowsDesktop
}

func (c *dynamicWindowsDesktopCollection) Resources() (r []types.Resource) {
	r = make([]types.Resource, 0, len(c.desktops))
	for _, resource := range c.desktops {
		r = append(r, resource)
	}
	return r
}

func (c *dynamicWindowsDesktopCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, d := range c.desktops {
		labels := common.FormatLabels(d.GetAllLabels(), verbose)
		rows = append(rows, []string{d.GetName(), d.GetAddr(), d.GetDomain(), labels})
	}
	headers := []string{"Name", "Address", "AD Domain", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type tokenCollection struct {
	tokens []types.ProvisionToken
}

func (c *tokenCollection) Resources() (r []types.Resource) {
	for _, resource := range c.tokens {
		r = append(r, resource)
	}
	return r
}

func (c *tokenCollection) WriteText(w io.Writer, verbose bool) error {
	for _, token := range c.tokens {
		_, err := w.Write([]byte(token.String()))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

type kubeServerCollection struct {
	servers []types.KubeServer
}

func (c *kubeServerCollection) Resources() (r []types.Resource) {
	for _, resource := range c.servers {
		r = append(r, resource)
	}
	return r
}

func (c *kubeServerCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, server := range c.servers {
		kube := server.GetCluster()
		if kube == nil {
			continue
		}
		labels := common.FormatLabels(kube.GetAllLabels(), verbose)
		rows = append(rows, []string{
			common.FormatResourceName(kube, verbose),
			labels,
			server.GetTeleportVersion(),
		})

	}
	headers := []string{"Cluster", "Labels", "Version"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by cluster name.
	t.SortRowsBy([]int{0}, true)

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (c *kubeServerCollection) writeYAML(w io.Writer) error {
	return utils.WriteYAML(w, c.servers)
}

func (c *kubeServerCollection) writeJSON(w io.Writer) error {
	return utils.WriteJSONArray(w, c.servers)
}

type crownJewelCollection struct {
	items []*crownjewelv1.CrownJewel
}

func (c *crownJewelCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

// writeText formats the crown jewels into a table and writes them into w.
// If verbose is disabled, labels column can be truncated to fit into the console.
func (c *crownJewelCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {
		labels := common.FormatLabels(item.GetMetadata().GetLabels(), verbose)
		rows = append(rows, []string{item.Metadata.GetName(), item.GetSpec().String(), labels})
	}
	headers := []string{"Name", "Spec", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type kubeClusterCollection struct {
	clusters []types.KubeCluster
}

func (c *kubeClusterCollection) Resources() (r []types.Resource) {
	for _, resource := range c.clusters {
		r = append(r, resource)
	}
	return r
}

// writeText formats the dynamic kube clusters into a table and writes them into w.
// Name          Labels
// ------------- ----------------------------------------------------------------------------------------------------------
// cluster1      region=eastus,resource-group=cluster1,subscription-id=subID
// cluster2      region=westeurope,resource-group=cluster2,subscription-id=subID
// cluster3      region=northcentralus,resource-group=cluster3,subscription-id=subID
// cluster4      owner=cluster4,region=southcentralus,resource-group=cluster4,subscription-id=subID
// If verbose is disabled, labels column can be truncated to fit into the console.
func (c *kubeClusterCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, cluster := range c.clusters {
		labels := common.FormatLabels(cluster.GetAllLabels(), verbose)
		rows = append(rows, []string{
			common.FormatResourceName(cluster, verbose),
			labels,
		})
	}
	headers := []string{"Name", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type installerCollection struct {
	installers []types.Installer
}

func (c *installerCollection) Resources() []types.Resource {
	var r []types.Resource
	for _, inst := range c.installers {
		r = append(r, inst)
	}
	return r
}

func (c *installerCollection) WriteText(w io.Writer, verbose bool) error {
	for _, inst := range c.installers {
		if _, err := fmt.Fprintf(w, "Script: %s\n----------\n", inst.GetName()); err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(w, inst.GetScript()); err != nil {
			return trace.Wrap(err)
		}
		if _, err := fmt.Fprintln(w, "----------"); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

type integrationCollection struct {
	integrations []types.Integration
}

func (c *integrationCollection) Resources() (r []types.Resource) {
	for _, ig := range c.integrations {
		r = append(r, ig)
	}
	return r
}

func (c *integrationCollection) WriteText(w io.Writer, verbose bool) error {
	sort.Sort(types.Integrations(c.integrations))
	var rows [][]string
	for _, ig := range c.integrations {
		specProps := []string{}
		switch ig.GetSubKind() {
		case types.IntegrationSubKindAWSOIDC:
			specProps = append(specProps, fmt.Sprintf("RoleARN=%s", ig.GetAWSOIDCIntegrationSpec().RoleARN))
		}

		rows = append(rows, []string{
			ig.GetName(), ig.GetSubKind(), strings.Join(specProps, ","),
		})
	}
	headers := []string{"Name", "Type", "Spec"}
	t := asciitable.MakeTable(headers, rows...)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type externalAuditStorageCollection struct {
	externalAuditStorages []*externalauditstorage.ExternalAuditStorage
}

func (c *externalAuditStorageCollection) Resources() (r []types.Resource) {
	for _, a := range c.externalAuditStorages {
		r = append(r, a)
	}
	return r
}

func (c *externalAuditStorageCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, a := range c.externalAuditStorages {
		rows = append(rows, []string{
			a.GetName(),
			a.Spec.IntegrationName,
			a.Spec.PolicyName,
			a.Spec.Region,
			a.Spec.SessionRecordingsURI,
			a.Spec.AuditEventsLongTermURI,
			a.Spec.AthenaResultsURI,
			a.Spec.AthenaWorkgroup,
			a.Spec.GlueDatabase,
			a.Spec.GlueTable,
		})
	}
	headers := []string{"Name", "IntegrationName", "PolicyName", "Region", "SessionRecordingsURI", "AuditEventsLongTermURI", "AthenaResultsURI", "AthenaWorkgroup", "GlueDatabase", "GlueTable"}
	t := asciitable.MakeTable(headers, rows...)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type databaseServiceCollection struct {
	databaseServices []types.DatabaseService
}

func (c *databaseServiceCollection) Resources() (r []types.Resource) {
	for _, service := range c.databaseServices {
		r = append(r, service)
	}
	return r
}

func databaseResourceMatchersToString(in []*types.DatabaseResourceMatcher) string {
	resourceMatchersStrings := make([]string, 0, len(in))

	for _, resMatcher := range in {
		if resMatcher == nil || resMatcher.Labels == nil {
			continue
		}

		labelsString := make([]string, 0, len(*resMatcher.Labels))
		for key, values := range *resMatcher.Labels {
			if key == types.Wildcard {
				labelsString = append(labelsString, "<all databases>")
				continue
			}
			labelsString = append(labelsString, fmt.Sprintf("%v=%v", key, values))
		}

		resourceMatchersStrings = append(resourceMatchersStrings, fmt.Sprintf("(Labels: %s)", strings.Join(labelsString, ",")))
	}
	return strings.Join(resourceMatchersStrings, ",")
}

// writeText formats the DatabaseServices into a table and writes them into w.
// Example:
//
// Name                                 Resource Matchers
// ------------------------------------ --------------------------------------
// a6065ee9-d5ee-4555-8d47-94a78625277b (Labels: <all databases>)
// d4e13f2b-0a55-4e0a-b363-bacfb1a11294 (Labels: env=[prod],aws-tag=[xyz abc])
func (c *databaseServiceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Resource Matchers"})

	for _, dbService := range c.databaseServices {
		t.AddRow([]string{
			dbService.GetName(), databaseResourceMatchersToString(dbService.GetResourceMatchers()),
		})
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type loginRuleCollection struct {
	rules []*loginrulepb.LoginRule
}

func (l *loginRuleCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Priority"})
	for _, rule := range l.rules {
		t.AddRow([]string{rule.Metadata.Name, strconv.FormatInt(int64(rule.Priority), 10)})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (l *loginRuleCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(l.rules))
	for i, rule := range l.rules {
		resources[i] = loginrule.ProtoToResource(rule)
	}
	return resources
}

//nolint:revive // Because we want this to be IdP.
type samlIdPServiceProviderCollection struct {
	serviceProviders []types.SAMLIdPServiceProvider
}

func (c *samlIdPServiceProviderCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.serviceProviders))
	for i, resource := range c.serviceProviders {
		r[i] = resource
	}
	return r
}

func (c *samlIdPServiceProviderCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, serviceProvider := range c.serviceProviders {
		t.AddRow([]string{serviceProvider.GetName()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type databaseObjectImportRuleCollection struct {
	rules []*dbobjectimportrulev1.DatabaseObjectImportRule
}

func (c *databaseObjectImportRuleCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.rules))
	for i, b := range c.rules {
		resources[i] = databaseobjectimportrule.ProtoToResource(b)
	}
	return resources
}

func (c *databaseObjectImportRuleCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Priority", "Mapping Count", "DB Label Count"})
	for _, b := range c.rules {
		t.AddRow([]string{
			b.GetMetadata().GetName(),
			fmt.Sprintf("%v", b.GetSpec().GetPriority()),
			fmt.Sprintf("%v", len(b.GetSpec().GetMappings())),
			fmt.Sprintf("%v", len(b.GetSpec().GetDatabaseLabels())),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type databaseObjectCollection struct {
	objects []*dbobjectv1.DatabaseObject
}

func (c *databaseObjectCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.objects))
	for i, b := range c.objects {
		resources[i] = databaseobject.ProtoToResource(b)
	}
	return resources
}

func (c *databaseObjectCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Kind", "DB Service", "Protocol"})
	for _, b := range c.objects {
		t.AddRow([]string{
			b.GetMetadata().GetName(),
			fmt.Sprintf("%v", b.GetSpec().GetObjectKind()),
			fmt.Sprintf("%v", b.GetSpec().GetDatabaseServiceName()),
			fmt.Sprintf("%v", b.GetSpec().GetProtocol()),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type deviceCollection struct {
	devices []*devicepb.Device
}

func (c *deviceCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.devices))
	for i, dev := range c.devices {
		resources[i] = types.DeviceToResource(dev)
	}
	return resources
}

func (c *deviceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"ID", "OS Type", "Asset Tag", "Enrollment Status", "Creation Time", "Last Updated"})
	for _, device := range c.devices {
		t.AddRow([]string{
			device.Id,
			devicetrust.FriendlyOSType(device.OsType),
			device.AssetTag,
			devicetrust.FriendlyDeviceEnrollStatus(device.EnrollStatus),
			device.CreateTime.AsTime().Format(time.RFC3339),
			device.UpdateTime.AsTime().Format(time.RFC3339),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type discoveryConfigCollection struct {
	discoveryConfigs []*discoveryconfig.DiscoveryConfig
}

func (c *discoveryConfigCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.discoveryConfigs))
	for i, dc := range c.discoveryConfigs {
		resources[i] = dc
	}
	return resources
}

func (c *discoveryConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Discovery Group"})
	for _, dc := range c.discoveryConfigs {
		t.AddRow([]string{
			dc.GetName(),
			dc.GetDiscoveryGroup(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type oktaImportRuleCollection struct {
	importRules []types.OktaImportRule
}

func (c *oktaImportRuleCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.importRules))
	for i, resource := range c.importRules {
		r[i] = resource
	}
	return r
}

func (c *oktaImportRuleCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, importRule := range c.importRules {
		t.AddRow([]string{importRule.GetName()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type oktaAssignmentCollection struct {
	assignments []types.OktaAssignment
}

func (c *oktaAssignmentCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.assignments))
	for i, resource := range c.assignments {
		r[i] = oktaassignment.ToResource(resource)
	}
	return r
}

func (c *oktaAssignmentCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, assignment := range c.assignments {
		t.AddRow([]string{assignment.GetName()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type userGroupCollection struct {
	userGroups []types.UserGroup
}

func (c *userGroupCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.userGroups))
	for i, resource := range c.userGroups {
		r[i] = resource
	}
	return r
}

func (c *userGroupCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Origin"})
	for _, userGroup := range c.userGroups {
		t.AddRow([]string{
			userGroup.GetName(),
			userGroup.Origin(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type auditQueryCollection struct {
	auditQueries []*secreports.AuditQuery
}

func (c *auditQueryCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.auditQueries))
	for i, resource := range c.auditQueries {
		r[i] = resource
	}
	return r
}

func (c *auditQueryCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Title", "Query", "Description"})
	for _, v := range c.auditQueries {
		t.AddRow([]string{v.GetName(), v.Spec.Title, v.Spec.Query, v.Spec.Description})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type securityReportCollection struct {
	items []*secreports.Report
}

func (c *securityReportCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.items))
	for i, resource := range c.items {
		r[i] = resource
	}
	return r
}

func (c *securityReportCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Title", "Audit Queries", "Description"})
	for _, v := range c.items {
		auditQueriesNames := make([]string, 0, len(v.Spec.AuditQueries))
		for _, k := range v.Spec.AuditQueries {
			auditQueriesNames = append(auditQueriesNames, k.Name)
		}
		t.AddRow([]string{v.GetName(), v.Spec.Title, strings.Join(auditQueriesNames, ", "), v.Spec.Description})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type serverInfoCollection struct {
	serverInfos []types.ServerInfo
}

func (c *serverInfoCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.serverInfos))
	for i, resource := range c.serverInfos {
		r[i] = resource
	}
	return r
}

func (c *serverInfoCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Labels"})
	for _, si := range c.serverInfos {
		t.AddRow([]string{si.GetName(), printMetadataLabels(si.GetNewLabels())})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type accessListCollection struct {
	accessLists []*accesslist.AccessList
}

func (c *accessListCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.accessLists))
	for i, resource := range c.accessLists {
		r[i] = resource
	}
	return r
}

func (c *accessListCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Title", "Review Frequency", "Next Audit Date"})
	for _, al := range c.accessLists {
		t.AddRow([]string{
			al.GetName(),
			al.Spec.Title,
			al.Spec.Audit.Recurrence.Frequency.String(),
			al.Spec.Audit.NextAuditDate.Format(time.RFC822),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type vnetConfigCollection struct {
	vnetConfig *vnet.VnetConfig
}

func (c *vnetConfigCollection) Resources() []types.Resource {
	return []types.Resource{types.Resource153ToLegacy(c.vnetConfig)}
}

func (c *vnetConfigCollection) WriteText(w io.Writer, verbose bool) error {
	var dnsZoneSuffixes []string
	for _, dnsZone := range c.vnetConfig.Spec.CustomDnsZones {
		dnsZoneSuffixes = append(dnsZoneSuffixes, dnsZone.Suffix)
	}
	t := asciitable.MakeTable([]string{"IPv4 CIDR range", "Custom DNS Zones"})
	t.AddRow([]string{
		c.vnetConfig.GetSpec().GetIpv4CidrRange(),
		strings.Join(dnsZoneSuffixes, ", "),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type accessGraphSettings struct {
	accessGraphSettings *clusterconfigrec.AccessGraphSettings
}

func (c *accessGraphSettings) Resources() []types.Resource {
	return []types.Resource{c.accessGraphSettings}
}

func (c *accessGraphSettings) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"SSH Keys Scan"})
	t.AddRow([]string{
		c.accessGraphSettings.Spec.SecretsScanConfig,
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type accessRequestCollection struct {
	accessRequests []types.AccessRequest
}

func (c *accessRequestCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.accessRequests))
	for i, resource := range c.accessRequests {
		r[i] = resource
	}
	return r
}

func (c *accessRequestCollection) WriteText(w io.Writer, verbose bool) error {
	var t asciitable.Table
	var rows [][]string
	for _, al := range c.accessRequests {
		var annotations []string
		for k, v := range al.GetSystemAnnotations() {
			annotations = append(annotations, fmt.Sprintf("%s/%s", k, strings.Join(v, ",")))
		}
		rows = append(rows, []string{
			al.GetName(),
			al.GetUser(),
			strings.Join(al.GetRoles(), ", "),
			strings.Join(annotations, ", "),
		})
	}
	if verbose {
		t = asciitable.MakeTable([]string{"Name", "User", "Roles", "Annotations"}, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn([]string{"Name", "User", "Roles", "Annotations"}, rows, "Annotations")
	}

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type pluginCollection struct {
	plugins []types.Plugin
}

// pluginResourceWrapper provides custom JSON unmarshaling for Plugin resource
// types. The Plugin resource uses structures generated from a protobuf `oneof`
// directive, which the stdlib JSON unmarshaller can't handle, so we use this
// custom wrapper to help.
type pluginResourceWrapper struct {
	types.PluginV1
}

func (p *pluginResourceWrapper) UnmarshalJSON(data []byte) error {
	// If your plugin contains a `oneof` message, implement custom UnmarshalJSON/MarshalJSON
	// using gogo/jsonpb for the type.
	const (
		credOauth2AccessToken             = "oauth2_access_token"
		credBearerToken                   = "bearer_token"
		credIdSecret                      = "id_secret"
		credStaticCredentialsRef          = "static_credentials_ref"
		settingsSlackAccessPlugin         = "slack_access_plugin"
		settingsOpsgenie                  = "opsgenie"
		settingsOpenAI                    = "openai"
		settingsOkta                      = "okta"
		settingsJamf                      = "jamf"
		settingsIntune                    = "intune"
		settingsPagerDuty                 = "pager_duty"
		settingsMattermost                = "mattermost"
		settingsJira                      = "jira"
		settingsDiscord                   = "discord"
		settingsServiceNow                = "serviceNow"
		settingsGitlab                    = "gitlab"
		settingsEntraID                   = "entra_id"
		settingsDatadogIncidentManagement = "datadog_incident_management"
		settingsEmailAccessPlugin         = "email_access_plugin"
		settingsAWSIdentityCenter         = "aws_ic"
		settingsNetIQ                     = "net_iq"
	)
	type unknownPluginType struct {
		Spec struct {
			Settings map[string]json.RawMessage `json:"Settings"`
		} `json:"spec"`
		Status struct {
			Details map[string]json.RawMessage `json:"Details"`
		} `json:"status"`
		Credentials struct {
			Credentials map[string]json.RawMessage `json:"Credentials"`
		} `json:"credentials"`
	}

	var unknownPlugin unknownPluginType
	if err := json.Unmarshal(data, &unknownPlugin); err != nil {
		return err
	}

	if unknownPlugin.Spec.Settings == nil {
		return trace.BadParameter("plugin settings are missing")
	}
	if len(unknownPlugin.Spec.Settings) != 1 {
		return trace.BadParameter("unknown plugin settings count")
	}

	if len(unknownPlugin.Credentials.Credentials) == 1 {
		p.PluginV1.Credentials = &types.PluginCredentialsV1{}
		for k := range unknownPlugin.Credentials.Credentials {
			switch k {
			case credOauth2AccessToken:
				p.PluginV1.Credentials.Credentials = &types.PluginCredentialsV1_Oauth2AccessToken{}
			case credBearerToken:
				p.PluginV1.Credentials.Credentials = &types.PluginCredentialsV1_BearerToken{}
			case credIdSecret:
				p.PluginV1.Credentials.Credentials = &types.PluginCredentialsV1_IdSecret{}
			case credStaticCredentialsRef:
				p.PluginV1.Credentials.Credentials = &types.PluginCredentialsV1_StaticCredentialsRef{}
			default:
				return trace.BadParameter("unsupported plugin credential type: %v", k)
			}
		}
	}

	for k := range unknownPlugin.Spec.Settings {
		switch k {
		case settingsSlackAccessPlugin:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_SlackAccessPlugin{}
		case settingsOpsgenie:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Opsgenie{}
		case settingsOpenAI:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Openai{}
		case settingsOkta:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Okta{}
			p.PluginV1.Status.Details = &types.PluginStatusV1_Okta{}
		case settingsJamf:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Jamf{}
		case settingsIntune:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Intune{}
		case settingsPagerDuty:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_PagerDuty{}
		case settingsMattermost:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Mattermost{}
		case settingsJira:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Jira{}
		case settingsDiscord:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Discord{}
		case settingsServiceNow:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_ServiceNow{}
		case settingsGitlab:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Gitlab{}
			p.PluginV1.Status.Details = &types.PluginStatusV1_Gitlab{}
		case settingsEntraID:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_EntraId{}
			p.PluginV1.Status.Details = &types.PluginStatusV1_EntraId{}
		case settingsDatadogIncidentManagement:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Datadog{}
		case settingsEmailAccessPlugin:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Email{}
		case settingsAWSIdentityCenter:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_AwsIc{}
			p.PluginV1.Status.Details = &types.PluginStatusV1_AwsIc{}
		case settingsNetIQ:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_NetIq{}
			p.PluginV1.Status.Details = &types.PluginStatusV1_NetIq{}

		default:
			return trace.BadParameter("unsupported plugin type: %v", k)
		}
	}

	if err := json.Unmarshal(data, &p.PluginV1); err != nil {
		return err
	}
	return nil
}

func (c *pluginCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.plugins))
	for i, resource := range c.plugins {
		r[i] = resource
	}
	return r
}

func (c *pluginCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Status"})
	for _, plugin := range c.plugins {
		t.AddRow([]string{
			plugin.GetName(),
			plugin.GetStatus().GetCode().String(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type staticHostUserCollection struct {
	items []*userprovisioningpb.StaticHostUser
}

func (c *staticHostUserCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

func (c *staticHostUserCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {

		for _, matcher := range item.Spec.Matchers {
			labelMap := label.ToMap(matcher.NodeLabels)
			labelStringMap := make(map[string]string, len(labelMap))
			for k, vals := range labelMap {
				labelStringMap[k] = fmt.Sprintf("[%s]", printSortedStringSlice(vals))
			}
			var uid string
			if matcher.Uid != 0 {
				uid = strconv.Itoa(int(matcher.Uid))
			}
			var gid string
			if matcher.Gid != 0 {
				gid = strconv.Itoa(int(matcher.Gid))
			}
			rows = append(rows, []string{
				item.GetMetadata().Name,
				common.FormatLabels(labelStringMap, verbose),
				matcher.NodeLabelsExpression,
				printSortedStringSlice(matcher.Groups),
				uid,
				gid,
			})
		}
	}
	headers := []string{"Login", "Node Labels", "Node Expression", "Groups", "Uid", "Gid"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Node Expression")
	}
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func printSortedStringSlice(s []string) string {
	s = slices.Clone(s)
	slices.Sort(s)
	return strings.Join(s, ",")
}

type userTaskCollection struct {
	items []*usertasksv1.UserTask
}

func (c *userTaskCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

// writeText formats the user tasks into a table and writes them into w.
// If verbose is disabled, labels column can be truncated to fit into the console.
func (c *userTaskCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {
		labels := common.FormatLabels(item.GetMetadata().GetLabels(), verbose)
		rows = append(rows, []string{item.Metadata.GetName(), labels, item.Spec.TaskType, item.Spec.IssueType, item.Spec.GetIntegration()})
	}
	headers := []string{"Name", "Labels", "TaskType", "IssueType", "Integration"}
	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type autoUpdateConfigCollection struct {
	config *autoupdatev1pb.AutoUpdateConfig
}

func (c *autoUpdateConfigCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.config)}
}

func (c *autoUpdateConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Tools AutoUpdate Enabled"})
	t.AddRow([]string{
		c.config.GetMetadata().GetName(),
		fmt.Sprintf("%v", c.config.GetSpec().GetTools().GetMode()),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type autoUpdateVersionCollection struct {
	version *autoupdatev1pb.AutoUpdateVersion
}

func (c *autoUpdateVersionCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.version)}
}

func (c *autoUpdateVersionCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Tools AutoUpdate Version"})
	t.AddRow([]string{
		c.version.GetMetadata().GetName(),
		fmt.Sprintf("%v", c.version.GetSpec().GetTools().TargetVersion),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type autoUpdateAgentRolloutCollection struct {
	rollout *autoupdatev1pb.AutoUpdateAgentRollout
}

func (c *autoUpdateAgentRolloutCollection) Resources() []types.Resource {
	return []types.Resource{types.ProtoResource153ToLegacy(c.rollout)}
}

func (c *autoUpdateAgentRolloutCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Start Version", "Target Version", "Mode", "Schedule", "Strategy"})
	t.AddRow([]string{
		c.rollout.GetMetadata().GetName(),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetStartVersion()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetTargetVersion()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetAutoupdateMode()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetSchedule()),
		fmt.Sprintf("%v", c.rollout.GetSpec().GetStrategy()),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type autoUpdateAgentReportCollection struct {
	reports []*autoupdatev1pb.AutoUpdateAgentReport
}

func (c *autoUpdateAgentReportCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.reports))
	for i, report := range c.reports {
		resources[i] = types.ProtoResource153ToLegacy(report)
	}
	return resources
}

func (c *autoUpdateAgentReportCollection) WriteText(w io.Writer, verbose bool) error {
	groupSet := make(map[string]any)
	versionsSet := make(map[string]any)
	for _, report := range c.reports {
		for groupName, group := range report.GetSpec().GetGroups() {
			groupSet[groupName] = struct{}{}
			for versionName := range group.GetVersions() {
				versionsSet[versionName] = struct{}{}
			}
		}
	}

	groupNames := slices.Collect(maps.Keys(groupSet))
	versionNames := slices.Collect(maps.Keys(versionsSet))
	slices.Sort(groupNames)
	slices.Sort(versionNames)

	t := asciitable.MakeTable(append([]string{"Auth Server ID", "Agent Version"}, groupNames...))
	for _, report := range c.reports {
		for i, versionName := range versionNames {
			row := make([]string, len(groupNames)+2)
			if i == 0 {
				row[0] = report.GetMetadata().GetName()
			}
			row[1] = versionName
			for j, groupName := range groupNames {
				row[j+2] = strconv.Itoa(int(report.GetSpec().GetGroups()[groupName].GetVersions()[versionName].GetCount()))
			}
			t.AddRow(row)
		}
		t.AddRow(make([]string, len(versionNames)+2))
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type accessMonitoringRuleCollection struct {
	items []*accessmonitoringrulesv1pb.AccessMonitoringRule
}

func (c *accessMonitoringRuleCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

// writeText formats the user tasks into a table and writes them into w.
// If verbose is disabled, labels column can be truncated to fit into the console.
func (c *accessMonitoringRuleCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {
		labels := common.FormatLabels(item.GetMetadata().GetLabels(), verbose)
		rows = append(rows, []string{item.Metadata.GetName(), labels})
	}
	headers := []string{"Name", "Labels"}
	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type healthCheckConfigCollection struct {
	items []*healthcheckconfigv1.HealthCheckConfig
}

func (c *healthCheckConfigCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c.items))
	for _, item := range c.items {
		out = append(out, types.ProtoResource153ToLegacy(item))
	}
	return out
}

func (c *healthCheckConfigCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Interval", "Timeout", "Healthy Threshold", "Unhealthy Threshold", "DB Labels", "DB Expression"}
	var rows [][]string
	for _, item := range c.items {
		meta := item.GetMetadata()
		spec := item.GetSpec()
		rows = append(rows, []string{
			meta.GetName(),
			common.FormatDefault(spec.GetInterval().AsDuration(), defaults.HealthCheckInterval),
			common.FormatDefault(spec.GetTimeout().AsDuration(), defaults.HealthCheckTimeout),
			common.FormatDefault(spec.GetHealthyThreshold(), defaults.HealthCheckHealthyThreshold),
			common.FormatDefault(spec.GetUnhealthyThreshold(), defaults.HealthCheckUnhealthyThreshold),
			common.FormatMultiValueLabels(label.ToMap(spec.GetMatch().GetDbLabels()), verbose),
			spec.GetMatch().GetDbLabelsExpression(),
		})
	}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "DB Labels")
	}

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type scopedRoleCollection struct {
	items []*scopedaccessv1.ScopedRole
}

func (c *scopedRoleCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c.items))
	for _, item := range c.items {
		out = append(out, types.Resource153ToLegacy(item))
	}
	return out
}

func (c *scopedRoleCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Scope", "Name"}
	var rows [][]string
	for _, item := range c.items {
		rows = append(rows, []string{
			item.GetScope(),
			item.GetMetadata().GetName(),
		})
	}

	t := asciitable.MakeTable(headers, rows...)

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

type scopedRoleAssignmentCollection struct {
	items []*scopedaccessv1.ScopedRoleAssignment
}

func (c *scopedRoleAssignmentCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c.items))
	for _, item := range c.items {
		out = append(out, types.Resource153ToLegacy(item))
	}
	return out
}

func (c *scopedRoleAssignmentCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Scope", "Name", "Assigns"}
	var rows [][]string

	for _, item := range c.items {
		var assigns []string
		for _, subAssignment := range item.GetSpec().GetAssignments() {
			assigns = append(assigns, fmt.Sprintf("%s@%s", subAssignment.GetRole(), subAssignment.GetScope()))
		}

		rows = append(rows, []string{
			item.GetScope(),
			item.GetMetadata().GetName(),
			strings.Join(assigns, ", "),
		})
	}

	t := asciitable.MakeTable(headers, rows...)

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
