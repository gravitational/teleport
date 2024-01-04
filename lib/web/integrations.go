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
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
)

// integrationsCreate creates an Integration
func (h *Handler) integrationsCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
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
func (h *Handler) integrationsUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
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
func (h *Handler) integrationsDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
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

	return ui.MakeIntegration(ig), nil
}

// integrationsList returns a page of Integrations
func (h *Handler) integrationsList(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
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
