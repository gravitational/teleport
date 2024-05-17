/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

import (
	"context"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	accessgraphui "github.com/gravitational/teleport/e/lib/web/ui/access_graph"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web"
)

const (
	maxGRPCAccessGraphMessageSize = 20 * 1024 * 1024 // 20MB
	// accessGraphStaticPathPrefix is the prefix for static files served by the access graph.
	// We use it to distinguish between static files and the query endpoint since
	// the static files aren't authenticated.
	accessGraphStaticPathPrefix = "/static"
	// accessGraphQueryPath is the path to the access graph query endpoint.
	accessGraphQueryPath = "/query"
	// accessGraphIntegrationPath is the path to the access graph integration endpoint.
	// This endpoint is used to retrieve all the integrations that are available and enabled for the access graph.
	accessGraphIntegrationPath = "/integrations"
)

func (p *Plugin) accessGraphHandler(h *web.Handler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		path := params.ByName("path")
		accessGraphSupportsHTTP := p.getAccessGraphHTTPForwarder() != nil
		isStaticFile := strings.HasPrefix(path, accessGraphStaticPathPrefix)
		isQuery := strings.HasPrefix(path, accessGraphQueryPath)
		isIntegration := strings.EqualFold(path, accessGraphIntegrationPath)
		switch {
		case isStaticFile && !accessGraphSupportsHTTP:
			h.WithUnauthenticatedHighLimiter(p.getAccessGraphFileFallback)(w, r, params)
		case isStaticFile:
			h.WithUnauthenticatedHighLimiter(p.getAccessGraphUsingHTTPUnauthenticated)(w, r, params)
		case isQuery && !accessGraphSupportsHTTP:
			h.WithAuth(p.queryAccessGraph)(w, r, params)
		case isIntegration:
			h.WithAuth(p.listIntegrations)(w, r, params)
		case !accessGraphSupportsHTTP:
			p.Log.Warnf("Teleport Proxy received a request but the access graph service is not reachable. Returning 404.")
			// If the access graph service is not enabled, return 404.
			w.WriteHeader(http.StatusNotFound)
			return

		default:
			// use the new http router.
			h.WithAuth(p.getAccessGraphUsingHTTPWithAuth)(w, r, params)
		}
	}
}

// queryAccessGraph is a handler for the /v1/accessgraph/query endpoint.
// It queries the access graph and returns the results.
func (p *Plugin) queryAccessGraph(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, webCtx *web.SessionContext) (any, error) {
	query := r.URL.Query().Get("query")
	if query == "" {
		return nil, trace.BadParameter("query parameter is required")
	}

	cl, err := webCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	agClt := cl.AccessGraphClient()
	if agClt == nil {
		return nil, trace.NotFound("access graph client is not configured")
	}

	// Skip the RBAC check. Auth server will perform it.
	ctx := r.Context()

	resp, err := agClt.Query(
		ctx,
		&accessgraphv1.QueryRequest{
			Query: query,
		},
		grpc.MaxCallRecvMsgSize(maxGRPCAccessGraphMessageSize),
		grpc.MaxCallSendMsgSize(maxGRPCAccessGraphMessageSize),
	)

	usageReport := getTAGResponseReportFromGRPC(resp)
	p.submitUsageReport(usageReport, webCtx)

	return resp, trace.Wrap(err)
}

// getAccessGraphUsingHTTPWithAuth is a handler for the /v1/accessgraph/:path which
// requires authentication to access the access graph.
func (p *Plugin) getAccessGraphUsingHTTPWithAuth(w http.ResponseWriter, r *http.Request, params httprouter.Params, webCtx *web.SessionContext) (any, error) {
	accessChecker, err := webCtx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := accessChecker.CheckAccessToRule(&services.Context{},
		defaults.Namespace,
		types.KindAccessGraph,
		types.VerbRead); err != nil {
		return nil, trace.WrapWithMessage(err, "not allowed to read the access graph")
	}
	respRec := httplib.NewResponseStatusRecorder(w)
	rsp, err := p.getAccessGraphUsingHTTPUnauthenticated(respRec, r, params)

	usageReport := getTAGResponseReportFromHTTPHeaders(w.Header(), respRec.Status())
	p.submitUsageReport(usageReport, webCtx)
	return rsp, err
}

// getAccessGraphUsingHTTPUnauthenticated is a handler for the /v1/accessgraph/:path which
// does not require authentication to access the access graph.
func (p *Plugin) getAccessGraphUsingHTTPUnauthenticated(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	forwarder := p.getAccessGraphHTTPForwarder()
	urlPath := params.ByName("path")
	r = r.Clone(r.Context())
	r.URL.Scheme = "https"
	r.URL.Host, r.Host = p.AccessGraph.Addr, p.AccessGraph.Addr
	r.URL.Path, r.RequestURI = urlPath, "" /* we reset RequestURI and forwarder will build it from r.URL */
	forwarder.ServeHTTP(w, r)
	return nil, nil
}

// getAccessGraphFile is a handler for the /v1/accessgraph/file/:file endpoint.
// It returns web assets from TAG.
func (p *Plugin) getAccessGraphFileFallback(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	filePath := params.ByName("path")
	filePath = strings.TrimPrefix(filePath, accessGraphStaticPathPrefix)

	acl, err := p.getAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	agClt := acl.AccessGraphClient()
	if agClt == nil {
		return nil, trace.NotFound("access graph client is not configured")
	}
	ctx := r.Context()
	resp, err := agClt.GetFile(ctx, &accessgraphv1.GetFileRequest{
		Filepath: filePath,
	},
		grpc.MaxCallRecvMsgSize(maxGRPCAccessGraphMessageSize),
		grpc.MaxCallSendMsgSize(maxGRPCAccessGraphMessageSize),
	)
	if err != nil {
		// If access graph is not enabled in the Auth server, return 404 instead of 500.
		if trace.IsNotImplemented(err) {
			return nil, trace.NotFound("file %q is not found", filePath)
		}

		p.Log.Errorf("Failed to get file %q: %v", filePath, err)
		return nil, trace.Wrap(err)
	}

	ext := filepath.Ext(filePath)
	extType := mime.TypeByExtension(ext)
	w.Header().Set("Content-Type", extType)

	// Send the file contents to the client
	w.Write(resp.Data)

	// Return nil to prevent the content type to be set to application/json
	return nil, nil
}

// getTAGResponseReportFromGRPC returns a usage event report for the TAG query response.
// If the response is nil, it returns a report for an empty response with
// IsSuccess=false.
// This function is used to report TAG usage events to the auth server.
func getTAGResponseReportFromGRPC(rsp *accessgraphv1.QueryResponse) *usageeventsv1.TAGExecuteQueryEvent {
	if rsp == nil {
		return &usageeventsv1.TAGExecuteQueryEvent{
			TotalNodes: 0,
			TotalEdges: 0,
			IsSuccess:  false,
		}
	}
	return &usageeventsv1.TAGExecuteQueryEvent{
		TotalNodes: int64(len(rsp.Nodes)),
		TotalEdges: int64(len(rsp.Edges)),
		IsSuccess:  true,
	}
}

// getTAGResponseReport returns a usage event report for the TAG query response.
// If the response is nil, it returns a report for an empty response with
// IsSuccess=false.
// This function is used to report TAG usage events to the auth server.
func getTAGResponseReportFromHTTPHeaders(headers http.Header, statusCode int) *usageeventsv1.TAGExecuteQueryEvent {
	if statusCode != http.StatusOK {
		return &usageeventsv1.TAGExecuteQueryEvent{
			TotalNodes: 0,
			TotalEdges: 0,
			IsSuccess:  false,
		}
	}
	var nodesCount int64
	var edgesCount int64
	nodes := headers.Get("X-Access-Graph-Nodes")
	edges := headers.Get("X-Access-Graph-Edges")
	if nodes != "" {
		nodesCount, _ = strconv.ParseInt(nodes, 10, 64)
	}
	if edges != "" {
		edgesCount, _ = strconv.ParseInt(edges, 10, 64)
	}
	return &usageeventsv1.TAGExecuteQueryEvent{
		TotalNodes: nodesCount,
		TotalEdges: edgesCount,
		IsSuccess:  true,
	}
}

// getAccessGraphHTTPForwarder returns the access graph forwarder
// used to forward HTTP requests to the access graph service if it is enabled.
func (p *Plugin) getAccessGraphHTTPForwarder() *reverseproxy.Forwarder {
	p.accessGraphForwarderMu.Lock()
	defer p.accessGraphForwarderMu.Unlock()
	return p.accessGraphForwarder
}

func (p *Plugin) submitUsageReport(usageReport *usageeventsv1.TAGExecuteQueryEvent, webCtx *web.SessionContext) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		usageEventReq := &proto.SubmitUsageEventRequest{
			Event: &usageeventsv1.UsageEventOneOf{
				Event: &usageeventsv1.UsageEventOneOf_TagExecuteQuery{
					TagExecuteQuery: usageReport,
				},
			},
		}
		authClient, err := webCtx.GetClient()
		if err != nil {
			p.Log.WithError(err).Warn("Failed to get auth client")
			return
		}
		if err := authClient.SubmitUsageEvent(ctx, usageEventReq); err != nil {
			p.Log.WithError(err).Warn("Failed to emit TAG usage event")
		}
	}()
}

// listIntegrations is a handler to list all the integrations that are available
// and enabled for the access graph.
func (p *Plugin) listIntegrations(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, webCtx *web.SessionContext) (any, error) {
	cl, err := webCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	integrations, err := listAllIntegrations(r.Context(), cl)
	if trace.IsAccessDenied(err) {
		return nil, trace.AccessDenied("not allowed to list integrations")
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	discoveryConfigs, err := listAllDiscoveryConfigs(r.Context(), cl)
	if trace.IsAccessDenied(err) {
		return nil, trace.AccessDenied("not allowed to list discovery configs")
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	accessGraphPlugins, err := listAllAccessGraphPlugins(r.Context(), cl)
	if trace.IsAccessDenied(err) {
		return nil, trace.AccessDenied("not allowed to list access graph plugins")
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := accessgraphui.MakeListIntegrationResponse(discoveryConfigs, integrations, accessGraphPlugins)

	return resp, nil
}

func listAllIntegrations(ctx context.Context, client authclient.ClientI) ([]types.Integration, error) {
	var (
		nextPage        string
		page            []types.Integration
		err             error
		allIntegrations []types.Integration
	)
	for {
		page, nextPage, err = client.ListIntegrations(ctx, 0, nextPage)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		allIntegrations = append(allIntegrations, page...)
		if nextPage == "" {
			break
		}

	}
	return allIntegrations, nil
}

func listAllDiscoveryConfigs(ctx context.Context, client authclient.ClientI) ([]*discoveryconfig.DiscoveryConfig, error) {
	dC := client.DiscoveryConfigClient()
	var (
		nextPage        string
		page            []*discoveryconfig.DiscoveryConfig
		err             error
		allIntegrations []*discoveryconfig.DiscoveryConfig
	)
	for {
		page, nextPage, err = dC.ListDiscoveryConfigs(ctx, 0, nextPage)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, dc := range page {
			if dc.Spec.AccessGraph == nil {
				continue
			}
			allIntegrations = append(allIntegrations, dc)
		}

		if nextPage == "" {
			break
		}

	}
	return allIntegrations, nil

}

func listAllAccessGraphPlugins(ctx context.Context, client authclient.ClientI) ([]*types.PluginV1, error) {
	var (
		nextPage   string
		allPlugins []*types.PluginV1
	)
	pluginsC := client.PluginsClient()
	for {
		rsp, err := pluginsC.ListPlugins(ctx, &pluginsv1.ListPluginsRequest{
			PageSize:    apidefaults.DefaultChunkSize,
			StartKey:    nextPage,
			WithSecrets: false, /* don't return secrets */
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, plugin := range rsp.GetPlugins() {
			switch plugin.GetType() {
			case types.PluginTypeGitlab, types.PluginTypeOkta:
				allPlugins = append(allPlugins, plugin)
			}
		}
		if rsp.GetNextKey() == "" {
			break
		}
		nextPage = rsp.GetNextKey()

	}
	return allPlugins, nil
}
