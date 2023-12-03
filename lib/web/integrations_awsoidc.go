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
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
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

	awsClientReq, err := h.awsOIDCClientRequest(ctx, req.Region, p, sctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listDBsClient, err := awsoidc.NewListDatabasesClient(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := awsoidc.ListDatabases(ctx,
		listDBsClient,
		awsoidc.ListDatabasesRequest{
			Region:    req.Region,
			NextToken: req.NextToken,
			Engines:   req.Engines,
			RDSType:   req.RDSType,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCListDatabasesResponse{
		NextToken: resp.NextToken,
		Databases: ui.MakeDatabases(resp.Databases, nil, nil),
	}, nil
}

// awsOIDClientRequest receives a request to execute an action for the AWS OIDC integrations.
func (h *Handler) awsOIDCClientRequest(ctx context.Context, region string, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (*awsoidc.AWSClientRequest, error) {
	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	integration, err := clt.GetIntegration(ctx, integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if integration.GetSubKind() != types.IntegrationSubKindAWSOIDC {
		return nil, trace.BadParameter("integration subkind (%s) mismatch", integration.GetSubKind())
	}

	issuer, err := oidc.IssuerFromPublicAddress(h.cfg.PublicProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := clt.GenerateAWSOIDCToken(ctx, types.GenerateAWSOIDCTokenRequest{
		Issuer: issuer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsoidcSpec := integration.GetAWSOIDCIntegrationSpec()
	if awsoidcSpec == nil {
		return nil, trace.BadParameter("missing spec fields for %q (%q) integration", integration.GetName(), integration.GetSubKind())
	}

	return &awsoidc.AWSClientRequest{
		IntegrationName: integrationName,
		Token:           token,
		RoleARN:         awsoidcSpec.RoleARN,
		Region:          region,
	}, nil
}

// awsOIDCDeployService deploys a Discovery Service and a Database Service in Amazon ECS.
func (h *Handler) awsOIDCDeployService(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCDeployServiceRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := h.awsOIDCClientRequest(ctx, req.Region, p, sctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deployDBServiceClient, err := awsoidc.NewDeployServiceClient(ctx, awsClientReq, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databaseAgentMatcherLabels := make(types.Labels, len(req.DatabaseAgentMatcherLabels))
	for _, label := range req.DatabaseAgentMatcherLabels {
		databaseAgentMatcherLabels[label.Name] = utils.Strings{label.Value}
	}

	teleportVersionTag := teleport.Version
	if automaticUpgrades(h.ClusterFeatures) {
		cloudStableVersion, err := automaticupgrades.Version(ctx, "" /* use default version server */)
		if err != nil {
			return "", trace.Wrap(err)
		}

		// cloudStableVersion has vX.Y.Z format, however the container image tag does not include the `v`.
		teleportVersionTag = strings.TrimPrefix(cloudStableVersion, "v")
	}

	deployServiceResp, err := awsoidc.DeployService(ctx, deployDBServiceClient, awsoidc.DeployServiceRequest{
		Region:                        req.Region,
		AccountID:                     req.AccountID,
		SubnetIDs:                     req.SubnetIDs,
		SecurityGroups:                req.SecurityGroups,
		ClusterName:                   req.ClusterName,
		ServiceName:                   req.ServiceName,
		TaskName:                      req.TaskName,
		TaskRoleARN:                   req.TaskRoleARN,
		ProxyServerHostPort:           h.PublicProxyAddr(),
		TeleportClusterName:           h.auth.clusterName,
		TeleportVersionTag:            teleportVersionTag,
		DeploymentMode:                req.DeploymentMode,
		IntegrationName:               awsClientReq.IntegrationName,
		DatabaseResourceMatcherLabels: databaseAgentMatcherLabels,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCDeployServiceResponse{
		ClusterARN:          deployServiceResp.ClusterARN,
		ServiceARN:          deployServiceResp.ServiceARN,
		TaskDefinitionARN:   deployServiceResp.TaskDefinitionARN,
		ServiceDashboardURL: deployServiceResp.ServiceDashboardURL,
	}, nil
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
		fmt.Sprintf("--cluster=%s", clusterName),
		fmt.Sprintf("--name=%s", integrationName),
		fmt.Sprintf("--aws-region=%s", awsRegion),
		fmt.Sprintf("--role=%s", role),
		fmt.Sprintf("--task-role=%s", taskRole),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the browser to complete the database enrollment.",
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

	role := queryParams.Get("role")
	if err := aws.IsValidIAMRoleName(role); err != nil {
		return nil, trace.BadParameter("invalid role %q", role)
	}

	// The script must execute the following command:
	// teleport integration configure eice-iam
	argsList := []string{
		"integration", "configure", "eice-iam",
		fmt.Sprintf("--aws-region=%s", awsRegion),
		fmt.Sprintf("--role=%s", role),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the browser to complete the EC2 enrollment.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}

// awsOIDCListEC2 returns a list of EC2 Instances using the ListEC2 action of the AWS OIDC Integration.
func (h *Handler) awsOIDCListEC2(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCListEC2Request
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := h.awsOIDCClientRequest(ctx, req.Region, p, sctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listEC2Client, err := awsoidc.NewListEC2Client(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := awsoidc.ListEC2(ctx,
		listEC2Client,
		awsoidc.ListEC2Request{
			Integration: awsClientReq.IntegrationName,
			Region:      req.Region,
			NextToken:   req.NextToken,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := ui.MakeServers(h.auth.clusterName, resp.Servers, accessChecker)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCListEC2Response{
		NextToken: resp.NextToken,
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

	awsClientReq, err := h.awsOIDCClientRequest(r.Context(), req.Region, p, sctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listSGClient, err := awsoidc.NewListSecurityGroupsClient(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := awsoidc.ListSecurityGroups(ctx,
		listSGClient,
		awsoidc.ListSecurityGroupsRequest{
			VPCID:     req.VPCID,
			NextToken: req.NextToken,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCListSecurityGroupsResponse{
		NextToken:      resp.NextToken,
		SecurityGroups: resp.SecurityGroups,
	}, nil
}

// awsOIDCListEC2ICE returns a list of EC2 Instance Connect Endpoints using the ListEC2ICE action of the AWS OIDC Integration.
func (h *Handler) awsOIDCListEC2ICE(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCListEC2ICERequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := h.awsOIDCClientRequest(ctx, req.Region, p, sctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listEC2ICEClient, err := awsoidc.NewListEC2ICEClient(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := awsoidc.ListEC2ICE(ctx,
		listEC2ICEClient,
		awsoidc.ListEC2ICERequest{
			Region:    req.Region,
			VPCID:     req.VPCID,
			NextToken: req.NextToken,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCListEC2ICEResponse{
		NextToken: resp.NextToken,
		EC2ICEs:   resp.EC2ICEs,
	}, nil
}

// awsOIDCDeployC2ICE creates an EC2 Instance Connect Endpoint.
func (h *Handler) awsOIDCDeployEC2ICE(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	ctx := r.Context()

	var req ui.AWSOIDCDeployEC2ICERequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	awsClientReq, err := h.awsOIDCClientRequest(ctx, req.Region, p, sctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createEC2ICEClient, err := awsoidc.NewCreateEC2ICEClient(ctx, awsClientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := awsoidc.CreateEC2ICE(ctx,
		createEC2ICEClient,
		awsoidc.CreateEC2ICERequest{
			Cluster:          h.auth.clusterName,
			IntegrationName:  awsClientReq.IntegrationName,
			SubnetID:         req.SubnetID,
			SecurityGroupIDs: req.SecurityGroupIDs,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AWSOIDCDeployEC2ICEResponse{
		Name: resp.Name,
	}, nil
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

	proxyAddr, err := oidc.IssuerFromPublicAddress(h.cfg.PublicProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The script must execute the following command:
	// teleport integration configure awsoidc-idp
	argsList := []string{
		"integration", "configure", "awsoidc-idp",
		fmt.Sprintf("--cluster=%s", clusterName),
		fmt.Sprintf("--name=%s", integrationName),
		fmt.Sprintf("--role=%s", role),
		fmt.Sprintf("--proxy-public-url=%s", proxyAddr),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the browser to use the integration with AWS.",
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

	role := queryParams.Get("role")
	if err := aws.IsValidIAMRoleName(role); err != nil {
		return nil, trace.BadParameter("invalid role %q", role)
	}

	// The script must execute the following command:
	// teleport integration configure listdatabases-iam
	argsList := []string{
		"integration", "configure", "listdatabases-iam",
		fmt.Sprintf("--aws-region=%s", awsRegion),
		fmt.Sprintf("--role=%s", role),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs:   strings.Join(argsList, " "),
		SuccessMessage: "Success! You can now go back to the browser to complete the Database enrollment.",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	_, err = fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}
