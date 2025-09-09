package scimsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/elimity-com/scim/schema"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	icutils "github.com/gravitational/teleport/lib/utils/aws/identitycenterutils"
)

// Client is the interface for the SCIM SDK client
//
// WARNING: If you want to use the SCIM SDK please verify the SCIM integration and ensure
// that the SCIM implementation supports the methods you want to use and be aware of the SCIM API limitations.
//
// For example, AWS Identity Center SCIM have limitations on the ListUsers and ListGroups and  PatchOperations
// See https://docs.aws.amazon.com/singlesignon/latest/developerguide/listusers.html and
// https://docs.aws.amazon.com/singlesignon/latest/developerguide/listgroups.html
type Client interface {
	// CreateUser creates a user on the SCIM server.
	CreateUser(ctx context.Context, user *User) (*User, error)
	// DeleteUser deletes a user from the SCIM server.
	DeleteUser(ctx context.Context, id string) error
	// UpdateUser updates a user on the SCIM server.
	UpdateUser(ctx context.Context, user *User) (*User, error)
	// ListUsers returns a list of users from the SCIM server.
	// NOTE: Some implementations may not support pagination or filtering.
	// AWS IC SCIM Limitation see https://docs.aws.amazon.com/singlesignon/latest/developerguide/listusers.html:
	//  * startIndex, attributes, and excludedAttributes (despite being listed in the SCIM protocol)
	//  * At this time, the ListGroups API is only capable of returning up to 50 results.
	ListUsers(ctx context.Context, queryOptions ...QueryOption) (*ListUserResponse, error)
	// CreateGroup creates a group on the SCIM server.
	CreateGroup(ctx context.Context, group *Group) (*Group, error)
	// UpdateGroup updates a group on the SCIM server.
	UpdateGroup(ctx context.Context, group *Group) (*Group, error)
	// DeleteGroup deletes a group from the SCIM server.
	DeleteGroup(ctx context.Context, id string) error
	// ListGroups returns a list of groups from the SCIM server.
	// NOTE: Some implementations may not support pagination or filtering.
	// AWS IC SCIM Limitation see https://docs.aws.amazon.com/singlesignon/latest/developerguide/listgroups.html
	//  * ListGroups return an empty member list.
	//  * At this time, the ListGroups API is only capable of returning up to 50 results.
	ListGroups(ctx context.Context, queryOptions ...QueryOption) (*ListGroupResponse, error)
	// ReplaceGroupName replace the group display name.
	ReplaceGroupName(ctc context.Context, group *Group) error
	// ReplaceGroupMembers updates the members of a group.
	ReplaceGroupMembers(ctx context.Context, id string, members []*GroupMember) error
	// GetGroupByDisplayName returns a group by display name.
	GetGroupByDisplayName(ctx context.Context, displayName string) (*Group, error)
	// GetUserByUserName returns a user by username.
	GetUserByUserName(ctx context.Context, userName string) (*User, error)
	// Ping checks the connection to the SCIM server.
	Ping(ctx context.Context) error
	// GetUser returns a user by ID.
	GetUser(ctx context.Context, id string) (*User, error)
	// GetGroup returns a group by ID.
	GetGroup(ctx context.Context, id string) (*Group, error)
}

// ClientProvider is a function that creates a new SCIM SDK client.
// Please note that the ClientProvider is not thread-safe and
// should be used only in integration tests.
var ClientProvider = nativeClientProvider

// New creates a new SCIM SDK client.
func New(config *Config) (Client, error) {
	c, err := ClientProvider(config)
	return c, trace.Wrap(err)
}

func nativeClientProvider(config *Config) (Client, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &client{
		Config: config,
	}, nil
}

type client struct {
	*Config
}

// GetUser returns a user by ID from the SCIM server.
func (c *client) GetUser(ctx context.Context, id string) (*User, error) {
	u, err := c.endpointURL("Users", id)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.do(ctx, u, http.MethodGet, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	default:
		return nil, decodeError(resp)
	}

	var out User
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}

// GetGroup returns a group by ID from the SCIM server.
func (c *client) GetGroup(ctx context.Context, id string) (*Group, error) {
	u, err := c.endpointURL("Groups", id)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.do(ctx, u, http.MethodGet, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	default:
		return nil, decodeError(resp)
	}

	var out Group
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}

// UpdateGroup updates a group on the SCIM server.
func (c *client) UpdateGroup(ctx context.Context, group *Group) (*Group, error) {
	if c.Config.IntegrationType == types.PluginTypeAWSIdentityCenter {
		return nil, trace.BadParameter("AWS Identity Center does not support updating groups")
	}
	u, err := c.endpointURL("Groups", group.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	payload, err := json.Marshal(group)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.do(ctx, u, http.MethodPut, bytes.NewReader(payload))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	default:
		return nil, decodeError(resp)
	}

	var out Group
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}

// GetUserByUserName returns a user by userName from the SCIM server.
// SCIM endpoint identify resource by ID not by property like userName.
// The GetUsersByUserName method is implemented by injecting the userName into the filter.
func (c *client) GetUserByUserName(ctx context.Context, userName string) (*User, error) {
	listUserResp, err := c.ListUsers(ctx, WithUserNameFilter(userName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(listUserResp.Users) == 0 {
		return nil, trace.NotFound("user %q not found", userName)
	}
	if len(listUserResp.Users) != 1 {
		return nil, trace.BadParameter("expected one user, got %v", len(listUserResp.Users))
	}
	return listUserResp.Users[0], nil
}

// GetGroupByDisplayName returns a group by displayName from the SCIM server.
// SCIM endpoint identify resource by ID not by property like displayName.
// The GetGroupByDisplayName method is implemented by injecting the displayName into the filter.
func (c *client) GetGroupByDisplayName(ctx context.Context, displayName string) (*Group, error) {
	listGroupResp, err := c.ListGroups(ctx, WithDisplayNameFilter(displayName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(listGroupResp.Groups) == 0 {
		return nil, trace.NotFound("group %q not found", displayName)
	}
	if len(listGroupResp.Groups) != 1 {
		return nil, trace.BadParameter("expected one group, got %v", len(listGroupResp.Groups))
	}
	return listGroupResp.Groups[0], nil
}

// Config is the configuration for the SCIM SDK client
type Config struct {
	// Endpoint is the SCIM endpoint.
	Endpoint string
	// Token is the SCIM auth token.
	Token string
	// IntegrationType holds value of plugin or integration
	// for which this SCIM client is configured.
	IntegrationType string

	// Log is the logger.
	Log *slog.Logger
	// HTTPClient is the HTTP client.
	HTTPClient *http.Client
	// maxPageSize is the maximum page size for SCIM queries.
	maxPageSize int
}

// CheckAndSetDefaults checks the configuration and sets the defaults.
func (c *Config) checkAndSetDefaults() error {
	if c.Endpoint == "" {
		return trace.BadParameter("missing SCIM endpoint")
	}
	if c.IntegrationType == "" {
		return trace.BadParameter("missing integration type")
	}
	if c.IntegrationType == types.PluginTypeAWSIdentityCenter {
		ensuredURL, err := icutils.EnsureSCIMEndpoint(c.Endpoint)
		if err != nil {
			return trace.Wrap(err)
		}
		c.Endpoint = ensuredURL
	}
	if c.Token == "" {
		return trace.BadParameter("missing SCIM auth token")
	}
	if c.Log == nil {
		c.Log = slog.Default().With(teleport.ComponentKey, "SCIM_SDK")
	}
	if c.HTTPClient == nil {
		var err error
		if c.HTTPClient, err = defaults.HTTPClient(); err != nil {
			return trace.Wrap(err)
		}
	}
	if c.maxPageSize == 0 {
		// AWS Identity Center can handle a max of 100 member records at a time
		c.maxPageSize = 100
	}
	return nil
}

// ListUsers returns a list of users from the SCIM server.
// NOTE: Some implementations may not support pagination or filtering.
// AWS IC SCIM Limitation see https://docs.aws.amazon.com/singlesignon/latest/developerguide/listusers.html:
//   - startIndex, attributes, and excludedAttributes (despite being listed in the SCIM protocol)
//   - At this time, the ListGroups API is only capable of returning up to 50 results.
func (c *client) ListUsers(ctx context.Context, queryOptions ...QueryOption) (*ListUserResponse, error) {
	var options QueryOptions
	for _, opt := range queryOptions {
		opt(&options)
	}

	u, err := c.endpointURL("Users")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u.RawQuery = options.toQuery().Encode()
	u.RawQuery = u.Query().Encode()

	resp, err := c.do(ctx, u, http.MethodGet, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	default:
		return nil, decodeError(resp)
	}

	var listResp ListUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, trace.Wrap(err)
	}
	return &listResp, nil
}

// CreateUser creates a user on the SCIM server.
func (c *client) CreateUser(ctx context.Context, user *User) (*User, error) {
	user.Schemas = []string{schema.UserSchema}
	u, err := c.endpointURL("Users")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	payload, err := json.Marshal(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.do(ctx, u, http.MethodPost, bytes.NewReader(payload))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
	default:
		return nil, decodeError(resp)
	}

	var out User
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}

// DeleteUser deletes a user from the SCIM server.
func (c *client) DeleteUser(ctx context.Context, id string) error {
	return trace.Wrap(c.deleteResource(ctx, "Users", id))
}

// UpdateUser updates a user on the SCIM server.
func (c *client) UpdateUser(ctx context.Context, user *User) (*User, error) {
	u, err := c.endpointURL("Users", user.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	payload, err := json.Marshal(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.do(ctx, u, http.MethodPut, bytes.NewReader(payload))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
	default:
		return nil, decodeError(resp)
	}

	var out User
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}

// CreateGroup creates a group on the SCIM server.
func (c *client) CreateGroup(ctx context.Context, group *Group) (*Group, error) {
	group.Schemas = []string{schema.GroupSchema}
	u, err := c.endpointURL("Groups")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	payload, err := json.Marshal(group)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.do(ctx, u, http.MethodPost, bytes.NewReader(payload))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
	default:
		return nil, decodeError(resp)
	}

	var out Group
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}

// DeleteGroup deletes a group from the SCIM server.
func (c *client) DeleteGroup(ctx context.Context, id string) error {
	return trace.Wrap(c.deleteResource(ctx, "Groups", id))
}

// ReplaceGroupName updates a group on the SCIM server.
func (c *client) ReplaceGroupName(ctx context.Context, group *Group) error {
	u, err := c.endpointURL("Groups", group.ID)
	if err != nil {
		return trace.Wrap(err)
	}

	// AWS only supports patch operations on groups, so we have to patch the
	// values we want to change rather than do the more obvious PUT.

	patch := PatchOperations{
		Schemas: []string{PatchOpSchema},
		Operations: []PatchOp{
			{
				Operation: OpReplace,
				Path:      "displayName",
				Value:     group.DisplayName,
			},
		},
	}

	payload, err := json.Marshal(patch)
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := c.do(ctx, u, http.MethodPatch, bytes.NewReader(payload))
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
	default:
		return decodeError(resp)
	}

	return nil
}

// ReplaceGroupMembers replaces the members of a group.
// if members is empty, it will remove all members from the group.
// The HTTP PATCH method is used to replace the members of a group.
func (c *client) ReplaceGroupMembers(ctx context.Context, id string, members []*GroupMember) error {
	u, err := c.endpointURL("Groups", id)
	if err != nil {
		return trace.Wrap(err)
	}

	patchOp := OpReplace
	if len(members) == 0 {
		// AWS (at least) will not set an empty member list via OpReplace. In
		// order to clear the member list, we have to specifically `remove` it.
		patchOp = OpRemove
	}

	// Beware the odd post-loop condition test here. We need to go through this
	// *loop at least once* to handle the case where `groupMembers` is empty,
	// and we need to delete all users in the downstream group.
	for ok := true; ok; ok = len(members) > 0 {
		var membersPage []*GroupMember
		membersPage, members = takePage(members, c.maxPageSize)

		res := PatchOperations{
			Schemas: []string{PatchOpSchema},
			Operations: []PatchOp{
				{
					Operation: patchOp,
					Path:      "members",
					Value:     membersPage,
				},
			},
		}
		payload, err := json.Marshal(res)
		if err != nil {
			return trace.Wrap(err)
		}
		resp, err := c.do(ctx, u, http.MethodPatch, bytes.NewReader(payload))
		if err != nil {
			return trace.Wrap(err)
		}
		// Close the response body immediately after the request is done..
		resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK, http.StatusNoContent:
		default:
			return decodeError(resp)
		}
		// See https://docs.aws.amazon.com/singlesignon/latest/developerguide/patchgroup.html
		// * A maximum of 100 membership changes are allowed in a single request.
		// Bypass this limitation by appending the next 100 members to the group.
		patchOp = OpAdd
	}
	return nil
}

func (c *client) endpointURL(paths ...string) (*url.URL, error) {
	path, err := url.JoinPath(c.Endpoint, paths...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u, err := url.Parse(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

// ListGroups returns a list of groups from the SCIM server.
// NOTE: Some implementations may not support pagination or filtering.
// AWS IC SCIM Limitation see https://docs.aws.amazon.com/singlesignon/latest/developerguide/listgroups.html
//   - ListGroups return an empty member list.
//   - At this time, the ListGroups API is only capable of returning up to 50 results.
func (c *client) ListGroups(ctx context.Context, queryOptions ...QueryOption) (*ListGroupResponse, error) {
	var options QueryOptions
	for _, opt := range queryOptions {
		opt(&options)
	}

	u, err := c.endpointURL("Groups")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u.RawQuery = options.toQuery().Encode()
	u.RawQuery = u.Query().Encode()

	resp, err := c.do(ctx, u, http.MethodGet, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	default:
		return nil, decodeError(resp)
	}

	var listResp ListGroupResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, trace.Wrap(err)
	}
	return &listResp, nil
}

// Ping checks the connection to the SCIM server by sending a request to the ServiceProviderConfig endpoint.
func (c *client) Ping(ctx context.Context) error {
	u, err := c.endpointURL("ServiceProviderConfig")
	if err != nil {
		return trace.Wrap(err)
	}
	resp, err := c.do(ctx, u, http.MethodGet, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return trace.AccessDenied("unauthorized")
	case http.StatusInternalServerError:
		return trace.BadParameter("internal server error")
	default:
		return trace.BadParameter("unexpected status code %v", resp.StatusCode)
	}
	return nil
}

func (c *client) deleteResource(ctx context.Context, resourceType, id string) error {
	u, err := c.endpointURL(resourceType, id)
	if err != nil {
		return trace.Wrap(err)
	}
	resp, err := c.do(ctx, u, http.MethodDelete, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil

	case http.StatusNotFound:
		return trace.NotFound("resource not found")

	default:
		return decodeError(resp)
	}
}

func (c *client) do(ctx context.Context, u *url.URL, httpMethod string, r io.Reader) (*http.Response, error) {
	const (
		authHeaderKey   = "Authorization"
		acceptHeaderKey = "Accept"
	)
	if u == nil {
		return nil, trace.BadParameter("missing URL")
	}

	req, err := http.NewRequestWithContext(ctx, httpMethod, u.String(), r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.Header.Set(authHeaderKey, fmt.Sprintf("Bearer %s", c.Token))
	req.Header.Set(acceptHeaderKey, ContentType)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func takePage[S ~[]T, T any](src S, pageSize int) (S, S) {
	if len(src) <= pageSize {
		return src, nil
	}
	return src[:pageSize], src[pageSize:]
}
