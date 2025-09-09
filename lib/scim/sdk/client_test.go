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

package scimsdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestSCIMMockServer(t *testing.T) {
	mockServer := newSCIMHTTPServer(t)

	client := newTestClient(mockServer.URL())

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

type scimHTTPServer struct {
	server *httptest.Server
	store  *ClientMock
}

func NewSCIMHTTPServer(t *testing.T) *scimHTTPServer {
	return newSCIMHTTPServer(t)
}

func newSCIMHTTPServer(t *testing.T) *scimHTTPServer {
	mock := &scimHTTPServer{
		store: NewSCIMClientMock(),
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
	r.HandleFunc("DELETE /Groups/{id}", mock.deleteGroup)

	r.HandleFunc("GET /ServiceProviderConfig", mock.ping)

	// Start the test server with the router
	mock.server = httptest.NewServer(r)

	t.Cleanup(mock.server.Close)

	return mock
}

func (s *scimHTTPServer) NewClient() Client {
	return newTestClient(s.server.URL)
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

func (s *scimHTTPServer) ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func newTestClient(serverURL string) *client {
	return &client{
		Config: &Config{
			Endpoint:    serverURL,
			Token:       "store-token",
			HTTPClient:  http.DefaultClient,
			maxPageSize: 100,
		},
	}
}
