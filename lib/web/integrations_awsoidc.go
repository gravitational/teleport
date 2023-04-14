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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/web/ui"
)

const (
	// awsoidcListDatabases identifies the List Databases action for the AWS OIDC integration
	awsoidcListDatabases = "aws-oidc/list_databases"
)

// IntegrationAWSOIDCTokenGenerator describes the required methods to generate tokens for calling AWS OIDC Integration actions.
type IntegrationAWSOIDCTokenGenerator interface {
	// GenerateAWSOIDCToken generates a token to be used to execute an AWS OIDC Integration action.
	GenerateAWSOIDCToken(ctx context.Context, req types.GenerateAWSOIDCTokenRequest) (string, error)
}

// awsOIDCListDatabases returns a list of databases using the ListDatabases action of the AWS OIDC Integration.
func (h *Handler) awsOIDCListDatabases(ctx context.Context, ig types.Integration, req ui.AWSOIDCListDatabasesRequest, clt IntegrationAWSOIDCTokenGenerator) (*ui.AWSOIDCListDatabasesResponse, error) {
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

	rdsClient, err := awsoidc.NewRDSClient(ctx, awsoidc.RDSClientRequest{
		Token:   token,
		RoleARN: ig.GetAWSOIDCIntegrationSpec().RoleARN,
		Region:  req.Region,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := awsoidc.ListDatabases(ctx,
		rdsClient,
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

	return ui.MakeAWSOIDCListDatabasesResponse(resp), nil
}
