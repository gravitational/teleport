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
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/client/proto"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

// checkAccessToRegisteredResource checks if calling user has access to at least one registered resource.
func (h *Handler) checkAccessToRegisteredResource(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	// Get a client to the Auth Server with the logged in user's identity. The
	// identity of the logged in user is used to fetch the list of resources.
	clt, err := c.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resourceKinds := []string{types.KindNode, types.KindDatabaseServer, types.KindAppServer, types.KindKubeServer, types.KindWindowsDesktop}
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

func (h *Handler) listRolesHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	values := r.URL.Query()
	// If limit exists as a query parameter, this means its coming from a "new" webui
	// and can return the new paginated response.
	// TODO(gzdunek): DELETE IN 17.0.0: remove "getRoles".
	if values.Has("limit") {
		return listRoles(clt, values)
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

func listRoles(clt resourcesAPIGetter, values url.Values) (*listResourcesWithoutCountGetResponse, error) {
	limit, err := QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := clt.ListRoles(context.TODO(), &proto.ListRolesRequest{
		Limit:    limit,
		StartKey: values.Get("startKey"),
		Filter: &types.RoleFilter{
			SearchKeywords:  client.ParseSearchKeywords(values.Get("search"), ' '),
			SkipSystemRoles: true,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var typeRoles []types.Role
	for _, role := range roles.GetRoles() {
		typeRoles = append(typeRoles, role)
	}

	uiRoles, err := ui.NewRoles(typeRoles)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &listResourcesWithoutCountGetResponse{
		Items:    uiRoles,
		StartKey: roles.GetNextKey(),
	}, nil
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

func (h *Handler) createRoleHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := CreateResource(r, types.KindRole, services.UnmarshalRole, clt.CreateRole)
	return item, trace.Wrap(err)
}

func (h *Handler) updateRoleHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := UpdateResource(r, params, types.KindRole, services.UnmarshalRole, clt.UpdateRole)
	return item, trace.Wrap(err)
}

// getPresetRoles returns a list of preset roles expected to be available on
// this server. These are hard-coded for a given Teleport version, so this
// should have the same security implications as the Teleport version exposed
// via the public ping endpoint.
func (h *Handler) getPresetRoles(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	presets := auth.GetPresetRoles()
	return ui.NewRoles(presets)
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

func (h *Handler) updateGithubConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := UpdateResource[types.GithubConnector](r, params, types.KindGithubConnector, services.UnmarshalGithubConnector, clt.UpdateGithubConnector)
	return item, trace.Wrap(err)
}

func (h *Handler) createGithubConnectorHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := CreateResource(r, types.KindGithubConnector, services.UnmarshalGithubConnector, clt.CreateGithubConnector)
	return item, trace.Wrap(err)
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

	return upsertTrustedCluster(r.Context(), clt, req.Content, r.Method, params)
}

func upsertTrustedCluster(ctx context.Context, clt resourcesAPIGetter, content, httpMethod string, params httprouter.Params) (*ui.ResourceItem, error) {
	get := func(ctx context.Context, name string) (types.Resource, error) {
		// Remove the MFA resp from the context before getting the trusted cluster.
		// Otherwise, it will be consumed before the Upsert which actually
		// requires the MFA.
		// TODO(Joerger): Explicitly provide MFA response only where it is
		// needed instead of removing it like this.
		getCtx := mfa.ContextWithMFAResponse(ctx, nil)
		return clt.GetTrustedCluster(getCtx, name)
	}

	extractedRes, err := ExtractResourceAndValidate(content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if extractedRes.Kind != types.KindTrustedCluster {
		return nil, trace.BadParameter("resource kind %q is invalid", extractedRes.Kind)
	}

	if err := CheckResourceUpsert(ctx, httpMethod, params, extractedRes.Metadata.Name, get); err != nil {
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

// unmarshalFunc is a type signature for an unmarshaling function.
type unmarshalFunc[T types.Resource] func([]byte, ...services.MarshalOption) (T, error)

// CreateResource is a helper function for POST requests from the UI to create a new resource. It will
// validate the request contains the appropriate items, that the resource attempting to be created is
// valid. If all validations are satisfied then the creation is attempted. If the resource already exists
// a [trace.AlreadyExists] error is returned.
func CreateResource[T types.Resource](r *http.Request, kind string, unmarshalFn unmarshalFunc[T], createFn func(ctx context.Context, r T) (T, error)) (*ui.ResourceItem, error) {
	var req ui.ResourceItem
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	extractedRes, err := ExtractResourceAndValidate(req.Content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if extractedRes.Kind != kind {
		return nil, trace.BadParameter("resource kind %q is invalid", extractedRes.Kind)
	}

	resource, err := unmarshalFn(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := createFn(r.Context(), resource)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("resource with name %q already exists", extractedRes.Metadata.Name)
		}

		return nil, trace.Wrap(err)
	}

	item, err := ui.NewResourceItem(created)
	return item, trace.Wrap(err)
}

// UpdateResource is a helper function for PUT requests from the UI to update an existing resource. It will
// validate the request contains the appropriate items, that the resource attempting to be updated is
// valid. If all validations are satisfied then the update is attempted. If the resource does not exist
// a [trace.NotFound] error is returned.
func UpdateResource[T types.Resource](r *http.Request, params httprouter.Params, kind string, unmarshalFn unmarshalFunc[T], updateFn func(ctx context.Context, r T) (T, error)) (*ui.ResourceItem, error) {
	var req ui.ResourceItem
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	extractedRes, err := ExtractResourceAndValidate(req.Content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if extractedRes.Kind != kind {
		return nil, trace.BadParameter("resource kind %q is invalid", extractedRes.Kind)
	}

	resourceName := params.ByName("name")
	if resourceName == "" {
		return nil, trace.BadParameter("missing resource name")
	}

	// Error if the user is trying to rename the resource.
	if extractedRes.Metadata.Name != resourceName {
		return nil, trace.BadParameter("resource renaming is not supported, please create a different resource and then delete this one")
	}

	resource, err := unmarshalFn(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := updateFn(r.Context(), resource)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("resource with name %q does not exist", extractedRes.Metadata.Name)
		}

		return nil, trace.Wrap(err)
	}

	item, err := ui.NewResourceItem(updated)
	return item, trace.Wrap(err)
}

// getResource tries to retrieve a resource (by name),
// returning a NotFound error if the resource does not exist.
type getResource func(context.Context, string) (types.Resource, error)

// CheckResourceUpsert checks if the resource can be created or updated, depending on the http method.
func CheckResourceUpsert(ctx context.Context, httpMethod string, params httprouter.Params, payloadResourceName string, get getResource) error {
	switch httpMethod {
	case http.MethodPost:
		return trace.Wrap(checkResourceCreate(ctx, payloadResourceName, get))
	case http.MethodPut:
		resourceName := params.ByName("name")
		if resourceName == "" {
			return trace.BadParameter("missing resource name")
		}
		return trace.Wrap(checkResourceUpdate(ctx, payloadResourceName, resourceName, get))
	default:
		return trace.NotImplemented("http method %q not expected. this is a bug!", httpMethod)
	}
}

// checkResourceCreate checks if the resource can be created, returning nil if it can.
func checkResourceCreate(ctx context.Context, payloadResourceName string, get getResource) error {
	// Try to retrieve the resource by name.
	_, err := get(ctx, payloadResourceName)

	// If no error, then the resource already exists and cannot be created.
	if err == nil {
		return trace.AlreadyExists("resource with name %q already exists", payloadResourceName)
	}

	// If the error is not found, then the resource does not exist and can be created.
	if trace.IsNotFound(err) {
		return nil
	}

	return trace.Wrap(err)
}

// checkResourceUpdate checks if the resource can be updated, returning nil if it can.
func checkResourceUpdate(ctx context.Context, payloadResourceName, resourceName string, get getResource) error {
	// Error if the user is trying to rename the resource.
	if payloadResourceName != resourceName {
		return trace.BadParameter("resource renaming is not supported, please create a different resource and then delete this one")
	}

	// Try to retrieve the resource by name.
	_, err := get(ctx, payloadResourceName)

	// If no error, then the resource already exists and can be updated.
	if err == nil {
		return nil
	}

	// If the error is not found, then the resource does not exist and cannot be updated.
	if trace.IsNotFound(err) {
		return trace.NotFound("resource with name %q does not exist", payloadResourceName)
	}

	return trace.Wrap(err)
}

// ExtractResourceAndValidate extracts resource information from given string and validates basic fields.
func ExtractResourceAndValidate(yaml string) (*services.UnknownResource, error) {
	unknownRes, err := extractResource(yaml)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := unknownRes.Metadata.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &unknownRes, nil
}

func extractResource(yaml string) (services.UnknownResource, error) {
	var unknownRes services.UnknownResource
	reader := strings.NewReader(yaml)
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)

	if err := decoder.Decode(&unknownRes); err != nil {
		return services.UnknownResource{}, trace.BadParameter("not a valid resource declaration")
	}

	return unknownRes, nil
}

func convertListResourcesRequest(r *http.Request, kind string) (*proto.ListResourcesRequest, error) {
	values := r.URL.Query()

	limit, err := QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sortBy := types.GetSortByFromString(values.Get("sort"))

	startKey := values.Get("startKey")
	return &proto.ListResourcesRequest{
		ResourceType:        kind,
		Limit:               limit,
		StartKey:            startKey,
		SortBy:              sortBy,
		PredicateExpression: values.Get("query"),
		SearchKeywords:      client.ParseSearchKeywords(values.Get("search"), ' '),
		UseSearchAsRoles:    values.Get("searchAsRoles") == "yes",
	}, nil
}

// listKubeResources gets a list of kubernetes resources depending on the type of resource.
func listKubeResources(ctx context.Context, kubeClient kubeproto.KubeServiceClient, values url.Values, site, resourceKind string) (*kubeproto.ListKubernetesResourcesResponse, error) {
	req, err := newKubeListRequest(values, site, resourceKind)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return kubeClient.ListKubernetesResources(ctx, req)
}

// newKubeListRequest parses the request parameters into a ListKubernetesResourcesRequest.
func newKubeListRequest(values url.Values, site, resourceKind string) (*kubeproto.ListKubernetesResourcesRequest, error) {
	limit, err := QueryLimitAsInt32(values, "limit", defaults.MaxIterationLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sortBy := types.GetSortByFromString(values.Get("sort"))

	startKey := values.Get("startKey")
	req := &kubeproto.ListKubernetesResourcesRequest{
		ResourceType:        resourceKind,
		Limit:               limit,
		StartKey:            startKey,
		SortBy:              &sortBy,
		PredicateExpression: values.Get("query"),
		SearchKeywords:      client.ParseSearchKeywords(values.Get("search"), ' '),
		UseSearchAsRoles:    values.Get("searchAsRoles") == "yes",
		TeleportCluster:     site,
		KubernetesCluster:   values.Get("kubeCluster"),
		KubernetesNamespace: values.Get("kubeNamespace"),
	}
	return req, nil
}

type listResourcesGetResponse struct {
	// Items is a list of resources retrieved.
	Items interface{} `json:"items"`
	// StartKey is the position to resume search events.
	StartKey string `json:"startKey"`
	// TotalCount is the total count of resources available
	// after filter.
	TotalCount int `json:"totalCount"`
}

type listResourcesWithoutCountGetResponse struct {
	// Items is a list of resources retrieved.
	Items interface{} `json:"items"`
	// StartKey is the position to resume search events.
	StartKey string `json:"startKey"`
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
	// ListRoles returns a paginated list of roles.
	ListRoles(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error)
	// UpsertRole creates or updates role
	UpsertRole(ctx context.Context, role types.Role) (types.Role, error)
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
	// ListResources returns a paginated list of resources.
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)
}
