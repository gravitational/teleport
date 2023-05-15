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
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/web/ui"
)

// integrationsCreate creates an Integration
func (h *Handler) integrationsCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	var req *ui.Integration
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var ig *types.IntegrationV1
	var err error

	switch req.SubKind {
	case types.IntegrationSubKindAWSOIDC:
		ig, err = types.NewIntegrationAWSOIDC(
			types.Metadata{Name: req.Name},
			&types.AWSOIDCIntegrationSpecV1{
				RoleARN: req.AWSOIDC.RoleARN,
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

	return ui.MakeIntegration(storedIntegration), nil
}

// integrationsUpdate updates the Integration based on its name
func (h *Handler) integrationsUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	integrationName := p.ByName("name")
	if integrationName == "" {
		return nil, trace.BadParameter("an integration name is required")
	}

	var req *ui.UpdateIntegrationRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
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

		integration.SetAWSOIDCRoleARN(req.AWSOIDC.RoleARN)
	}

	if _, err := clt.UpdateIntegration(r.Context(), integration); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeIntegration(integration), nil
}

// integrationsDelete removes an Integration based on its name
func (h *Handler) integrationsDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	integrationName := p.ByName("name")
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
func (h *Handler) integrationsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
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

	return ui.MakeIntegration(ig), nil
}

// integrationsList returns a page of Integrations
func (h *Handler) integrationsList(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	values := r.URL.Query()
	limit, err := queryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	startKey := values.Get("startKey")

	igs, nextKey, err := clt.ListIntegrations(r.Context(), int(limit), startKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.IntegrationsListResponse{
		Items:   ui.MakeIntegrations(igs),
		NextKey: nextKey,
	}, nil
}
