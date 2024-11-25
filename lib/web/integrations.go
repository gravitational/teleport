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

package web

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/integrations/access/msteams"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
)

// integrationsCreate creates an Integration
func (h *Handler) integrationsCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	var req *ui.Integration
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var ig *types.IntegrationV1
	var err error

	switch req.SubKind {
	case types.IntegrationSubKindAWSOIDC:
		var s3Location string
		if req.AWSOIDC.IssuerS3Bucket != "" {
			issuerS3URI := url.URL{
				Scheme: "s3",
				Host:   req.AWSOIDC.IssuerS3Bucket,
				Path:   req.AWSOIDC.IssuerS3Prefix,
			}
			s3Location = issuerS3URI.String()
		}
		metadata := types.Metadata{Name: req.Name}
		ig, err = types.NewIntegrationAWSOIDC(
			metadata,
			&types.AWSOIDCIntegrationSpecV1{
				RoleARN:     req.AWSOIDC.RoleARN,
				IssuerS3URI: s3Location,
				Audience:    req.AWSOIDC.Audience,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.BadParameter("subkind %q is not supported", req.SubKind)
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	storedIntegration, err := clt.CreateIntegration(r.Context(), ig)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("failed to create Integration (%q already exists), please use another name", req.Name)
		}
		return nil, trace.Wrap(err)
	}

	uiIg, err := ui.MakeIntegration(storedIntegration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return uiIg, nil
}

// integrationsUpdate updates the Integration based on its name
func (h *Handler) integrationsUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	var req *ui.UpdateIntegrationRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	integration, err := clt.GetIntegration(r.Context(), integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.AWSOIDC != nil {
		if integration.GetSubKind() != types.IntegrationSubKindAWSOIDC {
			return nil, trace.BadParameter("cannot update %q fields for a %q integration", types.IntegrationSubKindAWSOIDC, integration.GetSubKind())
		}

		var s3Location string
		if req.AWSOIDC.IssuerS3Bucket != "" {
			issuerS3URI := url.URL{
				Scheme: "s3",
				Host:   req.AWSOIDC.IssuerS3Bucket,
				Path:   req.AWSOIDC.IssuerS3Prefix,
			}
			s3Location = issuerS3URI.String()
		}
		integration.SetAWSOIDCIssuerS3URI(s3Location)
		integration.SetAWSOIDCRoleARN(req.AWSOIDC.RoleARN)
	}

	if _, err := clt.UpdateIntegration(r.Context(), integration); err != nil {
		return nil, trace.Wrap(err)
	}

	uiIg, err := ui.MakeIntegration(integration)
	if err != nil {
		return nil, err
	}

	return uiIg, nil
}

// integrationsDelete removes an Integration based on its name
func (h *Handler) integrationsDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	integrationName := p.ByName("name_or_subkind")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.DeleteIntegration(r.Context(), integrationName); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// integrationsGet returns an Integration based on its name
func (h *Handler) integrationsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := clt.GetIntegration(r.Context(), integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiIg, err := ui.MakeIntegration(ig)
	if err != nil {
		return nil, err
	}

	return uiIg, nil
}

// integrationDashboard returns the integration summary.
func (h *Handler) integrationDashboard(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := clt.GetIntegration(r.Context(), integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	summary, err := collectAWSOIDCAutoDiscoverStats(r.Context(), ig, clt.DiscoveryConfigClient())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return summary, nil
}

func collectAWSOIDCAutoDiscoverStats(
	ctx context.Context,
	integration types.Integration,
	clt interface {
		ListDiscoveryConfigs(ctx context.Context, pageSize int, nextToken string) ([]*discoveryconfig.DiscoveryConfig, string, error)
	},
) (ui.IntegrationWithSummary, error) {
	var ret ui.IntegrationWithSummary

	uiIg, err := ui.MakeIntegration(integration)
	if err != nil {
		return ret, err
	}
	ret.Integration = uiIg

	var nextPage string
	for {
		discoveryConfigs, nextToken, err := clt.ListDiscoveryConfigs(ctx, 0, nextPage)
		if err != nil {
			return ret, trace.Wrap(err)
		}
		for _, dc := range discoveryConfigs {
			discoveredResources, ok := dc.Status.IntegrationDiscoveredResources[integration.GetName()]
			if !ok {
				continue
			}

			if matchers := matchersWithIntegration(dc, types.AWSMatcherEC2, integration.GetName()); matchers != 0 {
				ret.AWSEC2.RulesCount = ret.AWSEC2.RulesCount + matchers
				ret.AWSEC2.DiscoverLastSync = lastSync(ret.AWSEC2.DiscoverLastSync, dc.Status.LastSyncTime)
				ret.AWSEC2.ResourcesFound = ret.AWSEC2.ResourcesFound + int(discoveredResources.AwsEc2.Found)
				ret.AWSEC2.ResourcesEnrollmentSuccess = ret.AWSEC2.ResourcesEnrollmentSuccess + int(discoveredResources.AwsEc2.Enrolled)
				ret.AWSEC2.ResourcesEnrollmentFailed = ret.AWSEC2.ResourcesEnrollmentFailed + int(discoveredResources.AwsEc2.Failed)
			}

			if matchers := matchersWithIntegration(dc, types.AWSMatcherRDS, integration.GetName()); matchers != 0 {
				ret.AWSRDS.RulesCount = ret.AWSRDS.RulesCount + matchers
				ret.AWSRDS.DiscoverLastSync = lastSync(ret.AWSRDS.DiscoverLastSync, dc.Status.LastSyncTime)
				ret.AWSRDS.ResourcesFound = ret.AWSRDS.ResourcesFound + int(discoveredResources.AwsRds.Found)
				ret.AWSRDS.ResourcesEnrollmentSuccess = ret.AWSRDS.ResourcesEnrollmentSuccess + int(discoveredResources.AwsRds.Enrolled)
				ret.AWSRDS.ResourcesEnrollmentFailed = ret.AWSRDS.ResourcesEnrollmentFailed + int(discoveredResources.AwsRds.Failed)
			}

			if matchers := matchersWithIntegration(dc, types.AWSMatcherEKS, integration.GetName()); matchers != 0 {
				ret.AWSEKS.RulesCount = ret.AWSEKS.RulesCount + matchers
				ret.AWSEKS.DiscoverLastSync = lastSync(ret.AWSEKS.DiscoverLastSync, dc.Status.LastSyncTime)
				ret.AWSEKS.ResourcesFound = ret.AWSEKS.ResourcesFound + int(discoveredResources.AwsEks.Found)
				ret.AWSEKS.ResourcesEnrollmentSuccess = ret.AWSEKS.ResourcesEnrollmentSuccess + int(discoveredResources.AwsEks.Enrolled)
				ret.AWSEKS.ResourcesEnrollmentFailed = ret.AWSEKS.ResourcesEnrollmentFailed + int(discoveredResources.AwsEks.Failed)
			}
		}

		if nextToken == "" {
			break
		}
		nextPage = nextToken
	}

	// TODO(marco): add total number of ECS Database Services.
	ret.AWSRDS.ECSDatabaseServiceCount = 0

	return ret, nil
}

func lastSync(current *time.Time, new time.Time) *time.Time {
	if current == nil {
		return &new
	}

	if current.Before(new) {
		return &new
	}

	return current
}

func matchersWithIntegration(dc *discoveryconfig.DiscoveryConfig, matcherType string, integration string) int {
	ret := 0

	for _, matcher := range dc.Spec.AWS {
		if matcher.Integration != integration {
			continue
		}
		if !slices.Contains(matcher.Types, matcherType) {
			continue
		}
		ret += len(matcher.Regions)
	}
	return ret
}

// integrationsList returns a page of Integrations
func (h *Handler) integrationsList(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	values := r.URL.Query()
	limit, err := QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	startKey := values.Get("startKey")

	igs, nextKey, err := clt.ListIntegrations(r.Context(), int(limit), startKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	items, err := ui.MakeIntegrations(igs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.IntegrationsListResponse{
		Items:   items,
		NextKey: nextKey,
	}, nil
}

// integrationsMsTeamsAppZipGet generates and returns the app.zip required for the MsTeams plugin with the given name.
func (h *Handler) integrationsMsTeamsAppZipGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plugin, err := clt.PluginsClient().GetPlugin(r.Context(), &pluginspb.GetPluginRequest{
		Name:        p.ByName("plugin"),
		WithSecrets: false,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	spec, ok := plugin.Spec.Settings.(*types.PluginSpecV1_Msteams)
	if !ok {
		return nil, trace.BadParameter("plugin specified was not of type MsTeams")
	}

	w.Header().Add("Content-Type", "application/zip")
	w.Header().Add("Content-Disposition", "attachment; filename=app.zip")
	err = msteams.WriteAppZipTo(w, msteams.ConfigTemplatePayload{
		AppID:      spec.Msteams.AppId,
		TenantID:   spec.Msteams.TenantId,
		TeamsAppID: spec.Msteams.TeamsAppId,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, nil
}
