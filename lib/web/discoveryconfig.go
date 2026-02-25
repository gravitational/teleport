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
	"encoding/json"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
)

// discoveryconfigCreate creates a DiscoveryConfig
func (h *Handler) discoveryconfigCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
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

	clt, err := sctx.GetUserClient(r.Context(), cluster)
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
func (h *Handler) discoveryconfigUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
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

	clt, err := sctx.GetUserClient(r.Context(), cluster)
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
func (h *Handler) discoveryconfigDelete(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	discoveryconfigName := p.ByName("name")
	if discoveryconfigName == "" {
		return nil, trace.BadParameter("a discoveryconfig name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.DiscoveryConfigClient().DeleteDiscoveryConfig(r.Context(), discoveryconfigName); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// discoveryconfigGet returns a DiscoveryConfig based on its name
func (h *Handler) discoveryconfigGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	discoveryconfigName := p.ByName("name")
	if discoveryconfigName == "" {
		return nil, trace.BadParameter("as discoveryconfig name is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
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
func (h *Handler) discoveryconfigList(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
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

// ssmRunEntry is a lightweight representation of an SSMRun event for JSON
// serialisation to the ACR classification prompt.
type ssmRunEntry struct {
	AccountID     string `json:"account_id"`
	Region        string `json:"region"`
	InstanceID    string `json:"instance_id"`
	Status        string `json:"status"`
	ExitCode      int64  `json:"exit_code"`
	CommandID     string `json:"command_id"`
	InvocationURL string `json:"invocation_url"`
	Stdout        string `json:"stdout"`
	Stderr        string `json:"stderr"`
}

func (h *Handler) discoveryLog(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	ctx := r.Context()

	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	events, _, err := clt.SearchEvents(ctx, libevents.SearchEventsRequest{
		From:       time.Now().Add(-24 * time.Hour),
		To:         time.Now(),
		EventTypes: []string{libevents.SSMRunEvent},
		Limit:      1000,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Build a flat list of SSM run entries.
	var entries []ssmRunEntry
	for _, event := range events {
		ssmRun, ok := event.(*apievents.SSMRun)
		if !ok {
			continue
		}
		entries = append(entries, ssmRunEntry{
			AccountID:     ssmRun.AccountID,
			Region:        ssmRun.Region,
			InstanceID:    ssmRun.InstanceID,
			Status:        ssmRun.Status,
			ExitCode:      ssmRun.ExitCode,
			CommandID:     ssmRun.CommandID,
			InvocationURL: ssmRun.InvocationURL,
			Stdout:        ssmRun.StandardOutput,
			Stderr:        ssmRun.StandardError,
		})
	}

	// Hackathon
	if h.acrService == nil {
		return nil, trace.BadParameter("ACR service is not configured (OPENAI_API_KEY not set)")
	}

	auditJSON, err := json.Marshal(entries)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := h.acrService.Classify(ctx, string(auditJSON))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return result, nil
}
