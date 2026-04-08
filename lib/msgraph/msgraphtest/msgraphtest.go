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
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/gravitational/teleport/lib/msgraph"
)

// Server defines fake server.
type Server struct {
	mu        sync.RWMutex
	TLSServer *httptest.Server
	Storage   *Storage

	MonkeyPatch struct {
		HandleListUsersDelta func(w http.ResponseWriter, r *http.Request)
	}
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
	users := make([]*msgraph.User, 0, len(s.Storage.Users))
	for _, user := range s.Storage.Users {
		users = append(users, user)
	}
	s.mu.RUnlock()

	jsonResponse(w, map[string]interface{}{
		"value": users,
	})
}

func (s *Server) handleListUsersDelta(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	if s.MonkeyPatch.HandleListUsersDelta != nil {
		s.MonkeyPatch.HandleListUsersDelta(w, r)
		return
	}
	s.ListUsersDelta(w, r)
}

func (s *Server) ListUsersDelta(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("$deltatoken")
	currentKey := 0
	var users []msgraph.ListUsersDeltaResponse
	if token != "" {
		parts := strings.Split(token, "#")
		if parts[1] != "" {
			i, err := strconv.Atoi(parts[1])
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			currentKey = i
			users = append(users, s.Storage.UsersDelta[i]...)
		}

	} else {
		// reset the delta storage
		s.Storage.UsersDelta = make(map[int][]msgraph.ListUsersDeltaResponse)
		users = make([]msgraph.ListUsersDeltaResponse, 0, len(s.Storage.Users))
		for _, user := range s.Storage.Users {
			users = append(users, msgraph.ListUsersDeltaResponse{User: *user})
		}
	}

	currentKey++
	s.Storage.UsersDelta[currentKey] = []msgraph.ListUsersDeltaResponse{}

	s.mu.RUnlock()

	if len(users) == 0 {
		users = []msgraph.ListUsersDeltaResponse{}
	}

	jsonResponse(w, map[string]interface{}{
		"@odata.deltaLink": deltaLink(r, strconv.Itoa(currentKey)),
		"value":            users,
	})
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	groups := make([]*msgraph.Group, 0, len(s.Storage.Groups))
	for _, group := range s.Storage.Groups {
		groups = append(groups, group)
	}
	s.mu.RUnlock()

	jsonResponse(w, map[string]interface{}{
		"value": groups,
	})
}

func (s *Server) handleListGroupsDelta(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	token := r.URL.Query().Get("$deltatoken")
	currentKey := 0
	var groups []msgraph.ListGroupsDeltaResponse
	if token != "" {
		parts := strings.Split(token, "#")

		if parts[1] != "" {
			i, err := strconv.Atoi(parts[1])
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			currentKey = i
			groups = append(groups, s.Storage.GroupsDelta[i]...)
		}
	} else {
		// this is the first delta request
		s.Storage.GroupsDelta = make(map[int][]msgraph.ListGroupsDeltaResponse)
		groups = make([]msgraph.ListGroupsDeltaResponse, 0, len(s.Storage.Groups))
		for id, group := range s.Storage.Groups {
			memberDeltas := make([]msgraph.Delta, 0, len(s.Storage.GroupMembers[id]))
			for _, member := range s.Storage.GroupMembers[id] {
				if member == nil || member.GetID() == nil {
					continue
				}

				d := msgraph.Delta{
					DirectoryObject: msgraph.DirectoryObject{ID: member.GetID()},
				}
				switch member.(type) {
				case *msgraph.User:
					d.Type = "#microsoft.graph.user"
				case *msgraph.Group:
					d.Type = "#microsoft.graph.group"
				default:
					d.Type = "#microsoft.graph.directoryObject"
				}
				memberDeltas = append(memberDeltas, d)
			}

			ownerDeltas := make([]msgraph.Delta, 0, len(s.Storage.GroupOwners[id]))
			for _, owner := range s.Storage.GroupOwners[id] {
				if owner == nil || owner.ID == nil {
					continue
				}
				ownerDeltas = append(ownerDeltas, msgraph.Delta{
					DirectoryObject: msgraph.DirectoryObject{ID: owner.ID},
					Type:            "#microsoft.graph.user",
				})
			}

			groups = append(groups, msgraph.ListGroupsDeltaResponse{
				Group:   *group,
				Members: memberDeltas,
				Owners:  ownerDeltas,
			})
		}
	}
	s.mu.RUnlock()

	currentKey++
	s.Storage.GroupsDelta[currentKey] = []msgraph.ListGroupsDeltaResponse{}

	if len(groups) == 0 {
		groups = []msgraph.ListGroupsDeltaResponse{}
	}

	jsonResponse(w, map[string]interface{}{
		"@odata.deltaLink": deltaLink(r, strconv.Itoa(currentKey)),
		"value":            groups,
	})
}

func (s *Server) handleListGroupMembers(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")

	s.mu.RLock()
	groupMembers := s.Storage.GroupMembers[groupID]
	s.mu.RUnlock()

	members := make([]map[string]interface{}, 0, len(groupMembers))
	for _, member := range groupMembers {
		memberData := map[string]interface{}{
			"id": member.GetID(),
		}

		switch member.(type) {
		case *msgraph.User:
			memberData["@odata.type"] = "#microsoft.graph.user"
		case *msgraph.Group:
			memberData["@odata.type"] = "#microsoft.graph.group"
		default:
			// Default to user if unknown
			memberData["@odata.type"] = "#microsoft.graph.user"
		}

		members = append(members, memberData)
	}

	jsonResponse(w, map[string]interface{}{
		"value": members,
	})
}

func (s *Server) handleListGroupOwners(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")

	s.mu.RLock()
	owners := s.Storage.GroupOwners[groupID]
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
func (s *Server) SetUsers(users []*msgraph.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, user := range users {
		if user.ID != nil {
			s.Storage.Users[*user.ID] = user
		}
	}

	keys := slices.Collect(maps.Keys(s.Storage.UsersDelta))
	latestKey := len(keys)

	var userDelta []msgraph.ListUsersDeltaResponse
	for _, d := range users {
		userDelta = append(userDelta, msgraph.ListUsersDeltaResponse{
			User: *d,
		})
	}
	s.Storage.UsersDelta[latestKey] = append(s.Storage.UsersDelta[latestKey], userDelta...)
}

// DeleteUsers updates users storage.
func (s *Server) DeleteUsers(users []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, userID := range users {
		if userID != "" {
			delete(s.Storage.Users, userID)
		}
	}

	// update user delta
	keys := slices.Collect(maps.Keys(s.Storage.UsersDelta))
	latestKey := len(keys)

	var userDelta []msgraph.ListUsersDeltaResponse
	for _, userID := range users {
		userDelta = append(userDelta, msgraph.ListUsersDeltaResponse{
			User: msgraph.User{
				DirectoryObject: msgraph.DirectoryObject{
					ID: to.Ptr(userID),
				},
			},
			Removed: &msgraph.RemovedReason{
				Reason: to.Ptr("changed"),
			},
		})
	}
	s.Storage.UsersDelta[latestKey] = append(s.Storage.UsersDelta[latestKey], userDelta...)

	// remove user's group membership
	var groupsDelta []msgraph.ListGroupsDeltaResponse
	for gi, gms := range s.Storage.GroupMembers {
		newMembers := []msgraph.GroupMember{}
		removedMemberDelta := []msgraph.Delta{}
		for _, gm := range gms {
			if slices.Contains(users, *gm.GetID()) {
				// remove the user from the group membership
				// update group member delta
				memberType := "#microsoft.graph.user"
				switch gm.(type) {
				case *msgraph.Group:
					memberType = "#microsoft.graph.group"
				default:
					// handle unknown member type
				}

				// collect removed member Ids
				removedMemberDelta = append(removedMemberDelta, msgraph.Delta{
					DirectoryObject: msgraph.DirectoryObject{
						ID: gm.GetID(),
					},
					Type: memberType,
					Removed: &msgraph.RemovedReason{
						Reason: to.Ptr("deleted"),
					},
				})

			} else {
				newMembers = append(newMembers, gm)
			}
		}
		groupsDelta = append(groupsDelta, msgraph.ListGroupsDeltaResponse{
			Group: msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
					ID: to.Ptr(gi),
				},
			},
			Members: removedMemberDelta,
		})

		s.Storage.GroupMembers[gi] = newMembers
	}
}

// SetGroups updates groups storage.
func (s *Server) SetGroups(groups []*msgraph.Group) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, group := range groups {
		if group.ID != nil {
			s.Storage.Groups[*group.ID] = group
		}
	}
}

func (s *Server) DeleteGroups(groups []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, groupID := range groups {
		if groupID != "" {
			delete(s.Storage.Groups, groupID)
		}
	}

	// update user delta
	keys := slices.Collect(maps.Keys(s.Storage.GroupsDelta))
	latestKey := len(keys)

	var groupDeltas []msgraph.ListGroupsDeltaResponse
	for _, groupID := range groups {
		groupDeltas = append(groupDeltas, msgraph.ListGroupsDeltaResponse{
			Group: msgraph.Group{
				DirectoryObject: msgraph.DirectoryObject{
					ID: to.Ptr(groupID),
				},
			},
			Removed: &msgraph.RemovedReason{
				Reason: to.Ptr("deleted"),
			},
		})
	}

	// TODO(sshah): check if the group delta already exists for this group
	// e.g., group membership was updated by DeleteGroupMembers.
	s.Storage.GroupsDelta[latestKey] = append(s.Storage.GroupsDelta[latestKey], groupDeltas...)

}

// SetGroupMembers updates group members storage.
func (s *Server) SetGroupMembers(groupID string, members []msgraph.GroupMember) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Storage.GroupMembers[groupID] = members
}

func (s *Server) DeleteGroupMembers(groupID string, members []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	groupMembers := s.Storage.GroupMembers[groupID]
	newMembers := []msgraph.GroupMember{}
	newMemberDeltas := []msgraph.Delta{}
	for _, gm := range groupMembers {
		if slices.Contains(members, *gm.GetID()) {
			memberType := "#microsoft.graph.user"
			switch gm.(type) {
			case *msgraph.Group:
				memberType = "#microsoft.graph.group"
			default:
				// handle unknown member type
			}
			newMemberDeltas = append(newMemberDeltas, msgraph.Delta{
				DirectoryObject: msgraph.DirectoryObject{
					ID: gm.GetID(),
				},
				Type: memberType,
				Removed: &msgraph.RemovedReason{
					Reason: to.Ptr("changed"),
				},
			})
		} else {
			newMembers = append(newMembers, gm)
		}
	}
	s.Storage.GroupMembers[groupID] = newMembers

	// update delta
	keys := slices.Collect(maps.Keys(s.Storage.GroupsDelta))
	latestKey := len(keys)

	group, ok := s.Storage.Groups[groupID]
	if !ok {
		// should never happen
		return
	}

	// TODO(sshah): check if the group delta already exists for this group
	// e.g., group membership was updated by DeleteGroupMembers.
	// below, it replaces the whole group delta for the given groupiD
	s.Storage.GroupsDelta[latestKey] = append(s.Storage.GroupsDelta[latestKey], msgraph.ListGroupsDeltaResponse{
		Group: msgraph.Group{
			DirectoryObject: msgraph.DirectoryObject{
				ID:          to.Ptr(groupID),
				DisplayName: group.DisplayName,
			},
		},
		Members: newMemberDeltas,
	})

}

// SetGroupOwners updates group owners storage.
func (s *Server) SetGroupOwners(groupID string, users []*msgraph.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Storage.GroupOwners[groupID] = users
}

// SetApplications updates application storage.
func (s *Server) SetApplications(apps []*msgraph.Application) {
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

func deltaLink(r *http.Request, deltaToken string) string {
	u := *r.URL
	q := u.Query()
	q.Del("$deltatoken")
	q.Set("$deltatoken", "mock-delta-token#"+deltaToken)
	u.RawQuery = q.Encode()
	return u.String()
}
