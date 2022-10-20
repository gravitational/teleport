/*
Copyright 2022 Gravitational, Inc.

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

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
)

// databaseIAMPolicyResponse is the response type for databaseGetIAMPolicy.
type databaseIAMPolicyResponse struct {
	Type string               `json:"type"`
	AWS  databaseIAMPolicyAWS `json:"aws"`
}

// databaseIAMPolicyAWS contains IAM policy for AWS hosted databases.
type databaseIAMPolicyAWS struct {
	PolicyDocument string `json:"policy_document"`
}

// databaseGetIAMPolicy returns the required IAM policy for database.
func (h *Handler) databaseGetIAMPolicy(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	databaseName := p.ByName("db")
	if databaseName == "" {
		return nil, trace.BadParameter("missing database name")
	}

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	database, err := fetchDatabase(r.Context(), clt, r, databaseName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case database.IsAWSHosted():
		policy, err := database.GetIAMPolicy()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &databaseIAMPolicyResponse{
			Type: "aws",
			AWS: databaseIAMPolicyAWS{
				PolicyDocument: policy,
			},
		}, nil

	default:
		return nil, trace.BadParameter("IAM policy not supported for database %q", databaseName)
	}
}

// fetchDatabase fetch a database with provided database name.
func fetchDatabase(ctx context.Context, clt resourcesAPIGetter, r *http.Request, databaseName string) (types.Database, error) {
	resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
		Limit:               defaults.MaxIterationLimit,
		ResourceType:        types.KindDatabaseServer,
		PredicateExpression: fmt.Sprintf(`name == "%s"`, databaseName),
		UseSearchAsRoles:    r.URL.Query().Get("searchAsRoles") == "yes",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := types.ResourcesWithLabels(resp.Resources).AsDatabaseServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch len(servers) {
	case 1:
		return servers[0].GetDatabase(), nil
	case 0:
		return nil, trace.BadParameter("database %q not found", databaseName)
	default:
		// Should not happen. Just in case it does happen, return an internal server error.
		return nil, trace.Errorf("%d databases with same name %q are found", len(servers), databaseName)
	}
}
