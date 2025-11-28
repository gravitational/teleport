package msgraphtest

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sync"

	"github.com/gravitational/teleport/lib/msgraph"
)

var (
	applicationByAppIDPattern = regexp.MustCompile(`^/v1\.0/applications\(appId='([^']+)'\)$`)
)

type MockServer struct {
	mu      sync.RWMutex
	Storage *Storage
}

type MockServerOption func(*MockServer)

func WithStorage(storage *Storage) MockServerOption {
	return func(s *MockServer) {
		s.Storage = storage
	}
}

func NewMockServer(opts ...MockServerOption) *MockServer {
	// By default, use storage populated with default mock data
	s := &MockServer{
		Storage: NewDefaultStorage(),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *MockServer) SetUsers(users []*msgraph.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, user := range users {
		if user.ID != nil {
			s.Storage.Users[*user.ID] = user
		}
	}
}

func (s *MockServer) SetGroups(groups []*msgraph.Group) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, group := range groups {
		if group.ID != nil {
			s.Storage.Groups[*group.ID] = group
		}
	}
}

func (s *MockServer) SetGroupMembers(groupID string, members []msgraph.GroupMember) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Storage.GroupMembers[groupID] = members
}

func (s *MockServer) SetApplications(apps []*msgraph.Application) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, app := range apps {
		if app.AppID != nil {
			s.Storage.Applications[*app.AppID] = app
		}
	}
}

func (s *MockServer) Handler() http.Handler {
	r := http.NewServeMux()

	r.HandleFunc("GET /v1.0/users", s.handleListUsers)
	r.HandleFunc("GET /v1.0/groups", s.handleListGroups)
	r.HandleFunc("GET /v1.0/groups/{id}/members", s.handleListGroupMembers)
	r.HandleFunc("/v1.0/", s.handleCatchAll)

	return r
}

func (s *MockServer) handleListUsers(w http.ResponseWriter, r *http.Request) {
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

func (s *MockServer) handleListGroups(w http.ResponseWriter, r *http.Request) {
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

func (s *MockServer) handleListGroupMembers(w http.ResponseWriter, r *http.Request) {
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

// handleGetApplication handles GET /v1.0/applications(appId='...') requests.
func (s *MockServer) handleGetApplication(w http.ResponseWriter, r *http.Request, appID string) {
	s.mu.RLock()
	app, ok := s.Storage.Applications[appID]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "application not found", http.StatusNotFound)
		return
	}

	jsonResponse(w, app)
}

// handleCatchAll handles other endpoints like applications(appId='app-id').
func (s *MockServer) handleCatchAll(w http.ResponseWriter, r *http.Request) {
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

func jsonResponse(writer http.ResponseWriter, data interface{}) {
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(data); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
