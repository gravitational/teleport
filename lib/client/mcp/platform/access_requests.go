// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package platform

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types"
	clientmcp "github.com/gravitational/teleport/lib/client/mcp"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var (
	// listUserAccessRequestsTool is the current user list access requests tool
	// defintion.
	listUserAccessRequestsTool = mcp.NewTool(
		clientmcp.ToolName("list_access_requests"),
		mcp.WithDescription("Lists current user access requests using the provided filters."),
		mcp.WithString(
			"state",
			mcp.Required(),
			mcp.Description(fmt.Sprintf(
				"State of the request. Can be any of: %s.",
				strings.Join(validStateValues, ","),
			)),
		),
	)

	// listRequestableRoles is the current user list requestable roles tool
	// definition.
	listRequestableRoles = mcp.NewTool(
		clientmcp.ToolName("list_requestable_roles"),
		mcp.WithDescription("Lists Teleport roles the current user can request access to."),
	)

	// createAccessRequestTool is the create access request tool definition.
	createAccessRequestTool = mcp.NewTool(
		clientmcp.ToolName("create_access_request"),
		mcp.WithDescription("Requests access to a Teleport resource."),
		mcp.WithArray(
			"roles",
			mcp.Items(map[string]any{"type": "string"}),
			mcp.Required(),
			mcp.Description("Is the list of Teleport roles to request access to."),
		),
		mcp.WithString(
			"request_ttl",
			mcp.Required(),
			mcp.Description("Is the expiration time of the access request (how long it will await for approval). "+durationExamples),
		),
		mcp.WithString(
			"session_ttl",
			mcp.Required(),
			mcp.Description("Is the expiration time for the session that will be used if the access request is approved. "+durationExamples),
		),
		mcp.WithString("reason", mcp.Description("Indicates the reason for an access request.")),
	)

	// listRequestsToBeReviewedTool is the list access requests to be reviwed by
	// current user tool definition.
	listRequestsToBeReviewedTool = mcp.NewTool(
		clientmcp.ToolName("list_access_requests_to_be_reviewed"),
		mcp.WithDescription("Lists access requests that can be reviewed by the current user."),
	)

	// approveAccessRequestTool is the approve access requests tool definition.
	approveAccessRequestTool = mcp.NewTool(
		clientmcp.ToolName("approve_access_request"),
		mcp.WithDescription("Approves the access request."),
		mcp.WithString(
			"access_request_uri",
			mcp.Required(),
			mcp.Description("Teleport access request resource URI that will be approved."),
		),
		mcp.WithString(
			"reason",
			mcp.Required(),
			mcp.Description("Reason for the approval."),
		),
	)

	// denyAccessRequestTool is the deny access requests tool definition.
	denyAccessRequestTool = mcp.NewTool(
		clientmcp.ToolName("deny_access_request"),
		mcp.WithDescription("Denies the access request."),
		mcp.WithString(
			"access_request_uri",
			mcp.Required(),
			mcp.Description("Teleport access request resource URI that will be denied."),
		),
		mcp.WithString(
			"reason",
			mcp.Required(),
			mcp.Description("Reason for the denial."),
		),
	)

	// validStateValues is the list of supported access requests state
	// values.
	validStateValues = slices.Collect(maps.Values(types.RequestState_name))
)

const (
	// durationExamples includes a short description of the time duration
	// format and some examples.
	durationExamples = "It must follow Golang's duration format, for example '10m' for 10 minutes, '1d20h30s' for 1 day, 20 hours and 30 seconds."
)

// AccessRequestResource is the access request MCP resource.
type AccessRequestResource struct {
	types.Metadata
	// URI is the MCP URI resource.
	URI string `json:"uri"`
	// ClusterName is the cluster the access request is from.
	ClusterName string `json:"cluster_name"`
	// State is the access request state.
	State string `json:"state"`
	// Requester is the user requesting access.
	Requester string `json:"requester"`
	// RequestReason is the request reason.
	RequestReason string `json:"request_reason,omitempty"`
	// ResolveReason is the resolve reason.
	ResolveReason string `json:"resolve_reason,omitempty"`
	// Roles is the list of roles being requested.
	Roles []string `json:"roles,omitempty"`
	// Resources is the list of resources being requested.
	Resources []AccessRequestResourceItem `json:"resources,omitempty"`
}

// AccessRequestResourceItem is the requested resources MCP resource.
type AccessRequestResourceItem struct {
	// Kind is the resource kind.
	Kind string `json:"kind"`
	// Name is the resource name.
	Name string `json:"name"`
}

// AccessRequestStateArg represents the access request state argument.
type AccessRequestStateArg struct {
	// State used to only return access requests with specified state.
	State string `json:"state"`
}

// ParseState parses the access request state into protobuf type.
func (a AccessRequestStateArg) ParseState() (types.RequestState, error) {
	if a.State == "" {
		return types.RequestState_NONE, trace.BadParameter("an access request state must be provided")
	}

	// Some models are not consistent with upper/lower case, so here we enforce
	// uppercase to cover both scenarios.
	state, ok := types.RequestState_value[strings.ToUpper(a.State)]
	if !ok {
		return types.RequestState_NONE, trace.BadParameter("%q is not a valid access request state. Value must be one of the following: %s", state, strings.Join(validStateValues, ", "))
	}

	return types.RequestState(state), nil
}

// ListUserAccessRequestsToolArgs is list access requests MCP tool arguments
// definition.
// 
// The JSON field names must match the arguments on the `listUserAccessRequestsTool`
// tool definition.
type ListUserAccessRequestsToolArgs struct {
	AccessRequestStateArg
}

// AccessRequestsServer exposes Access Requests as MCP resources and tools.
type AccessRequestsServer struct {
	config PlatformServerConfig
	server *mcpserver.MCPServer
	logger *slog.Logger
}

// NewAccessRequestsServer initializes the access request MCP server.
func NewAccessRequestsServer(rootServer *PlatformServer) *AccessRequestsServer {
	srv := &AccessRequestsServer{
		config: rootServer.config,
		logger: rootServer.config.Logger.With("server", "access_requests"),
	}

	if rootServer.config.AccessRequesterServerEnabled {
		rootServer.AddTool(listRequestableRoles, srv.ListUserRequestableRoles)
		rootServer.AddTool(listUserAccessRequestsTool, mcp.NewTypedToolHandler(srv.ListUserRequests))
		rootServer.AddTool(createAccessRequestTool, mcp.NewTypedToolHandler(srv.Create))
	}

	if rootServer.config.AccessRequestsReviwerServerEnabled {
		rootServer.AddTool(listRequestsToBeReviewedTool, srv.ListRequestsToBeReviewed)
		rootServer.AddTool(approveAccessRequestTool, mcp.NewTypedToolHandler(srv.Approve))
		rootServer.AddTool(denyAccessRequestTool, mcp.NewTypedToolHandler(srv.Deny))
	}

	return srv
}

// Close closes the MCP server, cleaning up resources.
func (a *AccessRequestsServer) Close() error {
	return nil
}

// ListUserRequestableRolesResponse is the response object of the
// ListUserRequestableRoles tool call.
type ListUserRequestableRolesResponse struct {
	Roles []string `json:"roles"`
}

// ListUserRequestableRoles lists the current user requestable roles.
func (a *AccessRequestsServer) ListUserRequestableRoles(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// fetch requestable roles.
	caps, err := a.config.Client.GetAccessCapabilities(ctx, types.AccessCapabilitiesRequest{
		User:             a.config.Username,
		RequestableRoles: true,
	})
	if err != nil {
		return a.formatError(trace.Wrap(err))
	}

	resp, err := utils.FastMarshal(ListUserRequestableRolesResponse{
		Roles: caps.RequestableRoles,
	})
	if err != nil {
		return a.formatError(trace.Wrap(err))
	}

	return mcp.NewToolResultText(string(resp)), nil
}

// ListUserRequestsResponse is the response object of the ListUserRequests tool
// call.
type ListUserRequestsResponse struct {
	// Requests list of requests made by the user.
	Requests []AccessRequestResource `json:"requests"`
}

// ListUserRequests lists the current user access requests.
//
// TODO(gabrielcorado): add support for pagination.
func (a *AccessRequestsServer) ListUserRequests(ctx context.Context, _ mcp.CallToolRequest, args ListUserAccessRequestsToolArgs) (*mcp.CallToolResult, error) {
	state, err := args.ParseState()
	if err != nil {
		return a.formatError(trace.Wrap(err))
	}

	resp, err := a.config.Client.GetAccessRequests(ctx, types.AccessRequestFilter{
		Requester: a.config.Username,
		State:     state,
	})
	if err != nil {
		return a.formatError(trace.Wrap(err))
	}

	resources := make([]AccessRequestResource, len(resp))
	for i, req := range resp {
		resources[i] = a.buildAccessRequestResource(req)
	}

	out, err := utils.FastMarshal(ListUserRequestsResponse{Requests: resources})
	if err != nil {
		return a.formatError(trace.Wrap(err))
	}

	return mcp.NewToolResultText(string(out)), nil
}

// AccessRequestsCreateArgs is the arguments used by create access request MCP
// tool.
//
// The JSON field names must match the arguments on the `createAccessRequestTool`
// tool definition.
type AccessRequestsCreateArgs struct {
	// Roles are the requested roles.
	Roles []string `json:"roles"`
	// RequestTTL is the expiration time of the access requestis.
	RequestTTL types.Duration `json:"request_ttl"`
	// SessionTTL is the expiration time of the session.
	SessionTTL types.Duration `json:"session_ttl"`
	// Reason is the reason for an access request.
	Reason string `json:"reason"`
}

// CheckAndSetDefaults checks and set defaults.
func (a AccessRequestsCreateArgs) CheckAndSetDefaults() error {
	if len(a.Roles) == 0 {
		return trace.BadParameter("must request access to at least one role")
	}
	if a.RequestTTL == 0 {
		return trace.BadParameter("request_ttl duration must be greater than zero")
	}
	if a.SessionTTL == 0 {
		return trace.BadParameter("session_ttl duration must be greater than zero")
	}
	return nil
}

// AccessRequestsCreateResponse is the create access request response.
type AccessRequestsCreateResponse struct {
	// Request is the resulting access request.
	Request AccessRequestResource `json:"request"`
}

// Create creates a new access request.
//
// TODO(gabrielcorado): support requesting access to resources.
func (a *AccessRequestsServer) Create(ctx context.Context, _ mcp.CallToolRequest, args AccessRequestsCreateArgs) (*mcp.CallToolResult, error) {
	if err := args.CheckAndSetDefaults(); err != nil {
		return a.formatError(trace.Wrap(err))
	}

	accessReq, err := services.NewAccessRequestWithResources(a.config.Username, args.Roles, nil /* resourceIDs */)
	if err != nil {
		return a.formatError(trace.Wrap(err))
	}

	accessReq.SetRequestReason(args.Reason)
	accessReq.SetExpiry(time.Now().UTC().Add(args.RequestTTL.Value()))
	accessReq.SetAccessExpiry(time.Now().UTC().Add(args.SessionTTL.Value()))

	req, err := a.config.Client.CreateAccessRequestV2(ctx, accessReq)
	if err != nil {
		return a.formatError(trace.Wrap(err))
	}

	out, err := utils.FastMarshal(AccessRequestsCreateResponse{
		Request: a.buildAccessRequestResource(req),
	})
	if err != nil {
		return a.formatError(trace.Wrap(err))
	}

	return mcp.NewToolResultText(string(out)), nil
}

// ListRequestsToBeReviewedResponse is the list requests to be reviewed
// response.
type ListRequestsToBeReviewedResponse struct {
	// Requests list of requests that are still peding review.
	Requests []AccessRequestResource `json:"requests"`
}

// ListRequestsToBeReviewed lists access requests that can be reviewed by the
// current user.
//
// TODO(gabrielcorado): add support for pagination.
func (a *AccessRequestsServer) ListRequestsToBeReviewed(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reqs, err := a.config.Client.GetAccessRequests(ctx, types.AccessRequestFilter{
		State: types.RequestState_PENDING,
	})
	if err != nil {
		return a.formatError(trace.Wrap(err))
	}

	var filtered []AccessRequestResource
reqs:
	for _, req := range reqs {
		// Do not include requests made by the user.
		if req.GetUser() == a.config.Username {
			continue
		}

		// Also skip requests already reviewed by the user.
		for _, rev := range req.GetReviews() {
			if rev.Author == a.config.Username {
				continue reqs
			}
		}

		filtered = append(filtered, a.buildAccessRequestResource(req))
	}

	out, err := utils.FastMarshal(ListRequestsToBeReviewedResponse{Requests: filtered})
	if err != nil {
		return a.formatError(trace.Wrap(err))
	}

	return mcp.NewToolResultText(string(out)), nil
}

// AccessRequestReviewArgs is the args used by Approve and Deny MCP tools.
//
// The JSON field names must match the arguments on the `approveAccessRequestTool`
// and `denyAccessRequestTool` tool definitions.
type AccessRequestReviewArgs struct {
	// AccessRequestURI is the access request MCP resource URI.
	AccessRequestURI string `json:"access_request_uri"`
	// Reason is the reason to Approve or Deny the request.
	Reason string `json:"reason"`
}

// Approve approves an access request.
func (a *AccessRequestsServer) Approve(ctx context.Context, _ mcp.CallToolRequest, args AccessRequestReviewArgs) (*mcp.CallToolResult, error) {
	if err := a.reviewRequest(ctx, types.RequestState_APPROVED, args); err != nil {
		return a.formatError(trace.Wrap(err))
	}

	return mcp.NewToolResultText("Approved with success"), nil
}

// Deny denies an access request.
func (a *AccessRequestsServer) Deny(ctx context.Context, _ mcp.CallToolRequest, args AccessRequestReviewArgs) (*mcp.CallToolResult, error) {
	if err := a.reviewRequest(ctx, types.RequestState_DENIED, args); err != nil {
		return a.formatError(trace.Wrap(err))
	}

	return mcp.NewToolResultText("Denied with success"), nil
}

func (a *AccessRequestsServer) reviewRequest(ctx context.Context, propsedState types.RequestState, args AccessRequestReviewArgs) error {
	parsedURI, err := clientmcp.ParseResourceURI(args.AccessRequestURI)
	if err != nil {
		return trace.Wrap(err)
	}

	if !parsedURI.IsAccessRequest() {
		return trace.BadParameter("must provide a valid access request resource URI")
	}

	_, err = a.config.Client.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: parsedURI.GetAccessRequestID(),
		Review: types.AccessReview{
			Author:        a.config.Username,
			ProposedState: propsedState,
			Reason:        args.Reason,
			// TODO(gabrielcorado): support AssumeStartTime
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *AccessRequestsServer) buildAccessRequestResource(req types.AccessRequest) AccessRequestResource {
	var resources []AccessRequestResourceItem
	for _, res := range req.GetRequestedResourceIDs() {
		resources = append(resources, AccessRequestResourceItem{
			Kind: res.Kind,
			Name: res.Name,
		})
	}

	return AccessRequestResource{
		Metadata:      req.GetMetadata(),
		URI:           clientmcp.NewAccessRequestResourceURI(a.config.ClusterName, req.GetName()).String(),
		ClusterName:   a.config.ClusterName,
		State:         types.RequestState_name[int32(req.GetState())],
		Requester:     req.GetUser(),
		RequestReason: req.GetRequestReason(),
		ResolveReason: req.GetResolveReason(),
		Roles:         req.GetRoles(),
		Resources:     resources,
	}
}

func (a *AccessRequestsServer) formatError(err error) (*mcp.CallToolResult, error) {
	message := err.Error()
	if clientmcp.IsSessionExpiredError(err) {
		message = clientmcp.ReloginRequiredErrorMessage
	}

	return mcp.NewToolResultError(message), nil
}
