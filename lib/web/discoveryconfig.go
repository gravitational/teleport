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

	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
)

// discoveryconfigCreate creates a DiscoveryConfig
func (h *Handler) discoveryconfigCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	var req ui.DiscoveryConfig
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{
			Name: req.Name,
		},
		discoveryconfig.Spec{
			DiscoveryGroup: req.DiscoveryGroup,
			AWS:            req.AWS,
			Azure:          req.Azure,
			GCP:            req.GCP,
			Kube:           req.Kube,
			AccessGraph:    req.AccessGraph,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	storedDiscoveryConfig, err := clt.DiscoveryConfigClient().CreateDiscoveryConfig(r.Context(), dc)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("failed to create DiscoveryConfig (%q already exists), please use another name", req.Name)
		}
		return nil, trace.Wrap(err)
	}

	return ui.MakeDiscoveryConfig(storedDiscoveryConfig), nil
}

// discoveryconfigUpdate updates the DiscoveryConfig based on its name
func (h *Handler) discoveryconfigUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	dcName := p.ByName("name")
	if dcName == "" {
		return nil, trace.BadParameter("a discoveryconfig name is required")
	}

	var req *ui.UpdateDiscoveryConfigRequest
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

	dc, err := clt.DiscoveryConfigClient().GetDiscoveryConfig(r.Context(), dcName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dc.Spec.DiscoveryGroup = req.DiscoveryGroup
	dc.Spec.AWS = req.AWS
	dc.Spec.Azure = req.Azure
	dc.Spec.GCP = req.GCP
	dc.Spec.Kube = req.Kube
	dc.Spec.AccessGraph = req.AccessGraph

	dc, err = clt.DiscoveryConfigClient().UpdateDiscoveryConfig(r.Context(), dc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeDiscoveryConfig(dc), nil
}

// discoveryconfigDelete removes a DiscoveryConfig based on its name
func (h *Handler) discoveryconfigDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	discoveryconfigName := p.ByName("name")
	if discoveryconfigName == "" {
		return nil, trace.BadParameter("a discoveryconfig name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.DiscoveryConfigClient().DeleteDiscoveryConfig(r.Context(), discoveryconfigName); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// discoveryconfigGet returns a DiscoveryConfig based on its name
func (h *Handler) discoveryconfigGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	discoveryconfigName := p.ByName("name")
	if discoveryconfigName == "" {
		return nil, trace.BadParameter("as discoveryconfig name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dc, err := clt.DiscoveryConfigClient().GetDiscoveryConfig(r.Context(), discoveryconfigName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeDiscoveryConfig(dc), nil
}

// discoveryconfigList returns a page of DiscoveryConfigs
func (h *Handler) discoveryconfigList(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
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

	dcs, nextKey, err := clt.DiscoveryConfigClient().ListDiscoveryConfigs(r.Context(), int(limit), startKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.DiscoveryConfigsListResponse{
		Items:   ui.MakeDiscoveryConfigs(dcs),
		NextKey: nextKey,
	}, nil
}
