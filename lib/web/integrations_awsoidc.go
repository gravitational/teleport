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
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/google/safetext/shsprintf"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/deployserviceconfig"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	libui "github.com/gravitational/teleport/lib/ui"
	libutils "github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/utils/oidc"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
	"github.com/gravitational/teleport/lib/web/ui"
)

// awsOIDCListDatabases returns a list of databases using the ListDatabases action of the AWS OIDC Integration.
func (h *Handler) awsOIDCListDatabases(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCListDatabasesRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listDatabasesResp, err := clt.IntegrationAWSOIDCClient().ListDatabases(ctx, &integrationv1.ListDatabasesRequest{
		Integration: integrationName,
		Region:      req.Region,
		RdsType:     req.RDSType,
		Engines:     req.Engines,
		NextToken:   req.NextToken,
		VpcId:       req.VPCID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCListDatabasesResponse{
		NextToken: listDatabasesResp.NextToken,
		Databases: ui.MakeDatabases(listDatabasesResp.Databases, accessChecker, h.cfg.DatabaseREPLRegistry),
	}, nil
}

// awsOIDCDeployService deploys a Discovery Service and a Database Service in Amazon ECS.
func (h *Handler) awsOIDCDeployService(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCDeployServiceRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databaseAgentMatcherLabels := make(types.Labels, len(req.DatabaseAgentMatcherLabels)+3)
	for _, label := range req.DatabaseAgentMatcherLabels {
		databaseAgentMatcherLabels[label.Name] = utils.Strings{label.Value}
	}

	// DELETE in 19.0: delete only the outer if block (checking labels == 0).
	// The outer block is required since older UI's will not
	// send these values to the backend, but instead send custom labels (the UI
	// will require at least one label before proceeding).
	// Newer UI's will not send any labels, but instead send the required
	// fields for default labels.
	if len(req.DatabaseAgentMatcherLabels) == 0 {
		if req.VPCID == "" {
			return nil, trace.BadParameter("vpc ID is required")
		}
		if req.Region == "" {
			return nil, trace.BadParameter("AWS region is required")
		}
		if req.AccountID == "" {
			return nil, trace.BadParameter("AWS account ID is required")
		}
		// Add default labels.
		databaseAgentMatcherLabels[types.DiscoveryLabelVPCID] = []string{req.VPCID}
		databaseAgentMatcherLabels[types.DiscoveryLabelRegion] = []string{req.Region}
		databaseAgentMatcherLabels[types.DiscoveryLabelAccountID] = []string{req.AccountID}
	}

	iamTokenName := deployserviceconfig.DefaultTeleportIAMTokenName
	teleportConfigString, err := deployserviceconfig.GenerateTeleportConfigString(
		h.PublicProxyAddr(),
		iamTokenName,
		databaseAgentMatcherLabels,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleportVersionTag := teleport.Version
	if automaticUpgrades(h.GetClusterFeatures()) {
		cloudStableVersion, err := h.cfg.AutomaticUpgradesChannels.DefaultVersion(ctx)
		if err != nil {
			return "", trace.Wrap(err)
		}

		// cloudStableVersion has vX.Y.Z format, however the container image tag does not include the `v`.
		teleportVersionTag = strings.TrimPrefix(cloudStableVersion, "v")
	}

	deployServiceResp, err := clt.IntegrationAWSOIDCClient().DeployService(ctx, &integrationv1.DeployServiceRequest{
		DeploymentJoinTokenName: iamTokenName,
		DeploymentMode:          req.DeploymentMode,
		TeleportConfigString:    teleportConfigString,
		Integration:             integrationName,
		Region:                  req.Region,
		SecurityGroups:          req.SecurityGroups,
		SubnetIds:               req.SubnetIDs,
		TaskRoleArn:             req.TaskRoleARN,
		TeleportVersion:         teleportVersionTag,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCDeployServiceResponse{
		ClusterARN:          deployServiceResp.ClusterArn,
		ServiceARN:          deployServiceResp.ServiceArn,
		TaskDefinitionARN:   deployServiceResp.TaskDefinitionArn,
		ServiceDashboardURL: deployServiceResp.ServiceDashboardUrl,
	}, nil
}

// awsOIDCDeployDatabaseService deploys a Database Service in Amazon ECS.
func (h *Handler) awsOIDCDeployDatabaseServices(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCDeployDatabaseServiceRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleportVersionTag := teleport.Version
	if automaticUpgrades(h.GetClusterFeatures()) {
		cloudStableVersion, err := h.cfg.AutomaticUpgradesChannels.DefaultVersion(ctx)
		if err != nil {
			return "", trace.Wrap(err)
		}

		// cloudStableVersion has vX.Y.Z format, however the container image tag does not include the `v`.
		teleportVersionTag = strings.TrimPrefix(cloudStableVersion, "v")
	}

	iamTokenName := deployserviceconfig.DefaultTeleportIAMTokenName
	deployments := make([]*integrationv1.DeployDatabaseServiceDeployment, 0, len(req.Deployments))
	for _, d := range req.Deployments {
		teleportConfigString, err := deployserviceconfig.GenerateTeleportConfigString(
			h.PublicProxyAddr(),
			iamTokenName,
			types.Labels{
				types.DiscoveryLabelVPCID:     []string{d.VPCID},
				types.DiscoveryLabelRegion:    []string{req.Region},
				types.DiscoveryLabelAccountID: []string{req.AccountID},
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		deployments = append(deployments, &integrationv1.DeployDatabaseServiceDeployment{
			VpcId:                d.VPCID,
			SubnetIds:            d.SubnetIDs,
			SecurityGroups:       d.SecurityGroups,
			TeleportConfigString: teleportConfigString,
		})
	}

	deployServiceResp, err := clt.IntegrationAWSOIDCClient().DeployDatabaseService(ctx, &integrationv1.DeployDatabaseServiceRequest{
		Integration:             integrationName,
		Region:                  req.Region,
		TaskRoleArn:             req.TaskRoleARN,
		Deployments:             deployments,
		TeleportVersion:         teleportVersionTag,
		DeploymentJoinTokenName: deployserviceconfig.DefaultTeleportIAMTokenName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCDeployDatabaseServiceResponse{
		ClusterARN:          deployServiceResp.ClusterArn,
		ClusterDashboardURL: deployServiceResp.ClusterDashboardUrl,
	}, nil
}

// awsOIDCListDeployedDatabaseService lists the deployed Database Services in Amazon ECS.
func (h *Handler) awsOIDCListDeployedDatabaseService(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()
	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	regions, err := regionsForListingDeployedDatabaseService(ctx, r, clt, clt.DiscoveryConfigClient())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	services, err := listDeployedDatabaseServices(ctx, h.logger, integrationName, regions, clt.IntegrationAWSOIDCClient())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCListDeployedDatabaseServiceResponse{
		Services: services,
	}, nil
}

func extractAWSRegionsFromQuery(r *http.Request) ([]string, error) {
	var ret []string
	for _, region := range r.URL.Query()["regions"] {
		if err := aws.IsValidRegion(region); err != nil {
			return nil, trace.BadParameter("invalid region %s", region)
		}
		ret = append(ret, region)
	}

	return ret, nil
}

func regionsForListingDeployedDatabaseService(ctx context.Context, r *http.Request, authClient databaseGetter, discoveryConfigsClient discoveryConfigLister) ([]string, error) {
	if r.URL.Query().Has("regions") {
		regions, err := extractAWSRegionsFromQuery(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return regions, err
	}

	regions, err := fetchRelevantAWSRegions(ctx, authClient, discoveryConfigsClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return regions, nil
}

type databaseGetter interface {
	GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error)
	GetDatabases(context.Context) ([]types.Database, error)
}

type discoveryConfigLister interface {
	ListDiscoveryConfigs(ctx context.Context, pageSize int, nextToken string) ([]*discoveryconfig.DiscoveryConfig, string, error)
}

func fetchRelevantAWSRegions(ctx context.Context, authClient databaseGetter, discoveryConfigsClient discoveryConfigLister) ([]string, error) {
	regionsSet := make(map[string]struct{})

	// Collect Regions from Database resources.
	databases, err := authClient.GetDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, resource := range databases {
		regionsSet[resource.GetAWS().Region] = struct{}{}
		regionsSet[resource.GetAllLabels()[types.DiscoveryLabelRegion]] = struct{}{}
	}

	// Iterate over all DatabaseServices and fetch their AWS Region in the matchers.
	var nextPageKey string
	for {
		req := &proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        defaults.MaxIterationLimit,
			StartKey:     nextPageKey,
			Labels:       map[string]string{types.AWSOIDCAgentLabel: types.True},
		}
		page, err := client.GetResourcePage[types.DatabaseService](ctx, authClient, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		maps.Copy(regionsSet, extractRegionsFromDatabaseServicesPage(page.Resources))

		if page.NextKey == "" {
			break
		}
		nextPageKey = page.NextKey
	}

	// Iterate over all DiscoveryConfigs and fetch their AWS Region in AWS Matchers.
	nextPageKey = ""
	for {
		resp, respNextPageKey, err := discoveryConfigsClient.ListDiscoveryConfigs(ctx, defaults.MaxIterationLimit, nextPageKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		maps.Copy(regionsSet, extractRegionsFromDiscoveryConfigPage(resp))

		if respNextPageKey == "" {
			break
		}
		nextPageKey = respNextPageKey
	}

	// Drop any invalid region.
	ret := make([]string, 0, len(regionsSet))
	for region := range regionsSet {
		if aws.IsValidRegion(region) == nil {
			ret = append(ret, region)
		}
	}

	return ret, nil
}

func extractRegionsFromDatabaseServicesPage(dbServices []types.DatabaseService) map[string]struct{} {
	regionsSet := make(map[string]struct{})
	for _, resource := range dbServices {
		for _, matcher := range resource.GetResourceMatchers() {
			if matcher.Labels == nil {
				continue
			}
			for labelKey, labelValues := range *matcher.Labels {
				if labelKey != types.DiscoveryLabelRegion {
					continue
				}
				for _, labelValue := range labelValues {
					regionsSet[labelValue] = struct{}{}
				}
			}
		}
	}

	return regionsSet
}

func extractRegionsFromDiscoveryConfigPage(discoveryConfigs []*discoveryconfig.DiscoveryConfig) map[string]struct{} {
	regionsSet := make(map[string]struct{})

	for _, dc := range discoveryConfigs {
		for _, awsMatcher := range dc.Spec.AWS {
			for _, region := range awsMatcher.Regions {
				regionsSet[region] = struct{}{}
			}
		}
	}

	return regionsSet
}

type deployedDatabaseServiceLister interface {
	ListDeployedDatabaseServices(ctx context.Context, in *integrationv1.ListDeployedDatabaseServicesRequest, opts ...grpc.CallOption) (*integrationv1.ListDeployedDatabaseServicesResponse, error)
}

func listDeployedDatabaseServices(ctx context.Context,
	logger *slog.Logger,
	integrationName string,
	regions []string,
	awsOIDCClient deployedDatabaseServiceLister,
) ([]ui.AWSOIDCDeployedDatabaseService, error) {
	var services []ui.AWSOIDCDeployedDatabaseService
	for _, region := range regions {
		var nextToken string
		for {
			resp, err := awsOIDCClient.ListDeployedDatabaseServices(ctx, &integrationv1.ListDeployedDatabaseServicesRequest{
				Integration: integrationName,
				Region:      region,
				NextToken:   nextToken,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			for _, deployedDatabaseService := range resp.DeployedDatabaseServices {
				matchingLabels, err := matchingLabelsFromDeployedService(deployedDatabaseService)
				if err != nil {
					logger.WarnContext(ctx, "Failed to obtain teleport config string from ECS Service",
						"ecs_service", deployedDatabaseService.ServiceDashboardUrl,
						"error", err,
					)
				}
				validTeleportConfigFound := err == nil

				services = append(services, ui.AWSOIDCDeployedDatabaseService{
					Name:                deployedDatabaseService.Name,
					DashboardURL:        deployedDatabaseService.ServiceDashboardUrl,
					MatchingLabels:      matchingLabels,
					ValidTeleportConfig: validTeleportConfigFound,
				})
			}

			if resp.NextToken == "" {
				break
			}
			nextToken = resp.NextToken
		}
	}
	return services, nil
}

func matchingLabelsFromDeployedService(deployedDatabaseService *integrationv1.DeployedDatabaseService) ([]libui.Label, error) {
	commandArgs := deployedDatabaseService.ContainerCommand
	// This command is what starts the teleport agent in the ECS Service Fargate container.
	// See deployservice.go/upsertTask for details.
	// It is expected to have at least 3 values, even if dumb-init is removed in the future.
	if len(commandArgs) < 3 {
		return nil, trace.BadParameter("unexpected command size, expected at least 3 args, got %d", len(commandArgs))
	}

	// The command should have a --config-string flag and then the teleport's base64 encoded configuration as argument
	teleportConfigStringFlagIdx := slices.Index(commandArgs, "--config-string")
	if teleportConfigStringFlagIdx == -1 {
		return nil, trace.BadParameter("missing --config-string flag in container command")
	}
	if len(commandArgs) < teleportConfigStringFlagIdx+1 {
		return nil, trace.BadParameter("missing --config-string argument in container command")
	}
	teleportConfigString := commandArgs[teleportConfigStringFlagIdx+1]

	labelMatchers, err := deployserviceconfig.ParseResourceLabelMatchers(teleportConfigString)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var matchingLabels []libui.Label
	for labelKey, labelValues := range labelMatchers {
		for _, labelValue := range labelValues {
			matchingLabels = append(matchingLabels, libui.Label{
				Name:  labelKey,
				Value: labelValue,
			})
		}
	}

	return matchingLabels, nil
}

// awsOIDCConfigureDeployServiceIAM returns a script that configures the required IAM permissions to enable the usage of DeployService action.
func (h *Handler) awsOIDCConfigureDeployServiceIAM(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	ctx := r.Context()

	queryParams := r.URL.Query()

	clusterName, err := h.GetProxyClient().GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := queryParams.Get("integrationName")
	if len(integrationName) == 0 {
		return nil, trace.BadParameter("missing integrationName param")
	}

	// Ensure the IntegrationName is valid.
	_, err = h.GetProxyClient().GetIntegration(ctx, integrationName)
	// NotFound error is ignored to prevent disclosure of whether the integration exists in a public/no-auth endpoint.
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	awsRegion := queryParams.Get("awsRegion")
	if err := aws.IsValidRegion(awsRegion); err != nil {
		return nil, trace.BadParameter("invalid awsRegion")
	}

	awsAccountID := queryParams.Get("awsAccountID")
	if err := aws.IsValidAccountID(awsAccountID); err != nil {
		return nil, trace.Wrap(err, "invalid awsAccountID")
	}

	role := queryParams.Get("role")
	if err := aws.IsValidIAMRoleName(role); err != nil {
		return nil, trace.BadParameter("invalid role %q", role)
	}

	taskRole := queryParams.Get("taskRole")
	if err := aws.IsValidIAMRoleName(taskRole); err != nil {
		return nil, trace.BadParameter("invalid taskRole")
	}

	// The script must execute the following command:
	// teleport integration configure deployservice-iam
	argsList := []string{
		"integration", "configure", "deployservice-iam",
		fmt.Sprintf("--cluster=%s", shsprintf.EscapeDefaultContext(clusterName)),
		fmt.Sprintf("--name=%s", shsprintf.EscapeDefaultContext(integrationName)),
		fmt.Sprintf("--aws-region=%s", shsprintf.EscapeDefaultContext(awsRegion)),
		fmt.Sprintf("--role=%s", shsprintf.EscapeDefaultContext(role)),
		fmt.Sprintf("--task-role=%s", shsprintf.EscapeDefaultContext(taskRole)),
		fmt.Sprintf("--aws-account-id=%s", shsprintf.EscapeDefaultContext(awsAccountID)),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to complete the database enrollment.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}

// awsOIDCConfigureEICEIAM returns a script that configures the required IAM permissions to enable the usage of EC2 Instance Connect Endpoint
// to access EC2 instances.
func (h *Handler) awsOIDCConfigureEICEIAM(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	queryParams := r.URL.Query()

	awsRegion := queryParams.Get("awsRegion")
	if err := aws.IsValidRegion(awsRegion); err != nil {
		return nil, trace.BadParameter("invalid awsRegion")
	}

	awsAccountID := queryParams.Get("awsAccountID")
	if err := aws.IsValidAccountID(awsAccountID); err != nil {
		return nil, trace.Wrap(err, "invalid awsAccountID")
	}

	role := queryParams.Get("role")
	if err := aws.IsValidIAMRoleName(role); err != nil {
		return nil, trace.BadParameter("invalid role %q", role)
	}

	// The script must execute the following command:
	// teleport integration configure eice-iam
	argsList := []string{
		"integration", "configure", "eice-iam",
		fmt.Sprintf("--aws-region=%s", shsprintf.EscapeDefaultContext(awsRegion)),
		fmt.Sprintf("--role=%s", shsprintf.EscapeDefaultContext(role)),
		fmt.Sprintf("--aws-account-id=%s", shsprintf.EscapeDefaultContext(awsAccountID)),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to complete the EC2 enrollment.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}

// awsOIDCConfigureAppAccessIAM returns a script that configures the required IAM permissions to enable App Access
// using the AWS OIDC Credentials.
// Only IAM Roles with `teleport.dev/integration: Allowed` Tag can be used.
// It receives the IAM Role from a query param "role".
// The script is returned using the Content-Type "text/x-shellscript". No Content-Disposition header is set.
func (h *Handler) awsOIDCConfigureAWSAppAccessIAM(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	queryParams := r.URL.Query()

	role := queryParams.Get("role")
	if err := aws.IsValidIAMRoleName(role); err != nil {
		return nil, trace.BadParameter("invalid role %q", role)
	}

	// The script must execute the following command:
	// teleport integration configure aws-app-access
	argsList := []string{
		"integration", "configure", "aws-app-access-iam",
		fmt.Sprintf("--role=%s", shsprintf.EscapeDefaultContext(role)),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to use AWS App Access.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())

	_, err = fmt.Fprint(w, script)
	return nil, trace.Wrap(err)
}

// awsOIDCConfigureEC2SSMIAM returns a script that configures AWS IAM Policies and creates an SSM Document
// to enable EC2 Auto Discover Script mode, using the AWS OIDC Credentials.
// It receives the IAM Role, AWS Region and SSM Document Name from query params ("role", "awsRegion" and "ssmDocument").
//
// The script is returned using the Content-Type "text/x-shellscript".
// No Content-Disposition header is set.
func (h *Handler) awsOIDCConfigureEC2SSMIAM(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	queryParams := r.URL.Query()

	integrationName := queryParams.Get("integrationName")
	if len(integrationName) == 0 {
		return nil, trace.BadParameter("missing integrationName param")
	}

	role := queryParams.Get("role")
	if err := aws.IsValidIAMRoleName(role); err != nil {
		return nil, trace.BadParameter("invalid role %q", role)
	}

	region := queryParams.Get("awsRegion")
	if err := aws.IsValidRegion(region); err != nil {
		return nil, trace.BadParameter("invalid region %q", region)
	}

	awsAccountID := queryParams.Get("awsAccountID")
	if err := aws.IsValidAccountID(awsAccountID); err != nil {
		return nil, trace.Wrap(err, "invalid awsAccountID")
	}

	ssmDocumentName := queryParams.Get("ssmDocument")
	if ssmDocumentName == "" {
		return nil, trace.BadParameter("missing ssmDocument query param")
	}
	// PublicProxyAddr() might return tenant.teleport.sh
	// However, the expected format for --proxy-public-url includes the protocol `https://`
	proxyPublicURL := h.PublicProxyAddr()
	if !strings.HasPrefix(proxyPublicURL, "https://") {
		proxyPublicURL = "https://" + proxyPublicURL
	}

	clusterName, err := h.GetProxyClient().GetDomainName(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The script must execute the following command:
	// teleport integration configure ec2-ssm-iam
	argsList := []string{
		"integration", "configure", "ec2-ssm-iam",
		fmt.Sprintf("--role=%s", shsprintf.EscapeDefaultContext(role)),
		fmt.Sprintf("--aws-region=%s", shsprintf.EscapeDefaultContext(region)),
		fmt.Sprintf("--ssm-document-name=%s", shsprintf.EscapeDefaultContext(ssmDocumentName)),
		fmt.Sprintf("--proxy-public-url=%s", shsprintf.EscapeDefaultContext(proxyPublicURL)),
		fmt.Sprintf("--cluster=%s", shsprintf.EscapeDefaultContext(clusterName)),
		fmt.Sprintf("--name=%s", shsprintf.EscapeDefaultContext(integrationName)),
		fmt.Sprintf("--aws-account-id=%s", shsprintf.EscapeDefaultContext(awsAccountID)),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to finish the EC2 auto discover set up.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())

	_, err = fmt.Fprint(w, script)
	return nil, trace.Wrap(err)
}

// awsOIDCConfigureEKSIAM returns a script that configures the required IAM permissions to enroll EKS clusters into Teleport.
func (h *Handler) awsOIDCConfigureEKSIAM(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	queryParams := r.URL.Query()

	awsRegion := queryParams.Get("awsRegion")
	if err := aws.IsValidRegion(awsRegion); err != nil {
		return nil, trace.BadParameter("invalid aws region")
	}

	awsAccountID := queryParams.Get("awsAccountID")
	if err := aws.IsValidAccountID(awsAccountID); err != nil {
		return nil, trace.Wrap(err, "invalid awsAccountID")
	}

	role := queryParams.Get("role")
	if err := aws.IsValidIAMRoleName(role); err != nil {
		return nil, trace.BadParameter("invalid role %q", role)
	}

	// The script must execute the following command:
	// "teleport integration configure eks-iam"
	argsList := []string{
		"integration", "configure", "eks-iam",
		fmt.Sprintf("--aws-region=%s", shsprintf.EscapeDefaultContext(awsRegion)),
		fmt.Sprintf("--role=%s", shsprintf.EscapeDefaultContext(role)),
		fmt.Sprintf("--aws-account-id=%s", shsprintf.EscapeDefaultContext(awsAccountID)),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to complete the EKS enrollment.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}

// awsOIDCEnrollEKSClusters enroll EKS clusters by installing teleport-kube-agent Helm chart on them.
// v2 endpoint introduces "extraLabels" field.
func (h *Handler) awsOIDCEnrollEKSClusters(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCEnrollEKSClustersRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	agentVersion, err := kubeutils.GetKubeAgentVersion(ctx, h.cfg.ProxyClient, h.GetClusterFeatures(), h.cfg.AutomaticUpgradesChannels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	extraLabels := make(map[string]string, len(req.ExtraLabels))
	for _, label := range req.ExtraLabels {
		extraLabels[label.Name] = label.Value
	}

	response, err := clt.IntegrationAWSOIDCClient().EnrollEKSClusters(ctx, &integrationv1.EnrollEKSClustersRequest{
		Integration:        integrationName,
		Region:             req.Region,
		EksClusterNames:    req.ClusterNames,
		EnableAppDiscovery: req.EnableAppDiscovery,
		AgentVersion:       agentVersion,
		ExtraLabels:        extraLabels,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var data []ui.EKSClusterEnrollmentResult
	for _, result := range response.Results {
		data = append(data, ui.EKSClusterEnrollmentResult{
			ClusterName: result.EksClusterName,
			Error:       result.Error,
			ResourceId:  result.ResourceId,
		},
		)
	}

	return ui.AWSOIDCEnrollEKSClustersResponse{
		Results: data,
	}, nil
}

// awsOIDCListEKSClusters returns a list of EKS clusters using the ListEKSClusters action of the AWS OIDC integration.
func (h *Handler) awsOIDCListEKSClusters(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCListEKSClustersRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listResp, err := clt.IntegrationAWSOIDCClient().ListEKSClusters(ctx, &integrationv1.ListEKSClustersRequest{
		Integration: integrationName,
		Region:      req.Region,
		NextToken:   req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCListEKSClustersResponse{
		NextToken: listResp.NextToken,
		Clusters:  ui.MakeEKSClusters(listResp.Clusters),
	}, nil
}

// awsOIDCListEC2 returns a list of EC2 Instances using the ListEC2 action of the AWS OIDC Integration.
func (h *Handler) awsOIDCListEC2(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCListEC2Request
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listResp, err := clt.IntegrationAWSOIDCClient().ListEC2(ctx, &integrationv1.ListEC2Request{
		Integration: integrationName,
		Region:      req.Region,
		NextToken:   req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers := make([]ui.Server, 0, len(listResp.Servers))
	for _, s := range listResp.Servers {
		logins, err := accessChecker.GetAllowedLoginsForResource(s)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		slices.Sort(logins)

		servers = append(servers, ui.MakeServer(h.auth.clusterName, s, logins, false /* requiresRequest */))
	}

	return ui.AWSOIDCListEC2Response{
		NextToken: listResp.NextToken,
		Servers:   servers,
	}, nil
}

// awsOIDCListSecurityGroups returns a list of VPC Security Groups using the ListSecurityGroups action of the AWS OIDC Integration.
func (h *Handler) awsOIDCListSecurityGroups(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCListSecurityGroupsRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listResp, err := clt.IntegrationAWSOIDCClient().ListSecurityGroups(ctx, &integrationv1.ListSecurityGroupsRequest{
		Integration: integrationName,
		Region:      req.Region,
		VpcId:       req.VPCID,
		NextToken:   req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sgs := make([]awsoidc.SecurityGroup, 0, len(listResp.SecurityGroups))
	for _, sg := range listResp.SecurityGroups {
		sgs = append(sgs, awsoidc.SecurityGroup{
			Name:          sg.Name,
			ID:            sg.Id,
			Description:   sg.Description,
			InboundRules:  awsOIDCSecurityGroupsRulesConverter(sg.InboundRules),
			OutboundRules: awsOIDCSecurityGroupsRulesConverter(sg.OutboundRules),
		})
	}

	return ui.AWSOIDCListSecurityGroupsResponse{
		NextToken:      listResp.NextToken,
		SecurityGroups: sgs,
	}, nil
}

func awsOIDCSecurityGroupsRulesConverter(inRules []*integrationv1.SecurityGroupRule) []awsoidc.SecurityGroupRule {
	out := make([]awsoidc.SecurityGroupRule, 0, len(inRules))
	for _, r := range inRules {
		var cidrs []awsoidc.CIDR
		if len(r.Cidrs) > 0 {
			cidrs = make([]awsoidc.CIDR, 0, len(r.Cidrs))
		}
		for _, cidr := range r.Cidrs {
			cidrs = append(cidrs, awsoidc.CIDR{
				CIDR:        cidr.Cidr,
				Description: cidr.Description,
			})
		}

		var groupIDs []awsoidc.GroupIDRule
		if len(r.GroupIds) > 0 {
			groupIDs = make([]awsoidc.GroupIDRule, 0, len(r.GroupIds))
		}
		for _, group := range r.GroupIds {
			groupIDs = append(groupIDs, awsoidc.GroupIDRule{
				GroupId:     group.GroupId,
				Description: group.Description,
			})
		}
		out = append(out, awsoidc.SecurityGroupRule{
			IPProtocol: r.IpProtocol,
			FromPort:   int(r.FromPort),
			ToPort:     int(r.ToPort),
			CIDRs:      cidrs,
			Groups:     groupIDs,
		})
	}
	return out
}

// awsOIDCRequiredDatabasesVPCS returns a map of required VPC's and its subnets.
// This is required during the web UI discover flow (where users opt for auto
// discovery) to determine if user can skip the auto deployment screen (where we deploy
// database agents).
//
// This api will return empty if we already have agents that can proxy the discovered databases.
// Otherwise it will return with a map of VPC and its subnets where it's values are later used
// to configure and deploy an agent (deploy an agent per unique VPC).
func (h *Handler) awsOIDCRequiredDatabasesVPCS(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCRequiredVPCSRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	respAllDatabases, err := awsOIDCListAllDatabases(ctx, clt, integrationName, req.Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(respAllDatabases) == 0 {
		return nil, trace.BadParameter("there are no available RDS instances or clusters found in region %q", req.Region)
	}

	resp, err := awsOIDCRequiredVPCSHelper(ctx, clt, req, respAllDatabases)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func awsOIDCListAllDatabases(ctx context.Context, clt authclient.ClientI, integration, region string) ([]*types.DatabaseV3, error) {
	nextToken := ""
	var fetchedRDSs []*types.DatabaseV3

	// Get all rds instances.
	for {
		resp, err := clt.IntegrationAWSOIDCClient().ListDatabases(ctx, &integrationv1.ListDatabasesRequest{
			Integration: integration,
			Region:      region,
			RdsType:     services.RDSDescribeTypeInstance,
			Engines:     []string{services.RDSEngineMySQL, services.RDSEngineMariaDB, services.RDSEnginePostgres},
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		fetchedRDSs = append(fetchedRDSs, resp.Databases...)
		nextToken = resp.NextToken

		if len(nextToken) == 0 {
			break
		}
	}

	// Get all rds clusters.
	nextToken = ""
	for {
		resp, err := clt.IntegrationAWSOIDCClient().ListDatabases(ctx, &integrationv1.ListDatabasesRequest{
			Integration: integration,
			Region:      region,
			RdsType:     services.RDSDescribeTypeCluster,
			Engines:     []string{services.RDSEngineAuroraMySQL, services.RDSEngineAuroraPostgres},
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		fetchedRDSs = append(fetchedRDSs, resp.Databases...)
		nextToken = resp.NextToken

		if len(nextToken) == 0 {
			break
		}
	}

	return fetchedRDSs, nil
}

func awsOIDCRequiredVPCSHelper(ctx context.Context, clt client.GetResourcesClient, req ui.AWSOIDCRequiredVPCSRequest, fetchedRDSs []*types.DatabaseV3) (*ui.AWSOIDCRequiredVPCSResponse, error) {
	// Get all database services with ecs/fargate metadata label.
	fetchedDbSvcs, err := fetchAWSOIDCDatabaseServices(ctx, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Construct map of VPCs and its subnets.
	vpcLookup := map[string][]string{}
	for _, db := range fetchedRDSs {
		rds := db.GetAWS().RDS
		vpcId := rds.VPCID
		if _, found := vpcLookup[vpcId]; !found {
			vpcLookup[vpcId] = rds.Subnets
			continue
		}
		combinedSubnets := append(vpcLookup[vpcId], rds.Subnets...)
		vpcLookup[vpcId] = utils.Deduplicate(combinedSubnets)
	}

	for _, svc := range fetchedDbSvcs {
		vpcID := getDBServiceVPC(svc, req.AccountID, req.Region)
		if vpcID != "" {
			delete(vpcLookup, vpcID)
		}
	}

	return &ui.AWSOIDCRequiredVPCSResponse{
		VPCMapOfSubnets: vpcLookup,
	}, nil
}

// awsOIDCListEC2ICE returns a list of EC2 Instance Connect Endpoints using the ListEC2ICE action of the AWS OIDC Integration.
func (h *Handler) awsOIDCListEC2ICE(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCListEC2ICERequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vpcIds := req.VPCIDs
	if len(vpcIds) == 0 {
		vpcIds = []string{req.VPCID}
	}

	resp, err := clt.IntegrationAWSOIDCClient().ListEICE(ctx, &integrationv1.ListEICERequest{
		Integration: integrationName,
		Region:      req.Region,
		VpcIds:      vpcIds,
		NextToken:   req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	endpoints := make([]awsoidc.EC2InstanceConnectEndpoint, 0, len(resp.Ec2Ices))
	for _, e := range resp.Ec2Ices {
		endpoints = append(endpoints, awsoidc.EC2InstanceConnectEndpoint{
			Name:          e.Name,
			State:         e.State,
			StateMessage:  e.StateMessage,
			DashboardLink: e.DashboardLink,
			SubnetID:      e.SubnetId,
			VPCID:         e.VpcId,
		})
	}

	return ui.AWSOIDCListEC2ICEResponse{
		NextToken:     resp.NextToken,
		DashboardLink: resp.DashboardLink,
		EC2ICEs:       endpoints,
	}, nil
}

// awsOIDCDeployC2ICE creates an EC2 Instance Connect Endpoint.
func (h *Handler) awsOIDCDeployEC2ICE(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCDeployEC2ICERequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	endpoints := make([]*integrationv1.EC2ICEndpoint, 0, len(req.Endpoints))
	for _, endpoint := range req.Endpoints {
		endpoints = append(endpoints, &integrationv1.EC2ICEndpoint{
			SubnetId:         endpoint.SubnetID,
			SecurityGroupIds: endpoint.SecurityGroupIDs,
		})
	}

	// Backwards compatible: get the endpoint from the deprecated fields.
	if len(endpoints) == 0 {
		endpoints = append(endpoints, &integrationv1.EC2ICEndpoint{
			SubnetId:         req.SubnetID,
			SecurityGroupIds: req.SecurityGroupIDs,
		})
	}

	createResp, err := clt.IntegrationAWSOIDCClient().CreateEICE(ctx, &integrationv1.CreateEICERequest{
		Integration: integrationName,
		Region:      req.Region,
		Endpoints:   endpoints,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	respEndpoints := make([]ui.AWSOIDCDeployEC2ICEResponseEndpoint, 0, len(createResp.CreatedEndpoints))
	for _, endpoint := range createResp.CreatedEndpoints {
		respEndpoints = append(respEndpoints, ui.AWSOIDCDeployEC2ICEResponseEndpoint{
			Name:     endpoint.Name,
			SubnetID: endpoint.SubnetId,
		})
	}

	return ui.AWSOIDCDeployEC2ICEResponse{
		Name:      createResp.Name,
		Endpoints: respEndpoints,
	}, nil
}

// awsOIDCCreateAWSAppAccess creates an AppServer that uses an AWS OIDC Integration for proxying access.
// v2 endpoint introduces "labels" field
func (h *Handler) awsOIDCCreateAWSAppAccess(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCCreateAWSAppAccessRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := clt.GetIntegration(ctx, integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ig.GetSubKind() != types.IntegrationSubKindAWSOIDC {
		return nil, trace.BadParameter("only aws oidc integrations are supported")
	}

	getUserGroupLookup := h.getUserGroupLookup(r.Context(), clt)

	publicAddr := libutils.DefaultAppPublicAddr(integrationName, h.PublicProxyAddr())

	parsedRoleARN, err := awsutils.ParseRoleARN(ig.GetAWSOIDCIntegrationSpec().RoleARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels := make(map[string]string)
	if len(req.Labels) > 0 {
		labels = req.Labels
	}
	labels[constants.AWSAccountIDLabel] = parsedRoleARN.AccountID

	appServer, err := types.NewAppServerForAWSOIDCIntegration(integrationName, h.cfg.HostUUID, publicAddr, labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the integration name contains a dot, then the proxy must provide a certificate allowing *.<something>.<proxyPublicAddr>
	if strings.Contains(integrationName, ".") {
		// Teleport Cloud only provides certificates for *.<tenant>.teleport.sh, so this would generate an invalid address.
		if h.GetClusterFeatures().Cloud {
			return nil, trace.BadParameter(`Invalid integration name for enabling AWS Access. Please re-create the integration without the "."`)
		}

		// Typically, self-hosted clusters will also have a single wildcard for the name.
		// Logging a warning message should help debug the problem in case the certificate is not valid.
		h.logger.WarnContext(ctx, `Enabling AWS Access using an integration with a "." might not work unless your Proxy's certificate is valid for the address`, "public_addr", appServer.GetApp().GetPublicAddr())
	}

	if _, err := clt.UpsertApplicationServer(ctx, appServer); err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedAWSRoles, err := accessChecker.GetAllowedLoginsForResource(appServer.GetApp())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedAWSRolesLookup := map[string][]string{
		appServer.GetName(): allowedAWSRoles,
	}

	return ui.MakeApp(appServer.GetApp(), ui.MakeAppsConfig{
		LocalClusterName:      h.auth.clusterName,
		LocalProxyDNSName:     h.proxyDNSName(),
		AppClusterName:        site.GetName(),
		AllowedAWSRolesLookup: allowedAWSRolesLookup,
		UserGroupLookup:       getUserGroupLookup(),
		Logger:                h.log,
	}), nil
}

// awsOIDCDeleteAWSAppAccess deletes the AWS AppServer created that uses the AWS OIDC Integration for proxying requests.
func (h *Handler) awsOIDCDeleteAWSAppAccess(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	subkind := p.ByName("name_or_subkind")
	if subkind != types.IntegrationSubKindAWSOIDC {
		return nil, trace.BadParameter("only aws oidc integrations are supported")
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := clt.GetIntegration(ctx, integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ig.GetSubKind() != types.IntegrationSubKindAWSOIDC {
		return nil, trace.BadParameter("only aws oidc integrations are supported")
	}

	integrationAppServer, err := h.getAppServerByName(ctx, clt, integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if integrationAppServer.GetApp().GetIntegration() != integrationName {
		return nil, trace.NotFound("app %s is not using integration %s", integrationAppServer.GetName(), integrationName)
	}

	if err := clt.DeleteApplicationServer(ctx, apidefaults.Namespace, integrationAppServer.GetHostID(), integrationName); err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

func (h *Handler) getAppServerByName(ctx context.Context, userClient authclient.ClientI, appServerName string) (types.AppServer, error) {
	appServers, err := userClient.GetApplicationServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, s := range appServers {
		if s.GetName() == appServerName {
			return s, nil
		}
	}
	return nil, trace.NotFound("app %q not found", appServerName)
}

// awsOIDCConfigureIdP returns a script that configures AWS OIDC Integration
// by creating an OIDC Identity Provider that trusts Teleport instance.
func (h *Handler) awsOIDCConfigureIdP(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	ctx := r.Context()

	queryParams := r.URL.Query()

	clusterName, err := h.GetProxyClient().GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := queryParams.Get("integrationName")
	if len(integrationName) == 0 {
		return nil, trace.BadParameter("missing integrationName param")
	}

	// Ensure the IntegrationName is valid.
	_, err = h.GetProxyClient().GetIntegration(ctx, integrationName)
	// NotFound error is ignored to prevent disclosure of whether the integration exists in a public/no-auth endpoint.
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	role := queryParams.Get("role")
	if err := aws.IsValidIAMRoleName(role); err != nil {
		return nil, trace.BadParameter("invalid role %q", role)
	}

	proxyAddr, err := oidc.IssuerFromPublicAddress(h.cfg.PublicProxyAddr, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The script must execute the following command:
	// teleport integration configure awsoidc-idp
	argsList := []string{
		"integration", "configure", "awsoidc-idp",
		fmt.Sprintf("--cluster=%s", shsprintf.EscapeDefaultContext(clusterName)),
		fmt.Sprintf("--name=%s", shsprintf.EscapeDefaultContext(integrationName)),
		fmt.Sprintf("--role=%s", shsprintf.EscapeDefaultContext(role)),
		fmt.Sprintf("--proxy-public-url=%s", shsprintf.EscapeDefaultContext(proxyAddr)),
	}

	policyPreset := queryParams.Get("policyPreset")
	if err := awsoidc.ValidatePolicyPreset(awsoidc.PolicyPreset(policyPreset)); err != nil {
		return nil, trace.Wrap(err)
	}
	if policyPreset != "" {
		argsList = append(argsList, fmt.Sprintf("--policy-preset=%s", shsprintf.EscapeDefaultContext(policyPreset)))
	}

	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to use the integration with AWS.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}

// awsOIDCConfigureListDatabasesIAM returns a script that configures the required IAM permissions to allow Listing RDS DB Clusters and Instances.
func (h *Handler) awsOIDCConfigureListDatabasesIAM(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	queryParams := r.URL.Query()

	awsRegion := queryParams.Get("awsRegion")
	if err := aws.IsValidRegion(awsRegion); err != nil {
		return nil, trace.BadParameter("invalid awsRegion")
	}

	awsAccountID := queryParams.Get("awsAccountID")
	if err := aws.IsValidAccountID(awsAccountID); err != nil {
		return nil, trace.Wrap(err, "invalid awsAccountID")
	}

	role := queryParams.Get("role")
	if err := aws.IsValidIAMRoleName(role); err != nil {
		return nil, trace.BadParameter("invalid role %q", role)
	}

	// The script must execute the following command:
	// teleport integration configure listdatabases-iam
	argsList := []string{
		"integration", "configure", "listdatabases-iam",
		fmt.Sprintf("--aws-region=%s", shsprintf.EscapeDefaultContext(awsRegion)),
		fmt.Sprintf("--role=%s", shsprintf.EscapeDefaultContext(role)),
		fmt.Sprintf("--aws-account-id=%s", shsprintf.EscapeDefaultContext(awsAccountID)),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to complete the Database enrollment.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}

// accessGraphCloudSyncOIDC returns a script that configures the required IAM permissions to sync
// Cloud resources with Teleport Access Graph.
func (h *Handler) accessGraphCloudSyncOIDC(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	queryParams := r.URL.Query()

	switch kind := queryParams.Get("kind"); kind {
	case "aws-iam":
		return h.awsAccessGraphOIDCSync(w, r, p)
	default:
		return nil, trace.BadParameter("unsupported kind provided %q", kind)
	}
}

func (h *Handler) awsAccessGraphOIDCSync(w http.ResponseWriter, r *http.Request, _ httprouter.Params) (any, error) {
	queryParams := r.URL.Query()
	role := queryParams.Get("role")
	if err := aws.IsValidIAMRoleName(role); err != nil {
		return nil, trace.BadParameter("invalid role %q", role)
	}

	awsAccountID := queryParams.Get("awsAccountID")
	if err := aws.IsValidAccountID(awsAccountID); err != nil {
		return nil, trace.Wrap(err, "invalid awsAccountID")
	}

	// The script must execute the following command:
	// "teleport integration configure access-graph aws-iam"
	argsList := []string{
		"integration", "configure", "access-graph", "aws-iam",
		fmt.Sprintf("--role=%s", shsprintf.EscapeDefaultContext(role)),
		fmt.Sprintf("--aws-account-id=%s", shsprintf.EscapeDefaultContext(awsAccountID)),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the Teleport Web UI to complete the Access Graph AWS Sync enrollment.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}

// awsOIDCListSubnets returns a list of VPC subnets using the ListSubnets action of the AWS OIDC Integration.
func (h *Handler) awsOIDCListSubnets(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCListSubnetsRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listResp, err := clt.IntegrationAWSOIDCClient().ListSubnets(ctx, &integrationv1.ListSubnetsRequest{
		Integration: integrationName,
		Region:      req.Region,
		VpcId:       req.VPCID,
		NextToken:   req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	subnets := make([]awsoidc.Subnet, 0, len(listResp.Subnets))
	for _, s := range listResp.Subnets {
		subnets = append(subnets, awsoidc.Subnet{
			Name:             s.Name,
			ID:               s.Id,
			AvailabilityZone: s.AvailabilityZone,
		})
	}

	return ui.AWSOIDCListSubnetsResponse{
		NextToken: listResp.NextToken,
		Subnets:   subnets,
	}, nil
}

// awsOIDCListDatabaseVPCs returns a list of VPCs using the ListVpcs action
// of the AWS OIDC Integration, and includes a link to the ECS service if
// a database service has been deployed for each VPC.
func (h *Handler) awsOIDCListDatabaseVPCs(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCListVPCsRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listResp, err := clt.IntegrationAWSOIDCClient().ListVPCs(ctx, &integrationv1.ListVPCsRequest{
		Integration: integrationName,
		Region:      req.Region,
		NextToken:   req.NextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbServices, err := fetchAWSOIDCDatabaseServices(ctx, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceURLByVPC, err := getServiceURLs(dbServices, req.AccountID, req.Region, h.auth.clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vpcs := make([]ui.DatabaseEnrollmentVPC, 0, len(listResp.Vpcs))
	for _, vpc := range listResp.Vpcs {
		vpcs = append(vpcs, ui.DatabaseEnrollmentVPC{
			VPC: awsoidc.VPC{
				Name: vpc.Name,
				ID:   vpc.Id,
			},
			ECSServiceDashboardURL: serviceURLByVPC[vpc.Id],
		})
	}

	return ui.AWSOIDCDatabaseVPCsResponse{
		NextToken: listResp.NextToken,
		VPCs:      vpcs,
	}, nil
}

func fetchAWSOIDCDatabaseServices(ctx context.Context, clt client.GetResourcesClient) ([]types.DatabaseService, error) {
	// Get all database services with the AWS OIDC agent metadata label.
	var nextToken string
	var fetchedDbSvcs []types.DatabaseService
	for {
		page, err := client.GetResourcePage[types.DatabaseService](ctx, clt, &proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        defaults.MaxIterationLimit,
			StartKey:     nextToken,
			Labels:       map[string]string{types.AWSOIDCAgentLabel: types.True},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		fetchedDbSvcs = append(fetchedDbSvcs, page.Resources...)
		nextToken = page.NextKey
		if len(nextToken) == 0 {
			return fetchedDbSvcs, nil
		}
	}
}

// getDBServiceVPC returns the database service's VPC ID selector value if the
// database service was deployed by the AWS OIDC integration, otherwise it
// returns an empty string.
func getDBServiceVPC(svc types.DatabaseService, accountID, region string) string {
	if len(svc.GetResourceMatchers()) != 1 || svc.GetResourceMatchers()[0].Labels == nil {
		return ""
	}

	// Database services deployed by Teleport have known configurations where
	// we will only define a single resource matcher.
	labelMatcher := *svc.GetResourceMatchers()[0].Labels

	// We check for length 3, because we are only
	// wanting/checking for 3 discovery labels.
	if len(labelMatcher) != 3 {
		return ""
	}
	if slices.Compare(labelMatcher[types.DiscoveryLabelAccountID], []string{accountID}) != 0 {
		return ""
	}
	if slices.Compare(labelMatcher[types.DiscoveryLabelRegion], []string{region}) != 0 {
		return ""
	}
	if len(labelMatcher[types.DiscoveryLabelVPCID]) != 1 {
		return ""
	}
	return labelMatcher[types.DiscoveryLabelVPCID][0]
}

// getServiceURLs returns a map vpcID -> service URL for ECS services deployed
// by the OIDC integration in the given account and region.
func getServiceURLs(dbServices []types.DatabaseService, accountID, region, teleportClusterName string) (map[string]string, error) {
	serviceURLByVPC := make(map[string]string)
	for _, svc := range dbServices {
		vpcID := getDBServiceVPC(svc, accountID, region)
		if vpcID == "" {
			continue
		}
		svcURL, err := awsoidc.ECSDatabaseServiceDashboardURL(region, teleportClusterName, vpcID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		serviceURLByVPC[vpcID] = svcURL
	}
	return serviceURLByVPC, nil
}

// awsOIDCPing performs an health check for the integration.
// If ARN is present in the request body, that's the ARN that will be used instead of using the one stored in the integration.
// Returns meta information: account id and assumed the ARN for the IAM Role.
func (h *Handler) awsOIDCPing(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	var req ui.AWSOIDCPingRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.RoleARN != "" {
		integrationName = ""
	}

	pingResp, err := clt.IntegrationAWSOIDCClient().Ping(ctx, &integrationv1.PingRequest{
		Integration: integrationName,
		RoleArn:     req.RoleARN,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCPingResponse{
		AccountID: pingResp.AccountId,
		ARN:       pingResp.Arn,
		UserID:    pingResp.UserId,
	}, nil
}
