/*
Copyright 2021 Gravitational, Inc.

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
	"strings"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
	"golang.org/x/mod/semver"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// checkAccessToRegisteredResource checks if calling user has access to at least one registered resource.
func (h *Handler) checkAccessToRegisteredResource(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of resources.
	clt, err := c.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceKinds := []string{types.KindNode, types.KindDatabaseServer, types.KindAppServer, types.KindKubeService, types.KindWindowsDesktop}
	for _, kind := range resourceKinds {
		res, err := clt.ListResources(r.Context(), proto.ListResourcesRequest{
			ResourceType: kind,
			Limit:        1,
		})

		if err != nil {
			// Access denied error is returned when user does not have permissions
			// to read/list a resource kind which can be ignored as this function is not
			// about checking if user has the right perms.
			if trace.IsAccessDenied(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}

		if len(res.Resources) > 0 {
			return checkAccessToRegisteredResourceResponse{
				HasResource: true,
			}, nil
		}
	}

	return checkAccessToRegisteredResourceResponse{
		HasResource: false,
	}, nil
}

func (h *Handler) getRolesHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getRoles(clt)
}

func getRoles(clt resourcesAPIGetter) ([]ui.ResourceItem, error) {
	roles, err := clt.GetRoles(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewRoles(roles)
}

func (h *Handler) deleteRole(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roleName := params.ByName("name")
	if err := clt.DeleteRole(r.Context(), roleName); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

func (h *Handler) upsertRoleHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req ui.ResourceItem
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return upsertRole(r.Context(), clt, req.Content, r.Method)
}

func upsertRole(ctx context.Context, clt resourcesAPIGetter, content, httpMethod string) (*ui.ResourceItem, error) {
	extractedRes, err := ExtractResourceAndValidate(content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if extractedRes.Kind != types.KindRole {
		return nil, trace.BadParameter("resource kind %q is invalid", extractedRes.Kind)
	}

	_, err = clt.GetRole(ctx, extractedRes.Metadata.Name)
	if err := CheckResourceUpsertableByError(err, httpMethod, extractedRes.Metadata.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	role, err := services.UnmarshalRole(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.UpsertRole(ctx, role); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewResourceItem(role)
}

func (h *Handler) getGithubConnectorsHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getGithubConnectors(r.Context(), clt)
}

func getGithubConnectors(ctx context.Context, clt resourcesAPIGetter) ([]ui.ResourceItem, error) {
	connectors, err := clt.GetGithubConnectors(ctx, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewGithubConnectors(connectors)
}

func (h *Handler) deleteGithubConnector(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectorName := params.ByName("name")
	if err := clt.DeleteGithubConnector(r.Context(), connectorName); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

func (h *Handler) upsertGithubConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req ui.ResourceItem
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return upsertGithubConnector(r.Context(), clt, req.Content, r.Method)
}

func upsertGithubConnector(ctx context.Context, clt resourcesAPIGetter, content, httpMethod string) (*ui.ResourceItem, error) {
	extractedRes, err := ExtractResourceAndValidate(content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if extractedRes.Kind != types.KindGithubConnector {
		return nil, trace.BadParameter("resource kind %q is invalid", extractedRes.Kind)
	}

	_, err = clt.GetGithubConnector(ctx, extractedRes.Metadata.Name, false)
	if err := CheckResourceUpsertableByError(err, httpMethod, extractedRes.Metadata.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	connector, err := services.UnmarshalGithubConnector(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.UpsertGithubConnector(ctx, connector); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewResourceItem(connector)
}

func (h *Handler) getTrustedClustersHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getTrustedClusters(r.Context(), clt)
}

func getTrustedClusters(ctx context.Context, clt resourcesAPIGetter) ([]ui.ResourceItem, error) {
	trustedClusters, err := clt.GetTrustedClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewTrustedClusters(trustedClusters)
}

func (h *Handler) deleteTrustedCluster(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tcName := params.ByName("name")
	if err := clt.DeleteTrustedCluster(r.Context(), tcName); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

func (h *Handler) upsertTrustedClusterHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req ui.ResourceItem
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return upsertTrustedCluster(r.Context(), clt, req.Content, r.Method)
}

func upsertTrustedCluster(ctx context.Context, clt resourcesAPIGetter, content, httpMethod string) (*ui.ResourceItem, error) {
	extractedRes, err := ExtractResourceAndValidate(content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if extractedRes.Kind != types.KindTrustedCluster {
		return nil, trace.BadParameter("resource kind %q is invalid", extractedRes.Kind)
	}

	_, err = clt.GetTrustedCluster(ctx, extractedRes.Metadata.Name)
	if err := CheckResourceUpsertableByError(err, httpMethod, extractedRes.Metadata.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	tc, err := services.UnmarshalTrustedCluster(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clt.UpsertTrustedCluster(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewResourceItem(tc)
}

// CheckResourceUpsertableByError checks if the resource is upsertable by the state of error with
// the request http method used.
func CheckResourceUpsertableByError(err error, httpMethod, resourceName string) error {
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	exists := err == nil
	if exists && httpMethod == http.MethodPost {
		return trace.AlreadyExists("resource name %q already exists", resourceName)
	}

	if !exists && httpMethod == http.MethodPut {
		return trace.NotFound("cannot find resource with name %q", resourceName)
	}

	return nil
}

// ExtractResourceAndValidate extracts resource information from given string and validates basic fields.
func ExtractResourceAndValidate(yaml string) (*services.UnknownResource, error) {
	var unknownRes services.UnknownResource
	reader := strings.NewReader(yaml)
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)

	if err := decoder.Decode(&unknownRes); err != nil {
		return nil, trace.BadParameter("not a valid resource declaration")
	}

	if err := unknownRes.Metadata.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &unknownRes, nil
}

// attemptListResources first checks that the auth server supports
// pagination and filtering before attempting to call the new ListResources api.
func attemptListResources(clt resourcesAPIGetter, r *http.Request, resourceKind string) (*types.ListResourcesResponse, error) {
	// ListResources for 'pagination' feature was available starting from v8
	// for DatabaseServers and AppServers, but it wasn't until v9.1 when
	// 'filtering' feature became available. Also in v8/v9.0, the web UI did not
	// use this new api, so we will treat v8/v9.0 as not implemented to avoid
	// false positives (really only applies to DatabaseServers and AppServers).
	pingRes, err := clt.Ping(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	currVersion := fmt.Sprintf("v%s", pingRes.GetServerVersion())
	filterSuppotedVersion := "v9.1.0"
	// If currVersion < filterSuppotedVersion
	if semver.Compare(currVersion, filterSuppotedVersion) == -1 {
		return nil, trace.NotImplemented("resource type %s does not support pagination", resourceKind)
	}

	res, err := listResources(clt, r, resourceKind)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return res, nil
}

// listResources gets a list of resources depending on the type of resource.
func listResources(clt resourcesAPIGetter, r *http.Request, resourceKind string) (*types.ListResourcesResponse, error) {
	values := r.URL.Query()

	limit, err := queryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Sort is expected in format `<fieldName>:<asc|desc>` where
	// index 0 is fieldName and index 1 is direction.
	// If a direction is not set, or is not recognized, it defaults to ASC.
	var sortBy types.SortBy
	sortParam := values.Get("sort")
	if sortParam != "" {
		vals := strings.Split(sortParam, ":")
		if vals[0] != "" {
			sortBy.Field = vals[0]
			if len(vals) > 1 && vals[1] == "desc" {
				sortBy.IsDesc = true
			}
		}
	}

	startKey := values.Get("startKey")
	req := proto.ListResourcesRequest{
		ResourceType:        resourceKind,
		Limit:               limit,
		StartKey:            startKey,
		SortBy:              sortBy,
		PredicateExpression: values.Get("query"),
		SearchKeywords:      client.ParseSearchKeywords(values.Get("search"), ' '),
		UseSearchAsRoles:    values.Get("searchAsRoles") == "yes",
	}

	return clt.ListResources(r.Context(), req)
}

func handleClusterNodesGet(clt resourcesAPIGetter, r *http.Request, clusterName string, userRoles services.RoleSet) (*listResourcesGetResponse, error) {
	resp, err := attemptListResources(clt, r, types.KindNode)
	if err == nil {
		servers, err := types.ResourcesWithLabels(resp.Resources).AsServers()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &listResourcesGetResponse{
			Items:      ui.MakeServers(clusterName, servers, userRoles),
			StartKey:   &resp.NextKey,
			TotalCount: &resp.TotalCount,
		}, nil
	}

	if !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err)
	}

	// Fallback support.
	nodes, err := clt.GetNodes(r.Context(), apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &listResourcesGetResponse{
		Items: ui.MakeServers(clusterName, nodes, userRoles),
	}, nil
}

func handleClusterDatabasesGet(clt resourcesAPIGetter, r *http.Request, clusterName string) (*listResourcesGetResponse, error) {
	resp, err := attemptListResources(clt, r, types.KindDatabaseServer)
	if err == nil {
		servers, err := types.ResourcesWithLabels(resp.Resources).AsDatabaseServers()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Make a list of all proxied databases.
		var databases []types.Database
		for _, server := range servers {
			databases = append(databases, server.GetDatabase())
		}

		return &listResourcesGetResponse{
			Items:      ui.MakeDatabases(clusterName, databases),
			StartKey:   &resp.NextKey,
			TotalCount: &resp.TotalCount,
		}, nil
	}

	if !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err)
	}

	// Fallback support.
	dbServers, err := clt.GetDatabaseServers(r.Context(), apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases []types.Database
	for _, server := range dbServers {
		databases = append(databases, server.GetDatabase())
	}

	return &listResourcesGetResponse{
		Items: ui.MakeDatabases(clusterName, types.DeduplicateDatabases(databases)),
	}, nil
}

func handleClusterAppsGet(clt resourcesAPIGetter, r *http.Request, cfg ui.MakeAppsConfig) (*listResourcesGetResponse, error) {
	// Check app config is not empty
	if cfg.Identity == nil || cfg.LocalClusterName == "" || cfg.LocalProxyDNSName == "" || cfg.AppClusterName == "" {
		return nil, trace.BadParameter("missing MakeAppsConfig required fields")
	}

	resp, err := attemptListResources(clt, r, types.KindAppServer)
	if err == nil {
		appServers, err := types.ResourcesWithLabels(resp.Resources).AsAppServers()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var apps types.Apps
		for _, server := range appServers {
			apps = append(apps, server.GetApp())
		}

		cfg.Apps = apps

		return &listResourcesGetResponse{
			Items:      ui.MakeApps(cfg),
			StartKey:   &resp.NextKey,
			TotalCount: &resp.TotalCount,
		}, nil
	}

	if !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err)
	}

	// Fallback support.
	appServers, err := clt.GetApplicationServers(r.Context(), apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var apps types.Apps
	for _, server := range appServers {
		apps = append(apps, server.GetApp())
	}

	cfg.Apps = types.DeduplicateApps(apps)

	return &listResourcesGetResponse{
		Items: ui.MakeApps(cfg),
	}, nil
}

func handleClusterDesktopsGet(clt resourcesAPIGetter, r *http.Request) (*listResourcesGetResponse, error) {
	resp, err := attemptListResources(clt, r, types.KindWindowsDesktop)
	if err == nil {
		windowsDesktops, err := types.ResourcesWithLabels(resp.Resources).AsWindowsDesktops()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &listResourcesGetResponse{
			Items:      ui.MakeDesktops(windowsDesktops),
			StartKey:   &resp.NextKey,
			TotalCount: &resp.TotalCount,
		}, nil
	}

	if !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err)
	}

	// Fallback support.
	windowsDesktops, err := clt.GetWindowsDesktops(r.Context(), types.WindowsDesktopFilter{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	windowsDesktops = types.DeduplicateDesktops(windowsDesktops)

	return &listResourcesGetResponse{
		Items: ui.MakeDesktops(windowsDesktops),
	}, nil
}

func handleClusterKubesGet(clt resourcesAPIGetter, r *http.Request) (*listResourcesGetResponse, error) {
	resp, err := attemptListResources(clt, r, types.KindKubernetesCluster)
	if err == nil {
		clusters, err := types.ResourcesWithLabels(resp.Resources).AsKubeClusters()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &listResourcesGetResponse{
			Items:      ui.MakeKubeClusters(clusters),
			StartKey:   &resp.NextKey,
			TotalCount: &resp.TotalCount,
		}, nil
	}

	if !trace.IsNotImplemented(err) {
		return nil, trace.Wrap(err)
	}

	// Fallback support.
	kubeServices, err := clt.GetKubeServices(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &listResourcesGetResponse{
		Items: ui.MakeLegacyKubeClusters(kubeServices),
	}, nil
}

type listResourcesGetResponse struct {
	// Items is a list of resources retrieved.
	Items interface{} `json:"items"`
	// StartKey is the position to resume search events.
	// 'nil' means that pagination is not supported in the Web UI.
	StartKey *string `json:"startKey"`
	// TotalCount is the total count of resources available
	// after filter.
	// 'nil' means that pagination is not supported in the Web UI.
	TotalCount *int `json:"totalCount"`
}

type checkAccessToRegisteredResourceResponse struct {
	// HasResource is a flag to indicate if user has any access
	// to a registered resource or not.
	HasResource bool `json:"hasResource"`
}

type resourcesAPIGetter interface {
	// GetRole returns role by name
	GetRole(ctx context.Context, name string) (types.Role, error)
	// GetRoles returns a list of roles
	GetRoles(ctx context.Context) ([]types.Role, error)
	// UpsertRole creates or updates role
	UpsertRole(ctx context.Context, role types.Role) error
	// UpsertGithubConnector creates or updates a Github connector
	UpsertGithubConnector(ctx context.Context, connector types.GithubConnector) error
	// GetGithubConnectors returns all configured Github connectors
	GetGithubConnectors(ctx context.Context, withSecrets bool) ([]types.GithubConnector, error)
	// GetGithubConnector returns the specified Github connector
	GetGithubConnector(ctx context.Context, id string, withSecrets bool) (types.GithubConnector, error)
	// DeleteGithubConnector deletes the specified Github connector
	DeleteGithubConnector(ctx context.Context, id string) error
	// UpsertTrustedCluster creates or updates a TrustedCluster in the backend.
	UpsertTrustedCluster(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error)
	// GetTrustedCluster returns a single TrustedCluster by name.
	GetTrustedCluster(ctx context.Context, name string) (types.TrustedCluster, error)
	// GetTrustedClusters returns all TrustedClusters in the backend.
	GetTrustedClusters(ctx context.Context) ([]types.TrustedCluster, error)
	// DeleteTrustedCluster removes a TrustedCluster from the backend by name.
	DeleteTrustedCluster(ctx context.Context, name string) error
	// ListResoures returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)

	GetApplicationServers(context.Context, string) ([]types.AppServer, error)
	GetDatabaseServers(context.Context, string, ...services.MarshalOption) ([]types.DatabaseServer, error)
	GetWindowsDesktops(context.Context, types.WindowsDesktopFilter) ([]types.WindowsDesktop, error)
	GetKubeServices(context.Context) ([]types.Server, error)
	GetNodes(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error)
	Ping(ctx context.Context) (proto.PingResponse, error)
}
