/*
Copyright 2023 Gravitational, Inc.
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

package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
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

	awsClientReq, err := h.awsOIDCClientRequest(r.Context(), req.Region, p, sctx, site)
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

	issuer, err := h.issuerFromPublicAddr()
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

	deployServiceResp, err := awsoidc.DeployService(ctx, deployDBServiceClient, awsoidc.DeployServiceRequest{
		Region:                        req.Region,
		AccountID:                     req.AccountID,
		SubnetIDs:                     req.SubnetIDs,
		ClusterName:                   req.ClusterName,
		ServiceName:                   req.ServiceName,
		TaskName:                      req.TaskName,
		TaskRoleARN:                   req.TaskRoleARN,
		ProxyServerHostPort:           h.PublicProxyAddr(),
		TeleportClusterName:           h.auth.clusterName,
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
		return nil, trace.BadParameter("invalid role")
	}

	taskRole := queryParams.Get("taskRole")
	if err := aws.IsValidIAMRoleName(taskRole); err != nil {
		return nil, trace.BadParameter("invalid taskRole")
	}

	// The script must execute the following command:
	// teleport integration configure deployservice-iam
	argsList := []string{
		"integration", "configure", "deployservice-iam",
		fmt.Sprintf(`--cluster="%s"`, clusterName),
		fmt.Sprintf(`--name="%s"`, integrationName),
		fmt.Sprintf(`--aws-region="%s"`, awsRegion),
		fmt.Sprintf(`--role="%s"`, role),
		fmt.Sprintf(`--task-role="%s"`, taskRole),
	}
	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		TeleportArgs: strings.Join(argsList, " "),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	fmt.Fprint(w, script)

	return nil, trace.Wrap(err)
}
