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
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/integrations/access/msteams"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	libui "github.com/gravitational/teleport/lib/ui"
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

// integrationStats returns the integration stats.
func (h *Handler) integrationStats(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
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

	summary, err := collectAWSOIDCAutoDiscoverStats(r.Context(), h.logger, ig, clt.DiscoveryConfigClient(), clt, clt.IntegrationAWSOIDCClient())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return summary, nil
}

func collectAWSOIDCAutoDiscoverStats(
	ctx context.Context,
	logger *slog.Logger,
	integration types.Integration,
	discoveryConfigLister discoveryConfigLister,
	databaseGetter databaseGetter,
	awsOIDCClient deployedDatabaseServiceLister,
) (*ui.IntegrationWithSummary, error) {
	ret := &ui.IntegrationWithSummary{}

	uiIg, err := ui.MakeIntegration(integration)
	if err != nil {
		return nil, err
	}
	ret.Integration = uiIg

	var nextPage string
	for {
		discoveryConfigs, nextToken, err := discoveryConfigLister.ListDiscoveryConfigs(ctx, 0, nextPage)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, dc := range discoveryConfigs {
			discoveredResources, ok := dc.Status.IntegrationDiscoveredResources[integration.GetName()]
			if !ok {
				continue
			}

			if matchers := rulesWithIntegration(dc, types.AWSMatcherEC2, integration.GetName()); matchers != 0 {
				ret.AWSEC2.RulesCount += matchers
				mergeResourceTypeSummary(&ret.AWSEC2, dc.Status.LastSyncTime, discoveredResources.AwsEc2)
			}

			if matchers := rulesWithIntegration(dc, types.AWSMatcherRDS, integration.GetName()); matchers != 0 {
				ret.AWSRDS.RulesCount += matchers
				mergeResourceTypeSummary(&ret.AWSRDS, dc.Status.LastSyncTime, discoveredResources.AwsRds)
			}

			if matchers := rulesWithIntegration(dc, types.AWSMatcherEKS, integration.GetName()); matchers != 0 {
				ret.AWSEKS.RulesCount += matchers
				mergeResourceTypeSummary(&ret.AWSEKS, dc.Status.LastSyncTime, discoveredResources.AwsEks)
			}
		}

		if nextToken == "" {
			break
		}
		nextPage = nextToken
	}

	regions, err := fetchRelevantAWSRegions(ctx, databaseGetter, discoveryConfigLister)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	services, err := listDeployedDatabaseServices(ctx, logger, integration.GetName(), regions, awsOIDCClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ret.AWSRDS.ECSDatabaseServiceCount = len(services)

	return ret, nil
}

func mergeResourceTypeSummary(in *ui.ResourceTypeSummary, lastSyncTime time.Time, new *discoveryconfigv1.ResourcesDiscoveredSummary) {
	in.DiscoverLastSync = lastSync(in.DiscoverLastSync, lastSyncTime)
	in.ResourcesFound += int(new.Found)
	in.ResourcesEnrollmentSuccess += int(new.Enrolled)
	in.ResourcesEnrollmentFailed += int(new.Failed)
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

// rulesWithIntegration returns the number of Rules for a given integration and matcher type in the DiscoveryConfig.
// A Rule is similar to a DiscoveryConfig's Matcher, eg DiscoveryConfig.Spec.AWS.[<Matcher>], however, a Rule has a single region.
// This means that the number of Rules for a given Matcher is equal to the number of regions on that Matcher.
func rulesWithIntegration(dc *discoveryconfig.DiscoveryConfig, matcherType string, integration string) int {
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

// integrationDiscoveryRules returns the Discovery Rules that are using a given integration.
// A Discovery Rule is just like a DiscoveryConfig Matcher, except that it breaks down by region.
// So, if a Matcher exists for two regions, that will be represented as two Rules.
func (h *Handler) integrationDiscoveryRules(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	values := r.URL.Query()
	startKey := values.Get("startKey")
	resourceType := values.Get("resourceType")

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := clt.GetIntegration(r.Context(), integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rules, err := collectAutoDiscoveryRules(r.Context(), ig.GetName(), startKey, resourceType, clt.DiscoveryConfigClient())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rules, nil
}

// collectAutoDiscoveryRules will iterate over all DiscoveryConfigs's Matchers and collect the Discovery Rules that exist in them for the given integration.
// It can also be filtered by Matcher Type (eg ec2, rds, eks)
// A Discovery Rule is a close match to a DiscoveryConfig's Matcher, except that it will count as many rules as regions exist.
// Eg if a DiscoveryConfig's Matcher has two regions, then it will output two (almost equal) Rules, one for each Region.
func collectAutoDiscoveryRules(
	ctx context.Context,
	integrationName string,
	nextPage string,
	resourceTypeFilter string,
	clt interface {
		ListDiscoveryConfigs(ctx context.Context, pageSize int, nextToken string) ([]*discoveryconfig.DiscoveryConfig, string, error)
	},
) (ui.IntegrationDiscoveryRules, error) {
	const (
		maxPerPage = 100
	)
	var ret ui.IntegrationDiscoveryRules
	for {
		discoveryConfigs, nextToken, err := clt.ListDiscoveryConfigs(ctx, 0, nextPage)
		if err != nil {
			return ret, trace.Wrap(err)
		}
		for _, dc := range discoveryConfigs {
			lastSync := &dc.Status.LastSyncTime
			if lastSync.IsZero() {
				lastSync = nil
			}

			for _, matcher := range dc.Spec.AWS {
				if matcher.Integration != integrationName {
					continue
				}

				for _, resourceType := range matcher.Types {
					if resourceTypeFilter != "" && resourceType != resourceTypeFilter {
						continue
					}

					for _, region := range matcher.Regions {
						uiLables := make([]libui.Label, 0, len(matcher.Tags))
						for labelKey, labelValues := range matcher.Tags {
							for _, labelValue := range labelValues {
								uiLables = append(uiLables, libui.Label{
									Name:  labelKey,
									Value: labelValue,
								})
							}
						}
						ret.Rules = append(ret.Rules, ui.IntegrationDiscoveryRule{
							ResourceType:    resourceType,
							Region:          region,
							LabelMatcher:    uiLables,
							DiscoveryConfig: dc.GetName(),
							LastSync:        lastSync,
						})
					}
				}
			}
		}

		ret.NextKey = nextToken

		if nextToken == "" || len(ret.Rules) > maxPerPage {
			break
		}

		nextPage = nextToken
	}

	return ret, nil
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
