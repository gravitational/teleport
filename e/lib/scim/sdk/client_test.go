package scimsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func TestSCIMMockServer(t *testing.T) {
	mockServer := newSCIMHTTPServer(t)

	client := mockServer.newClient()

	ctx := context.Background()

	user := &User{UserName: "test@example.com", DisplayName: "Test User"}
	createdUser, err := client.CreateUser(ctx, user)
	require.NoError(t, err)
	assert.NotEmpty(t, createdUser.ID)

	fetchedUser, err := client.GetUser(ctx, createdUser.ID)
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", fetchedUser.UserName)

	createdUser.DisplayName = "Updated User"
	updatedUser, err := client.UpdateUser(ctx, createdUser)
	require.NoError(t, err)
	assert.Equal(t, "Updated User", updatedUser.DisplayName)

	listResp, err := client.ListUsers(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(listResp.Users), 1)

	err = client.DeleteUser(ctx, createdUser.ID)
	require.NoError(t, err)

	group := &Group{DisplayName: "Test Group"}
	createdGroup, err := client.CreateGroup(ctx, group)
	require.NoError(t, err)
	assert.NotEmpty(t, createdGroup.ID)

	fetchedGroup, err := client.GetGroup(ctx, createdGroup.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Group", fetchedGroup.DisplayName)

	createdGroup.DisplayName = "Updated Group"
	updatedGroup, err := client.UpdateGroup(ctx, createdGroup)
	require.NoError(t, err)
	assert.Equal(t, "Updated Group", updatedGroup.DisplayName)

	listGroupsResp, err := client.ListGroups(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(listGroupsResp.Groups), 1)

	err = client.DeleteGroup(ctx, createdGroup.ID)
	require.NoError(t, err)

	err = client.Ping(ctx)
	require.NoError(t, err)
}

func TestClientTypeAWSIdentityCenter(t *testing.T) {
	mockServer := newSCIMHTTPServer(t)

	client := &client{
		Config: &Config{
			Endpoint:        mockServer.URL(),
			Token:           "store-token",
			HTTPClient:      http.DefaultClient,
			maxPageSize:     100,
			IntegrationType: types.PluginTypeAWSIdentityCenter,
		},
	}
	_, err := client.UpdateGroup(context.Background(), &Group{})
	require.True(t, trace.IsBadParameter(err))
}

func asMemberList(IDs []string) []*GroupMember {
	result := make([]*GroupMember, len(IDs))
	for i, id := range IDs {
		result[i] = &GroupMember{ExternalID: id}
	}
	return result
}

func makeMembers(n int, format string) []string {
	result := make([]string, n)
	for i := range result {
		result[i] = fmt.Sprintf(format, i)
	}
	return result
}

func TestClientPatchGroupMembers(t *testing.T) {
	const groupID = "patchable"
	mockServer := newSCIMHTTPServer(t, WithMaxPageSize(7))

	defaultMemberList := []string{
		"uid-aurora", "uid-beau", "uid-charlotte", "uid-david", "uid-emilia",
		"uid-francis", "uid-gabrielle", "uid-harry", "uid-isobel"}

	mockServer.store.CreateGroup(t.Context(),
		&Group{
			ID: groupID,
			Meta: &Metadata{
				ResourceType: ResourceTypeGroup,
				Location:     "/Patchable",
			},
		})

	client := mockServer.newClient()

	testCases := []struct {
		name           string
		toAdd          []string
		toRemove       []string
		errorAssertion require.ErrorAssertionFunc
		expected       []string
	}{
		{
			name:           "empty",
			errorAssertion: require.NoError,
			expected:       defaultMemberList,
		},
		{
			name:           "add",
			toAdd:          makeMembers(3, "uid-addend-%02d"),
			errorAssertion: require.NoError,
			expected:       append(defaultMemberList, makeMembers(3, "uid-addend-%02d")...),
		},
		{
			name:           "remove",
			toRemove:       []string{"uid-aurora", "uid-david", "uid-gabrielle", "uid-isobel"},
			errorAssertion: require.NoError,
			expected:       []string{"uid-beau", "uid-charlotte", "uid-emilia", "uid-francis", "uid-harry"},
		},
		{
			name:           "combined",
			toAdd:          makeMembers(3, "uid-addend-%02d"),
			toRemove:       []string{"uid-aurora", "uid-david", "uid-gabrielle", "uid-harry", "uid-isobel"},
			errorAssertion: require.NoError,
			expected: append(
				append([]string{"uid-beau"}, "uid-charlotte", "uid-emilia", "uid-francis"),
				makeMembers(3, "uid-addend-%02d")...),
		},
		{
			name:           "oversize",
			toAdd:          makeMembers(9, "uid-addend-%02d"),
			toRemove:       defaultMemberList,
			errorAssertion: require.NoError,
			expected:       makeMembers(9, "uid-addend-%02d"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mockServer.store.SetGroupMembers(t.Context(), groupID, asMemberList(defaultMemberList)...)
			toAdd := asMemberList(testCase.toAdd)
			toRemove := asMemberList(testCase.toRemove)

			err := client.PatchGroupMembers(t.Context(), groupID, toAdd, toRemove)
			testCase.errorAssertion(t, err)

			g, err := mockServer.store.GetGroup(t.Context(), groupID)
			require.NoError(t, err, "test target group is missing")

			actualMembers := sliceutils.Map(g.Members, (*GroupMember).GetExternalID)
			require.ElementsMatch(t, testCase.expected, actualMembers)
		})
	}
}

// TODO(tcsc): Break the test HTTP service out into its own file

type scimHTTPServer struct {
	server *httptest.Server
	store  *ClientMock
	token  string
	// The number of PATCH values that can be applied at once
	maxPatchSize int
}

type SCIMHTTPServerOption func(*scimHTTPServer)

func WithMaxPageSize(n int) SCIMHTTPServerOption {
	return func(mock *scimHTTPServer) {
		mock.maxPatchSize = n
	}
}

func NewSCIMHTTPServer(t *testing.T, opts ...SCIMHTTPServerOption) *scimHTTPServer {
	return newSCIMHTTPServer(t, opts...)
}

func newSCIMHTTPServer(t *testing.T, opts ...SCIMHTTPServerOption) *scimHTTPServer {
	mock := &scimHTTPServer{
		store:        NewSCIMClientMock(),
		token:        uuid.NewString(),
		maxPatchSize: 100,
	}

	r := http.NewServeMux()
	r.HandleFunc("POST /Users", mock.createUser)
	r.HandleFunc("GET /Users", mock.listUsers)
	r.HandleFunc("GET /Users/{id}", mock.getUser)
	r.HandleFunc("PUT /Users/{id}", mock.updateUser)
	r.HandleFunc("DELETE /Users/{id}", mock.deleteUser)

	r.HandleFunc("POST /Groups", mock.createGroup)
	r.HandleFunc("GET /Groups", mock.listGroups)
	r.HandleFunc("GET /Groups/{id}", mock.getGroup)
	r.HandleFunc("PUT /Groups/{id}", mock.updateGroup)
	r.HandleFunc("PATCH /Groups/{id}", mock.patchGroup)
	r.HandleFunc("DELETE /Groups/{id}", mock.deleteGroup)

	r.HandleFunc("GET /ServiceProviderConfig", mock.ping)

	for _, applyOption := range opts {
		applyOption(mock)
	}

	// Start the test server with the router
	mock.server = httptest.NewUnstartedServer(r)
	mock.server.StartTLS()

	t.Cleanup(mock.server.Close)

	return mock
}

func (s *scimHTTPServer) newClient() Client {
	return &client{
		Config: s.clientConfig(),
	}
}

func (s *scimHTTPServer) clientConfig() *Config {
	return &Config{
		Endpoint:    s.URL(),
		Token:       s.token,
		HTTPClient:  s.server.Client(),
		maxPageSize: s.maxPatchSize,
	}
}

// URL returns the server's base URL
func (s *scimHTTPServer) URL() string {
	return s.server.URL
}

func (s *scimHTTPServer) createUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.store.CreateUser(context.Background(), &user)
	if err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *scimHTTPServer) getUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := s.store.GetUser(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *scimHTTPServer) updateUser(w http.ResponseWriter, r *http.Request) {
	var updated User
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.store.UpdateUser(context.Background(), &updated)
	if err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *scimHTTPServer) deleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := s.store.DeleteUser(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *scimHTTPServer) listUsers(w http.ResponseWriter, r *http.Request) {
	resp, err := s.store.ListUsers(context.Background())
	if err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *scimHTTPServer) createGroup(w http.ResponseWriter, r *http.Request) {
	var group Group
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := s.store.CreateGroup(context.Background(), &group)
	if err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *scimHTTPServer) getGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	resp, err := s.store.GetGroup(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *scimHTTPServer) updateGroup(w http.ResponseWriter, r *http.Request) {
	var updated Group
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.store.UpdateGroup(context.Background(), &updated)
	if err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func writeResponse(w http.ResponseWriter, status int, format string, args ...any) {
	w.WriteHeader(status)
	fmt.Fprintf(w, format, args...)
}

func writeBadRequest(w http.ResponseWriter, format string, args ...any) {
	writeResponse(w, http.StatusBadRequest, format, args...)
}

func (s *scimHTTPServer) patchGroup(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r); err != nil {
		writeResponse(w, http.StatusUnauthorized, "%s", err.Error())
		return
	}

	groupID := r.PathValue("id")
	if groupID == "" {
		writeBadRequest(w, "missing group ID")
		return
	}

	var patch PatchOperations
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeBadRequest(w, "Malformed patch request: %s", err.Error())
		return
	}

	if !slices.Contains(patch.Schemas, PatchOpSchema) {
		writeBadRequest(w, "Missing SCIM patch schema")
		return
	}

	changeCount := 0

	type validatedPatchOp struct {
		applyFn func(context.Context, string, ...*GroupMember) error
		members []*GroupMember
	}

	var validatedPatchOps []validatedPatchOp

	for _, op := range patch.Operations {
		if op.Path != "members" {
			writeBadRequest(w, "Unsupported path %q, Group patching only supports \"members\"", op.Path)
			return
		}

		var err error
		var dst validatedPatchOp

		dst.members, err = decodeMemberList(op.Value)
		if err != nil {
			writeBadRequest(w, "Malformed patch value: %s", err.Error())
			return
		}
		changeCount += len(dst.members)

		switch op.Operation {
		case OpAdd:
			dst.applyFn = s.store.AddGroupMembers

		case OpRemove:
			dst.applyFn = s.store.DeleteGroupMembers

		case OpReplace:
			dst.applyFn = s.store.SetGroupMembers

		default:
			writeBadRequest(w, "Unrecognized patch operation: %s", op.Operation)
			return
		}

		validatedPatchOps = append(validatedPatchOps, dst)
	}

	if changeCount > s.maxPatchSize {
		writeBadRequest(w, "Too many membership changes in a single PATCH. Max: %d, received: %d",
			s.maxPatchSize, changeCount)
		return
	}

	for _, op := range validatedPatchOps {
		if err := op.applyFn(r.Context(), groupID, op.members...); err != nil {
			if trace.IsNotFound(err) {
				writeResponse(w, http.StatusNotFound, "")
				return
			}
			writeResponse(w, http.StatusInternalServerError, "%s", err.Error())
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func decodeMemberList(value any) ([]*GroupMember, error) {
	text, err := json.Marshal(&value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// A list of members is the most common case, so try that first
	var result []*GroupMember
	if json.Unmarshal(text, &result) == nil {
		return result, nil
	}

	var member GroupMember
	if err := json.Unmarshal(text, &member); err != nil {
		return nil, trace.Wrap(err, "malformed member/member list")
	}

	return []*GroupMember{&member}, nil
}

func (s *scimHTTPServer) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteGroup(context.Background(), id); err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *scimHTTPServer) listGroups(w http.ResponseWriter, r *http.Request) {
	resp, err := s.store.ListGroups(context.Background())
	if err != nil {
		http.Error(w, err.Error(), trace.ErrorToCode(err))
		return
	}
	if err := json.NewEncoder(w).Encode(&resp); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (s *scimHTTPServer) authorize(r *http.Request) error {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "Bearer "+s.token {
		return trace.AccessDenied("bad SCIM bearer token")
	}
	return nil
}

func (s *scimHTTPServer) ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
