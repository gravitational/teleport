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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	accessgraphui "github.com/gravitational/teleport/e/lib/web/ui/access_graph"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/modules"
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
		isGet := r.Method == http.MethodGet
		switch {
		case isStaticFile && !accessGraphSupportsHTTP && isGet:
			h.WithUnauthenticatedHighLimiter(p.getAccessGraphFileFallback)(w, r, params)
		case isStaticFile && isGet:
			h.WithUnauthenticatedHighLimiter(p.getAccessGraphUsingHTTPUnauthenticated)(w, r, params)
		case isQuery && !accessGraphSupportsHTTP && isGet:
			h.WithAuth(p.queryAccessGraph)(w, r, params)
		case isIntegration && isGet:
			h.WithAuth(p.listIntegrations)(w, r, params)
		case !accessGraphSupportsHTTP:
			p.Logger.WarnContext(r.Context(), "Teleport Proxy received a request but the access graph service is not reachable, returning 404")
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
	features := p.h.GetClusterFeatures()
	policy := modules.GetProtoEntitlement(&features, entitlements.Policy)
	if !policy.Enabled {
		return nil, trace.AccessDenied("not authorized to use access graph")
	}
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
		accessgraphv1.QueryRequest_builder{
			Query: query,
		}.Build(),
		grpc.MaxCallRecvMsgSize(maxGRPCAccessGraphMessageSize),
		grpc.MaxCallSendMsgSize(maxGRPCAccessGraphMessageSize),
	)

	usageReport := getTAGResponseReportFromGRPC(resp)
	p.submitUsageReport(usageReport, webCtx)

	return resp, trace.Wrap(err)
}

// demoModePaths are paths accessible in TAG with demo mode
var demoModePaths = map[string]struct{}{
	"/enterprise/accessgraph/graph/tester/teleport/role/v1": {},
}

// canUseAccessGraph is used to check and set values if policy and/or demo mode is enabled for requests
// that access resources in tag. It takes a path which is checked to exist in the valid demo mode paths
// If Policy is enabled, proceed as usual. Otherwise, check if Demo Mode is enabled and update
// the Demo Mode state on subsequent requests to skip redundant checks.
func (p *Plugin) canUseAccessGraph(ctx context.Context, requestedPath string) (bool, error) {
	features := p.h.GetClusterFeatures()
	policyEnabled := modules.GetProtoEntitlement(&features, entitlements.Policy).Enabled

	if policyEnabled {
		return true, nil
	}
	// if the path is not a demo path and policy isnt enabled, reject.
	if _, exists := demoModePaths[requestedPath]; !exists {
		return false, nil
	}
	demoEnabled := false
	p.mu.Lock()
	if p.AccessGraph != nil {
		demoEnabled = p.AccessGraph.DemoMode
	}
	p.mu.Unlock()

	// if either of these are true, we can shortcut any extra checks
	if demoEnabled {
		return true, nil
	}

	// These endpoints should generally not be reached unless the cluster has Policy or Demo Mode enabled.
	// Before making API requests, we ensure either is enabled. If not, we can check if the value of policy or demo
	// have been changed, and set them accordingly.
	clt, err := p.getAuthClient()
	if err != nil {
		return false, trace.Wrap(err)
	}

	accessGraphSettings, err := clt.ClusterConfigClient().
		GetAccessGraphSettings(
			ctx,
			&clusterconfigpb.GetAccessGraphSettingsRequest{},
		)
	if err != nil {
		return false, trace.Wrap(err)
	}

	p.mu.Lock()
	if p.AccessGraph != nil {
		p.AccessGraph.DemoMode = accessGraphSettings.GetSpec().GetDemoMode() == clusterconfigpb.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_ENABLED
		demoEnabled = p.AccessGraph.DemoMode
	}
	p.mu.Unlock()
	return demoEnabled, nil
}

// getAccessGraphUsingHTTPWithAuth is a handler for the /v1/accessgraph/:path which
// requires authentication to access the access graph.
func (p *Plugin) getAccessGraphUsingHTTPWithAuth(w http.ResponseWriter, r *http.Request, params httprouter.Params, webCtx *web.SessionContext) (any, error) {
	accessChecker, err := webCtx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	canAccess, err := p.canUseAccessGraph(r.Context(), r.URL.Path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !canAccess {
		return nil, trace.AccessDenied("not authorized to use access graph")
	}

	var requiredVerb string
	switch r.Method {
	case http.MethodGet, http.MethodOptions, http.MethodHead:
		requiredVerb = types.VerbRead
	case http.MethodPost:
		requiredVerb = types.VerbCreate
	case http.MethodPut, http.MethodPatch:
		requiredVerb = types.VerbUpdate
	case http.MethodDelete:
		requiredVerb = types.VerbDelete
	default:
		return nil, trace.BadParameter("unsupported HTTP method %q", r.Method)
	}

	if err := accessChecker.CheckAccessToRule(&services.Context{},
		defaults.Namespace,
		types.KindAccessGraph,
		requiredVerb); err != nil {
		return nil, trace.WrapWithMessage(err, "not allowed to read the access graph")
	}

	// Set the username header to the authenticated user.
	r.Header.Set(teleport.XTeleportUsernameHeader, webCtx.GetUser())

	respRec := httplib.NewResponseStatusRecorder(w)
	rsp, err := p.getAccessGraphUsingHTTPUnauthenticated(respRec, r, params)

	usageReport := getTAGResponseReportFromHTTPHeaders(w.Header(), respRec.Status())
	p.submitUsageReport(usageReport, webCtx)
	return rsp, err
}

// accessGraphCertHandler returns an http.Handler that serves Access Graph API
// requests authenticated via mTLS client certificate (RouteToApp.AccessGraph=true).
func (p *Plugin) accessGraphCertHandler(prefix string) http.Handler {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sCtx, err := p.h.AuthenticateReqForAccessGraphAPI(r)
		if err != nil {
			trace.WriteError(w, err)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/v1/") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/v1")
		}

		if !strings.HasPrefix(r.URL.Path, prefix) {
			trace.WriteError(w, trace.BadParameter("invalid path for access graph API"))
			return
		}

		// Build params with "path" key so getAccessGraphUsingHTTPUnauthenticated
		// (called inside serveAccessGraphHTTP) can rewrite the URL correctly.
		params := httprouter.Params{
			httprouter.Param{
				Key:   "path",
				Value: r.URL.Path[len(prefix)-1:],
			},
		}

		_, err = p.getAccessGraphUsingHTTPWithAuth(w, r, params, sCtx)
		if err != nil {
			trace.WriteError(w, err)
			return
		}
	})
}

// getAccessGraphUsingHTTPUnauthenticated is a handler for the /v1/accessgraph/:path which
// does not require authentication to access the access graph.
func (p *Plugin) getAccessGraphUsingHTTPUnauthenticated(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
	forwarder := p.getAccessGraphHTTPForwarder()
	if forwarder == nil {
		return nil, trace.NotFound("access graph service is not available")
	}
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
	resp, err := agClt.GetFile(ctx, accessgraphv1.GetFileRequest_builder{
		Filepath: filePath,
	}.Build(),
		grpc.MaxCallRecvMsgSize(maxGRPCAccessGraphMessageSize),
		grpc.MaxCallSendMsgSize(maxGRPCAccessGraphMessageSize),
	)
	if err != nil {
		// If access graph is not enabled in the Auth server, return 404 instead of 500.
		if trace.IsNotImplemented(err) {
			return nil, trace.NotFound("file %q is not found", filePath)
		}

		p.Logger.ErrorContext(ctx, "Failed to get file", "file", filePath, "error", err)
		return nil, trace.Wrap(err)
	}

	ext := filepath.Ext(filePath)
	extType := mime.TypeByExtension(ext)
	w.Header().Set("Content-Type", extType)

	// Send the file contents to the client
	w.Write(resp.GetData())

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
		TotalNodes: int64(len(rsp.GetNodes())),
		TotalEdges: int64(len(rsp.GetEdges())),
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
			p.Logger.WarnContext(ctx, "Failed to get auth client", "error", err)
			return
		}
		if err := authClient.SubmitUsageEvent(ctx, usageEventReq); err != nil {
			p.Logger.WarnContext(ctx, "Failed to emit TAG usage event", "error", err)
		}
	}()
}

// listIntegrations is a handler to list all the integrations that are available
// and enabled for the access graph.
func (p *Plugin) listIntegrations(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, webCtx *web.SessionContext) (any, error) {
	features := p.h.GetClusterFeatures()
	policy := modules.GetProtoEntitlement(&features, entitlements.Policy)
	if !policy.Enabled {
		return nil, trace.AccessDenied("not authorized to use access graph")
	}

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
		rsp, err := pluginsC.ListPlugins(ctx, pluginsv1.ListPluginsRequest_builder{
			PageSize:    defaults.DefaultChunkSize,
			StartKey:    nextPage,
			WithSecrets: false, /* don't return secrets */
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, plugin := range rsp.GetPlugins() {
			switch plugin.GetType() {
			case types.PluginTypeGitlab, types.PluginTypeOkta, types.PluginTypeEntraID, types.PluginTypeNetIQ, types.PluginTypeGithub:
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

// getAccessGraphSettings is the handler for GET /v1/enterprise/accessgraphsettings.
func (p *Plugin) getAccessGraphSettings(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := p.getAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessGraphSettings, err := clt.ClusterConfigClient().
		GetAccessGraphSettings(
			r.Context(),
			&clusterconfigpb.GetAccessGraphSettingsRequest{},
		)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessgraphui.FromProtoAccessGraphSettings(accessGraphSettings, p.accessGraphForwarder != nil /* httpReady */), nil
}

// updateAccessGraphSettings is the handler for POST /v1/enterprise/accessgraphsettings.
func (p *Plugin) updateAccessGraphSettings(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req accessgraphui.AccessGraphSettings
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterConfigClient := clt.ClusterConfigClient()

	getAccessGraphSettings := func() (*clusterconfigpb.AccessGraphSettings, error) {
		// Remove the MFA resp from the context before getting the access list.
		// Otherwise, it will be consumed before the Upsert which actually
		// requires the MFA.
		// TODO(Joerger): Explicitly provide MFA response only where it is
		// needed instead of removing it like this.
		accessGraphSettings, err := clusterConfigClient.
			GetAccessGraphSettings(
				mfa.ContextWithMFAResponse(r.Context(), nil),
				&clusterconfigpb.GetAccessGraphSettingsRequest{},
			)
		return accessGraphSettings, trace.Wrap(err)
	}

	accessGraphSettings, err := getAccessGraphSettings()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if this request is to update access graph demo mode settings and they do not have the entitlement to update access graph demo mode, reject
	canEnableDemoMode := false
	demoMode, ok := p.h.GetClusterFeatures().Entitlements[string(entitlements.AccessGraphDemoMode)]
	if ok && demoMode.Enabled {
		canEnableDemoMode = true
	}
	if accessGraphSettings.GetSpec().GetDemoMode() != clusterconfigpb.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_ENABLED && req.EnableDemoMode && !canEnableDemoMode {
		return nil, trace.AccessDenied("You do not have permission to enable Access Graph Demo Mode.")
	}
	// if enable demo mode, check and rebuild the accessgraphhttptransport
	if req.EnableDemoMode && p.accessGraphForwarder == nil {
		if err := p.checkAndBuildAccessGraphHTTPTransport(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	accessGraphSettings, err = clusterConfigClient.UpdateAccessGraphSettings(
		r.Context(),
		clusterconfigpb.UpdateAccessGraphSettingsRequest_builder{
			AccessGraphSettings: req.UpdateProto(accessGraphSettings),
		}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessgraphui.FromProtoAccessGraphSettings(accessGraphSettings, p.accessGraphForwarder != nil /* httpReady */), nil
}
