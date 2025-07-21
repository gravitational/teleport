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

	"github.com/gravitational/teleport/lib/client/conntest"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web/ui"
)

// getConnectionDiagnostic returns a connection diagnostic connection diagnostics.
func (h *Handler) getConnectionDiagnostic(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectionID := p.ByName("connectionid")
	connectionDiagnostic, err := clt.GetConnectionDiagnostic(r.Context(), connectionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.ConnectionDiagnostic{
		ID:      connectionDiagnostic.GetName(),
		Success: connectionDiagnostic.IsSuccess(),
		Message: connectionDiagnostic.GetMessage(),
		Traces:  ui.ConnectionDiagnosticTraceUIFromTypes(connectionDiagnostic.GetTraces()),
	}, nil
}

// diagnoseConnection executes and returns a connection diagnostic.
func (h *Handler) diagnoseConnection(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	req := conntest.TestConnectionRequest{}
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	userClt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxySettings, err := h.cfg.ProxySettings.GetProxySettings(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectionTesterConfig := conntest.ConnectionTesterConfig{
		ResourceKind:              req.ResourceKind,
		UserClient:                userClt,
		ProxyHostPort:             h.ProxyHostPort(),
		PublicProxyAddr:           h.PublicProxyAddr(),
		KubernetesPublicProxyAddr: h.kubeProxyHostPort(),
		TLSRoutingEnabled:         proxySettings.TLSRoutingEnabled,
	}

	tester, err := conntest.ConnectionTesterForKind(connectionTesterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectionDiagnostic, err := tester.TestConnection(r.Context(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.ConnectionDiagnostic{
		ID:      connectionDiagnostic.GetName(),
		Success: connectionDiagnostic.IsSuccess(),
		Message: connectionDiagnostic.GetMessage(),
		Traces:  ui.ConnectionDiagnosticTraceUIFromTypes(connectionDiagnostic.GetTraces()),
	}, nil
}
