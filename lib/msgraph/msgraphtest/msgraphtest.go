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

package msgraphtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

// Server defines fake server.
type Server struct {
	mu        sync.RWMutex
	TLSServer *httptest.Server
	Storage   *Storage
}

// ServerOption is a custom opt for [NewServer].
type ServerOption func(*Server)

// WithStorage configures default storage
func WithStorage(storage *Storage) ServerOption {
	return func(s *Server) {
		s.Storage = storage
	}
}

// NewServer creates a new fake server.
func NewServer(opts ...ServerOption) *Server {
	// By default, use storage populated with default mock data
	s := &Server{
		Storage: NewDefaultStorage(),
	}
	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	s.TLSServer = httptest.NewTLSServer(s.Handler())

	return s
}

// Fake server handler
func (s *Server) Handler() http.Handler {
	r := http.NewServeMux()

	r.HandleFunc("GET /v1.0/users", s.handleListUsers)
	r.HandleFunc("GET /v1.0/users/delta", s.handleListUsersDelta)
	r.HandleFunc("GET /v1.0/groups", s.handleListGroups)
	r.HandleFunc("GET /v1.0/groups/delta", s.handleListGroupsDelta)
	r.HandleFunc("GET /v1.0/groups/{id}/members", s.handleListGroupMembers)
	r.HandleFunc("GET /v1.0/groups/{id}/owners/microsoft.graph.user", s.handleListGroupOwners)
	r.HandleFunc("/v1.0/", s.handleCatchAll)
	r.HandleFunc("/metadata/identity/oauth2/token", s.handleGetToken)

	return r
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	users := make([]*models.User, 0, len(s.Storage.Users))
	for _, user := range s.Storage.Users {
		users = append(users, user)
	}
	s.mu.RUnlock()

	jsonResponse(w, map[string]interface{}{
		"value": users,
	})
}

// handleListUsersDelta handles user delta queries.
// It expects a delta key provided by the client in the
// form of a delta token. It does not support pagination.
// It consumes existing delta token on each request,
// increments delta token counter by one and responds with the
// new delta token.
func (s *Server) handleListUsersDelta(w http.ResponseWriter, r *http.Request) {
	currentKey := 0
	isLatest := false
	users := make([]models.ListUsersDeltaResponse, 0)
	token := r.URL.Query().Get("$deltatoken")
	switch token {
	case "latest":
		// latest request is the starting point.
		isLatest = true
	default:
		i, err := parseToken(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		currentKey = i
	}

	s.mu.Lock()
	if !isLatest {
		users = append(users, s.Storage.UsersDelta[currentKey]...)
	}
	currentKey++
	if _, ok := s.Storage.UsersDelta[currentKey]; !ok {
		// increment delta token counter if its not already set.
		s.Storage.UsersDelta[currentKey] = []models.ListUsersDeltaResponse{}
	}
	s.mu.Unlock()

	jsonResponse(w, map[string]interface{}{
		"@odata.deltaLink": deltaLink(r, strconv.Itoa(currentKey)),
		"value":            users,
	})
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	groups := make([]*models.Group, 0, len(s.Storage.Groups))
	for _, group := range s.Storage.Groups {
		groups = append(groups, group)
	}
	s.mu.RUnlock()

	jsonResponse(w, map[string]interface{}{
		"value": groups,
	})
}

// handleListGroupsDelta handles group delta queries.
// It expects a delta key provided by the client in the
// form of a delta token. It does not support pagination.
// It consumes existing delta token on each request,
// increments delta token counter by one and responds with the
// new delta token.
func (s *Server) handleListGroupsDelta(w http.ResponseWriter, r *http.Request) {
	currentKey := 0
	isLatest := false
	groups := make([]models.ListGroupsDeltaResponse, 0)
	token := r.URL.Query().Get("$deltatoken")
	switch token {
	case "latest":
		// latest request is the starting point.
		isLatest = true
	default:
		i, err := parseToken(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		currentKey = i
	}

	s.mu.Lock()
	if !isLatest {
		groups = append(groups, s.Storage.GroupsDelta[currentKey]...)
	}
	currentKey++
	if _, ok := s.Storage.GroupsDelta[currentKey]; !ok {
		// increment delta token counter if its not already set.
		s.Storage.GroupsDelta[currentKey] = []models.ListGroupsDeltaResponse{}
	}
	s.mu.Unlock()

	jsonResponse(w, map[string]interface{}{
		"@odata.deltaLink": deltaLink(r, strconv.Itoa(currentKey)),
		"value":            groups,
	})
}

func (s *Server) handleListGroupMembers(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")
	s.mu.RLock()
	groupMembers := slices.Clone(s.Storage.GroupMembers[groupID])
	members := make([]map[string]interface{}, 0, len(groupMembers))
	for _, member := range groupMembers {
		memberData := map[string]interface{}{
			"id": member.GetID(),
		}

		switch member.(type) {
		case *models.User:
			memberData["@odata.type"] = models.ODataUser
		case *models.Group:
			memberData["@odata.type"] = models.ODataGroup
		default:
			// Default to user if unknown
			memberData["@odata.type"] = models.ODataUser
		}

		members = append(members, memberData)
	}
	s.mu.RUnlock()

	jsonResponse(w, map[string]interface{}{
		"value": members,
	})
}

func (s *Server) handleListGroupOwners(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")
	s.mu.RLock()
	owners := slices.Clone(s.Storage.GroupOwners[groupID])
	s.mu.RUnlock()

	jsonResponse(w, map[string]interface{}{
		"value": owners,
	})
}

// handleGetApplication handles GET /v1.0/applications(appId='...') requests.
func (s *Server) handleGetApplication(w http.ResponseWriter, r *http.Request, appID string) {
	s.mu.RLock()
	app, ok := s.Storage.Applications[appID]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "application not found", http.StatusNotFound)
		return
	}

	jsonResponse(w, app)
}

var (
	applicationByAppIDPattern = regexp.MustCompile(`^/v1\.0/applications\(appId='([^']+)'\)$`)
)

// handleCatchAll handles other endpoints like applications(appId='app-id').
func (s *Server) handleCatchAll(w http.ResponseWriter, r *http.Request) {
	// Handle GET /v1.0/applications(appId='app-id')
	if r.Method == http.MethodGet {
		if matches := applicationByAppIDPattern.FindStringSubmatch(r.URL.Path); matches != nil {
			appID := matches[1]
			s.handleGetApplication(w, r, appID)
			return
		}
	}

	http.NotFound(w, r)
}

// handleGetToken handles token request.
func (s *Server) handleGetToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// credential detail is irrelevant.
	const token = `{
		"token_type": "Bearer",
		"scope": "Mail.Read User.Read",
		"expires_in": 3600,
		"ext_expires_in": 3600,
		"access_token": "abc-access-token",
		"refresh_token": "abc-refresh-token"
	}`
	w.Write([]byte(token))
}

// SetUsers updates users storage.
func (s *Server) SetUsers(users []*models.User) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, user := range users {
		if user.ID != nil {
			s.Storage.Users[*user.ID] = user
		}
	}

	// update user delta
	userDelta := make([]models.ListUsersDeltaResponse, 0, len(users))
	for _, d := range users {
		userDelta = append(userDelta, models.ListUsersDeltaResponse{
			User: d,
		})
	}
	appendUserDeltas(s.Storage, userDelta...)
}

// DeleteUsers removes users from the storage.
func (s *Server) DeleteUsers(users []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, userID := range users {
		if userID != "" {
			delete(s.Storage.Users, userID)
		}
	}

	// update user delta
	userDelta := make([]models.ListUsersDeltaResponse, 0, len(users))
	for _, userID := range users {
		userDelta = append(userDelta, models.ListUsersDeltaResponse{
			User: &models.User{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(userID),
				},
			},
			Removed: &models.RemovedReason{
				Reason: to.Ptr("deleted"),
			},
		})
	}
	appendUserDeltas(s.Storage, userDelta...)
	for _, userID := range users {
		s.deleteGroupMemberships(userID)
		s.deleteGroupOwnerships(userID)
	}
}

// SetGroups updates groups storage.
func (s *Server) SetGroups(groups []*models.Group) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, group := range groups {
		if group.ID != nil {
			s.Storage.Groups[*group.ID] = group
		}
	}

	// update group deltas.
	groupDeltas := make([]models.ListGroupsDeltaResponse, 0, len(groups))
	for _, g := range groups {
		groupDeltas = append(groupDeltas, models.ListGroupsDeltaResponse{
			Group: g,
		})
	}
	appendGroupDeltas(s.Storage, groupDeltas...)
}

// DeleteGroups deletes the groups from storage.
func (s *Server) DeleteGroups(groups []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, groupID := range groups {
		if groupID == "" {
			continue
		}
		delete(s.Storage.Groups, groupID)
		delete(s.Storage.GroupMembers, groupID)
		delete(s.Storage.GroupOwners, groupID)
	}

	// update group delta.
	groupDeltas := make([]models.ListGroupsDeltaResponse, 0, len(groups))
	for _, groupID := range groups {
		groupDeltas = append(groupDeltas, models.ListGroupsDeltaResponse{
			Group: &models.Group{
				DirectoryObject: models.DirectoryObject{
					ID: to.Ptr(groupID),
				},
			},
			Removed: &models.RemovedReason{
				Reason: to.Ptr("deleted"),
			},
		})

	}
	appendGroupDeltas(s.Storage, groupDeltas...)
	for _, groupID := range groups {
		s.deleteGroupMemberships(groupID)
	}
}

// SetGroupMembers updates group members storage.
func (s *Server) SetGroupMembers(groupID string, members []models.GroupMember) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existingMembers := setGroupMembers(s.Storage, groupID, members)

	// update member delta
	memberDeltas := getGroupMembersDelta(members, existingMembers)
	if len(memberDeltas) == 0 {
		return
	}

	group, ok := s.Storage.Groups[groupID]
	if !ok {
		// should never happen
		return
	}

	// Check if a delta object for the given group
	// already exists for the latest key. If it does, append to
	// existing delta, otherwise, create a new one.
	latestKey := latestDeltaKey(s.Storage.GroupsDelta)
	deltas := s.Storage.GroupsDelta[latestKey]
	found := false
	for i, d := range deltas {
		if d.Group == nil || d.Group.GetID() == nil {
			continue
		}
		if *d.Group.GetID() == groupID {
			found = true
			d.Members = append(d.Members, memberDeltas...)
			deltas[i] = d
		}
	}
	if found {
		// this is a new delta for the latest token
		s.Storage.GroupsDelta[latestKey] = deltas
		return
	}

	appendGroupDeltas(s.Storage, models.ListGroupsDeltaResponse{
		Group: &models.Group{
			DirectoryObject: models.DirectoryObject{
				ID:          to.Ptr(groupID),
				DisplayName: group.DisplayName,
			},
		},
		Members: memberDeltas,
	})
}

func setGroupMembers(s *Storage, groupID string, newMembers []models.GroupMember) map[string]struct{} {
	existingMembers := make(map[string]struct{})
	for _, m := range s.GroupMembers[groupID] {
		if m.GetID() == nil {
			continue
		}
		existingMembers[*m.GetID()] = struct{}{}
	}
	allMembers := slices.Concat(s.GroupMembers[groupID], newMembers)
	s.GroupMembers[groupID] = utils.DeduplicateAny(allMembers,
		func(m1, m2 models.GroupMember) bool {
			if m1.GetID() == nil || m2.GetID() == nil {
				return false
			}

			return *m1.GetID() == *m2.GetID()
		})
	return existingMembers
}

func getGroupMembersDelta(newMembers []models.GroupMember, existingMembers map[string]struct{}) []models.MembersDelta {
	var memberDeltas []models.MembersDelta
	for _, m := range newMembers {
		if m.GetID() == nil {
			continue
		}
		if _, ok := existingMembers[*m.GetID()]; ok {
			// this may be a no op if member already exists.
			continue
		}

		// this is a new membership delta
		memberDeltas = append(memberDeltas, models.MembersDelta{
			DirectoryObject: &models.DirectoryObject{
				ID: m.GetID(),
			},
			Type: memberType(m),
		})
		// add this new member to existingMembers map.
		existingMembers[*m.GetID()] = struct{}{}
	}
	return memberDeltas
}

// DeleteGroupMembers removes group memberships.
func (s *Server) DeleteGroupMembers(groupID string, members []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deleteGroupMembers(groupID, members)
}

func (s *Server) deleteGroupMembers(groupID string, members []string) {
	newMembersDeltas := deleteGroupMembers(s.Storage, groupID, members)
	if len(newMembersDeltas) == 0 {
		return
	}

	group, ok := s.Storage.Groups[groupID]
	if !ok {
		// should never happen
		return
	}

	// check if a delta object for the given group
	// already exists for the latest key.
	latestKey := latestDeltaKey(s.Storage.GroupsDelta)
	deltas := s.Storage.GroupsDelta[latestKey]
	found := false
	for i, d := range deltas {
		if d.Group == nil || d.Group.GetID() == nil {
			continue
		}
		if *d.Group.GetID() == groupID {
			found = true
			d.Members = append(d.Members, newMembersDeltas...)
			deltas[i] = d
		}
	}
	if found {
		// this is the first delta object for the group
		// in the latest key.
		s.Storage.GroupsDelta[latestKey] = deltas
		return
	}
	appendGroupDeltas(s.Storage, models.ListGroupsDeltaResponse{
		Group: &models.Group{
			DirectoryObject: models.DirectoryObject{
				ID:          to.Ptr(groupID),
				DisplayName: group.DisplayName,
			},
		},
		Members: newMembersDeltas,
	})
}

func deleteGroupMembers(s *Storage, groupID string, deletedMembers []string) []models.MembersDelta {
	existingMembers := s.GroupMembers[groupID]
	newMembersDeltas := []models.MembersDelta{}
	newMembers := []models.GroupMember{}
	for _, gm := range existingMembers {
		if gm.GetID() == nil {
			continue
		}
		if slices.Contains(deletedMembers, *gm.GetID()) {
			newMembersDeltas = append(newMembersDeltas, models.MembersDelta{
				DirectoryObject: &models.DirectoryObject{
					ID: gm.GetID(),
				},
				Removed: &models.RemovedReason{
					Reason: to.Ptr("deleted"),
				},
				Type: memberType(gm),
			})
		} else {
			newMembers = append(newMembers, gm)
		}
	}
	s.GroupMembers[groupID] = newMembers

	return newMembersDeltas
}

func (s *Server) deleteGroupMemberships(memberID string) {
	// deleteGroupMemberships expects caller to hold the lock.
	for gid := range s.Storage.GroupMembers {
		s.deleteGroupMembers(gid, []string{memberID})
	}
}

func (s *Server) deleteGroupOwnerships(ownerID string) {
	// deleteGroupOwnerships expects the caller to hold the lock.
	for gid := range s.Storage.GroupOwners {
		s.deleteGroupOwners(gid, []string{ownerID})
	}
}

// SetGroupOwners updates group owners storage.
func (s *Server) SetGroupOwners(groupID string, owners []*models.User) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// set owners
	existingOwners := setGroupOwners(s.Storage, groupID, owners)

	// update owners delta
	var ownerDeltas []models.OwnersDelta
	for _, o := range owners {
		if o.GetID() == nil {
			continue
		}
		if _, ok := existingOwners[*o.GetID()]; ok {
			continue
		}
		existingOwners[*o.GetID()] = struct{}{}
		ownerDeltas = append(ownerDeltas, models.OwnersDelta{
			User: &models.User{
				DirectoryObject: models.DirectoryObject{
					ID: o.GetID(),
				},
			},
			Type: models.ODataUser,
		})
	}

	if len(ownerDeltas) == 0 {
		return
	}

	group, ok := s.Storage.Groups[groupID]
	if !ok {
		// should never happen
		return
	}

	// check if a delta object for the given group
	// already exists for the latest key.
	latestKey := latestDeltaKey(s.Storage.GroupsDelta)
	deltas := s.Storage.GroupsDelta[latestKey]
	found := false
	for i, d := range deltas {
		if d.Group == nil || d.Group.GetID() == nil {
			continue
		}
		if *d.Group.GetID() == groupID {
			found = true
			d.Owners = append(d.Owners, ownerDeltas...)
			deltas[i] = d
		}
	}

	if found {
		// this is the new delta object for latest token
		s.Storage.GroupsDelta[latestKey] = deltas
		return
	}
	// append delta to existing delta object.
	appendGroupDeltas(s.Storage, models.ListGroupsDeltaResponse{
		Group: &models.Group{
			DirectoryObject: models.DirectoryObject{
				ID:          to.Ptr(groupID),
				DisplayName: group.DisplayName,
			},
		},
		Owners: ownerDeltas,
	})
}

func setGroupOwners(s *Storage, groupID string, owners []*models.User) map[string]struct{} {
	existingOwners := make(map[string]struct{})
	for _, m := range s.GroupOwners[groupID] {
		if m.GetID() == nil {
			continue
		}
		existingOwners[*m.GetID()] = struct{}{}
	}
	allOwners := slices.Concat(s.GroupOwners[groupID], owners)
	s.GroupOwners[groupID] = utils.DeduplicateAny(allOwners,
		func(m1, m2 *models.User) bool {
			if m1.GetID() == nil || m2.GetID() == nil {
				return false
			}
			return *m1.GetID() == *m2.GetID()
		})
	return existingOwners
}

// DeleteGroupOwners removes group ownership.
func (s *Server) DeleteGroupOwners(groupID string, owners []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deleteGroupOwners(groupID, owners)
}

func (s *Server) deleteGroupOwners(groupID string, owners []string) {
	deletedOwnersDelta := deleteGroupOwners(s.Storage, groupID, owners)
	if len(deletedOwnersDelta) == 0 {
		return
	}

	group, ok := s.Storage.Groups[groupID]
	if !ok {
		// should never happen
		return
	}

	// check if a delta object for the given group
	// already exists for the latest key.
	latestKey := latestDeltaKey(s.Storage.GroupsDelta)
	deltas := s.Storage.GroupsDelta[latestKey]
	found := false
	for i, d := range deltas {
		if d.Group == nil || d.Group.GetID() == nil {
			continue
		}
		if *d.Group.GetID() == groupID {
			found = true
			d.Owners = append(d.Owners, deletedOwnersDelta...)
			deltas[i] = d
		}
	}
	if found {
		// this is a new delta object for the group.
		s.Storage.GroupsDelta[latestKey] = deltas
		return
	}
	// append to existing delta.
	appendGroupDeltas(s.Storage, models.ListGroupsDeltaResponse{
		Group: &models.Group{
			DirectoryObject: models.DirectoryObject{
				ID:          to.Ptr(groupID),
				DisplayName: group.DisplayName,
			},
		},
		Owners: deletedOwnersDelta,
	})
}

func deleteGroupOwners(s *Storage, groupID string, deletedOwners []string) []models.OwnersDelta {
	groupOwners := s.GroupOwners[groupID]
	newOwners := []*models.User{}
	deletedOwnersDelta := []models.OwnersDelta{}
	for _, o := range groupOwners {
		if o.GetID() == nil {
			continue
		}
		if slices.Contains(deletedOwners, *o.GetID()) {
			// only expecting owner of user type.
			deletedOwnersDelta = append(deletedOwnersDelta, models.OwnersDelta{
				User: &models.User{
					DirectoryObject: models.DirectoryObject{
						ID: o.GetID(),
					},
				},
				Removed: &models.RemovedReason{
					Reason: to.Ptr("deleted"),
				},
				Type: models.ODataUser,
			})
			continue
		}

		newOwners = append(newOwners, o)
	}
	s.GroupOwners[groupID] = newOwners

	return deletedOwnersDelta
}

// SetApplications updates application storage.
func (s *Server) SetApplications(apps []*models.Application) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, app := range apps {
		if app.AppID != nil {
			s.Storage.Applications[*app.AppID] = app
		}
	}
}

func jsonResponse(writer http.ResponseWriter, data interface{}) {
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

// RewriteTransport configures custom transport.
type RewriteTransport struct {
	Base http.RoundTripper
	URL  *url.URL
}

// RoundTrip swaps incoming URL with configured URL.
func (rt *RewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = rt.URL.Scheme
	req.URL.Host = rt.URL.Host
	return rt.Base.RoundTrip(req)
}

// FakeDeltaStore implements [DeltaStore].
type FakeDeltaStore struct {
	mu    sync.Mutex
	cache map[string]string
}

// NewFakeDeltaStore creates a new [FakeDeltaStore].
func NewFakeDeltaStore() *FakeDeltaStore {
	return &FakeDeltaStore{
		cache: make(map[string]string),
	}
}

// Get returns delta token for the given endpoint.
func (s *FakeDeltaStore) Get(endpoint string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cache[endpoint]
}

// Set sets delta token for the given endpoint.
func (s *FakeDeltaStore) Set(endpoint, link string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[endpoint] = link
}

// Clear removes delta token for the given endpoint.
func (s *FakeDeltaStore) Clear(endpoint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, endpoint)
}

func memberType(gm models.GroupMember) string {
	memberType := models.ODataUser
	switch gm.(type) {
	case *models.Group:
		memberType = models.ODataGroup
	default:
		// handle unknown member type
	}

	return memberType
}

func appendUserDeltas(s *Storage, deltas ...models.ListUsersDeltaResponse) {
	key := latestDeltaKey(s.UsersDelta)
	s.UsersDelta[key] = append(s.UsersDelta[key], deltas...)
}

func appendGroupDeltas(s *Storage, deltas ...models.ListGroupsDeltaResponse) {
	key := latestDeltaKey(s.GroupsDelta)
	s.GroupsDelta[key] = append(s.GroupsDelta[key], deltas...)
}

func parseToken(token string) (int, error) {
	parts := strings.Split(token, "#") // delta token counter is separated with #
	if len(parts) != 2 {
		return 0, trace.BadParameter("invalid delta token")
	}
	if parts[1] == "" {
		return 0, trace.BadParameter("invalid delta token")
	}

	return strconv.Atoi(parts[1])
}

func latestDeltaKey[T any](deltaMap map[int][]T) int {
	latest := 0
	for key := range deltaMap {
		if key > latest {
			latest = key
		}
	}
	return latest
}

func deltaLink(r *http.Request, deltaToken string) string {
	u := url.URL{
		Host:     r.Host,
		Scheme:   "https",
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}
	q := u.Query()
	q.Del("$deltatoken")
	q.Set("$deltatoken", "fake-deltatoken#"+deltaToken)
	u.RawQuery = q.Encode()
	return u.String()
}
