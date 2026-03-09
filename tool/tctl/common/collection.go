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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/api/types/label"
	"github.com/gravitational/teleport/api/types/secreports"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
	"github.com/gravitational/teleport/tool/tctl/common/loginrule"
	"github.com/gravitational/teleport/tool/tctl/common/oktaassignment"
	"github.com/gravitational/teleport/tool/tctl/common/resources"
)

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
		settingsMsteams                   = "msteams"
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
		case settingsMsteams:
			p.PluginV1.Spec.Settings = &types.PluginSpecV1_Msteams{}

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
