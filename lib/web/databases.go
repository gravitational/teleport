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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
	"github.com/gravitational/teleport/lib/web/ui"
)

// createDatabaseRequest contains the necessary basic information to create a database.
// Database here is the database resource, containing information to a real database (protocol, uri)
type createDatabaseRequest struct {
	Name     string     `json:"name,omitempty"`
	Labels   []ui.Label `json:"labels,omitempty"`
	Protocol string     `json:"protocol,omitempty"`
	URI      string     `json:"uri,omitempty"`
}

func (r *createDatabaseRequest) checkAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing database name")
	}

	if r.Protocol == "" {
		return trace.BadParameter("missing protocol")
	}

	if r.URI == "" {
		return trace.BadParameter("missing uri")
	}

	return nil
}

// handleDatabaseCreate creates a database's metadata.
func (h *Handler) handleDatabaseCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	var req *createDatabaseRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	labels := make(map[string]string)
	for _, label := range req.Labels {
		labels[label.Name] = label.Value
	}

	database, err := types.NewDatabaseV3(
		types.Metadata{
			Name:   req.Name,
			Labels: labels,
		},
		types.DatabaseSpecV3{
			Protocol: req.Protocol,
			URI:      req.URI,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.CreateDatabase(r.Context(), database); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeDatabase(database, nil /* dbUsers */, nil /* dbNames */), nil
}

// updateDatabaseRequest contains some updatable fields of a database resource.
type updateDatabaseRequest struct {
	CACert string `json:"ca_cert,omitempty"`
}

func (r *updateDatabaseRequest) checkAndSetDefaults() error {
	if r.CACert == "" {
		return trace.BadParameter("missing CA certificate data")
	}

	if _, err := tlsutils.ParseCertificatePEM([]byte(r.CACert)); err != nil {
		return trace.BadParameter("could not parse provided CA as X.509 PEM certificate")
	}

	return nil
}

// handleDatabaseUpdate updates the database
func (h *Handler) handleDatabaseUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	databaseName := p.ByName("database")
	if databaseName == "" {
		return nil, trace.BadParameter("a database name is required")
	}

	var req *updateDatabaseRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	database, err := clt.GetDatabase(r.Context(), databaseName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	database.SetCA(req.CACert)

	if err := clt.UpdateDatabase(r.Context(), database); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeDatabase(database, nil /* dbUsers */, nil /* dbNames */), nil
}

// databaseIAMPolicyResponse is the response type for handleDatabaseGetIAMPolicy.
type databaseIAMPolicyResponse struct {
	// Type is the type of the IAM policy.
	Type string `json:"type"`
	// AWS contains the IAM policy for AWS-hosted databases.
	AWS *databaseIAMPolicyAWS `json:"aws,omitempty"`
}

// databaseIAMPolicyAWS contains IAM policy for AWS-hosted databases.
type databaseIAMPolicyAWS struct {
	// PolicyDocument is the AWS IAM policy document.
	PolicyDocument string `json:"policy_document"`
	// Placeholders are placeholders found in the policy document.
	Placeholders []string `json:"placeholders,omitempty"`
}

// handleDatabaseGetIAMPolicy returns the required IAM policy for database.
func (h *Handler) handleDatabaseGetIAMPolicy(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	databaseName := p.ByName("database")
	if databaseName == "" {
		return nil, trace.BadParameter("missing database name")
	}

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	database, err := fetchDatabaseWithName(r.Context(), clt, r, databaseName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case database.IsAWSHosted():
		policy, placeholders, err := dbiam.GetAWSPolicyDocument(database)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		policyJSON, err := json.Marshal(policy)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &databaseIAMPolicyResponse{
			Type: "aws",
			AWS: &databaseIAMPolicyAWS{
				PolicyDocument: string(policyJSON),
				Placeholders:   placeholders,
			},
		}, nil

	default:
		return nil, trace.BadParameter("IAM policy not supported for database type %q", database.GetType())
	}
}

// fetchDatabaseWithName fetch a database with provided database name.
func fetchDatabaseWithName(ctx context.Context, clt resourcesAPIGetter, r *http.Request, databaseName string) (types.Database, error) {
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
	case 0:
		return nil, trace.NotFound("database %q not found", databaseName)
	default:
		return servers[0].GetDatabase(), nil
	}
}
