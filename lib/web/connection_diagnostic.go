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
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/client/conntest"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/web/ui"
)

// getConnectionDiagnostic returns a connection diagnostic connection diagnostics.
func (h *Handler) getConnectionDiagnostic(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
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
func (h *Handler) diagnoseConnection(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
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
