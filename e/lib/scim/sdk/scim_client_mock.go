package scimsdk

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/set"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

// NewSCIMClientMock creates a new mock SCIM client.
func NewSCIMClientMock() *ClientMock {
	return &ClientMock{
		Users:  make(map[string]*User),
		Groups: make(map[string]*Group),
	}
}

// ClientMock is a mock SCIM client.
type ClientMock struct {
	Users  map[string]*User
	Groups map[string]*Group
	Mu     sync.Mutex
}

func (s *ClientMock) UpdateGroup(ctx context.Context, group *Group) (*Group, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if _, exists := s.Groups[group.ID]; !exists {
		return nil, trace.NotFound("group with ID %q not found", group.ID)
	}
	s.Groups[group.ID] = group
	return group, nil
}

func (s *ClientMock) AddGroupMembers(ctx context.Context, groupID string, members ...*GroupMember) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	var group *Group
	var exists bool

	if group, exists = s.Groups[groupID]; !exists {
		return trace.NotFound("group with ID %q not found", groupID)
	}

	existingMembers := set.New(sliceutils.Map(group.Members, (*GroupMember).GetExternalID)...)
	for _, candidate := range members {
		if existingMembers.Contains(candidate.ExternalID) {
			continue
		}
		existingMembers.Add(candidate.ExternalID)
		group.Members = append(group.Members, candidate)
	}

	return nil
}

func (s *ClientMock) DeleteGroupMembers(ctx context.Context, groupID string, members ...*GroupMember) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	var group *Group
	var exists bool

	if group, exists = s.Groups[groupID]; !exists {
		return trace.NotFound("group with ID %q not found", groupID)
	}

	condemned := set.New(sliceutils.Map(members, (*GroupMember).GetExternalID)...)
	group.Members = slices.DeleteFunc(group.Members, func(m *GroupMember) bool {
		return condemned.Contains(m.ExternalID)
	})

	return nil
}

func (s *ClientMock) SetGroupMembers(ctx context.Context, groupID string, members ...*GroupMember) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	var group *Group
	var exists bool

	if group, exists = s.Groups[groupID]; !exists {
		return trace.NotFound("group with ID %q not found", groupID)
	}

	group.Members = slices.Clone(members)
	return nil
}

func (s *ClientMock) GetUser(ctx context.Context, id string) (*User, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	u, ok := s.Users[id]
	if !ok {
		return nil, trace.NotFound("user with ID %q not found", id)
	}
	return u, nil
}

func (s *ClientMock) GetGroup(ctx context.Context, id string) (*Group, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	u, ok := s.Groups[id]
	if !ok {
		return nil, trace.NotFound("user with ID %q not found", id)
	}
	return u, nil
}

// CreateUser creates a new user.
func (s *ClientMock) CreateUser(ctx context.Context, user *User) (*User, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	if _, exists := s.Users[user.ID]; exists {
		return nil, trace.BadParameter("user with ID %q already exists", user.ID)
	}
	s.Users[user.ID] = user
	return user, nil
}

// DeleteUser deletes a user.
func (s *ClientMock) DeleteUser(ctx context.Context, id string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if _, exists := s.Users[id]; !exists {
		return trace.Wrap(trace.NotFound("user with ID %q not found", id))
	}
	delete(s.Users, id)
	return nil
}

// UpdateUser updates a user.
func (s *ClientMock) UpdateUser(ctx context.Context, user *User) (*User, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if _, exists := s.Users[user.ID]; !exists {
		return nil, trace.NotFound("user with ID %q not found", user.ID)
	}
	s.Users[user.ID] = user
	return user, nil
}

// ListUsers lists all Users.
func (s *ClientMock) ListUsers(ctx context.Context, queryOptions ...QueryOption) (*ListUserResponse, error) {
	options := parseQueryOptions(queryOptions...)

	s.Mu.Lock()
	defer s.Mu.Unlock()

	startIndex, maxPageSize := options.extractRange(len(s.Users))

	response := &ListUserResponse{
		Schemas:      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		TotalResults: int32(len(s.Users)),
	}

	// As per RFC7644 §3.4.2.4, a page size request of 0 should be interpreted
	// as a query for just the total number of users.
	if maxPageSize == 0 {
		return response, nil
	}

	// Presort keys to force the map iteration order
	keys := slices.Collect(maps.Keys(s.Users))
	slices.Sort(keys)

	// Pull out the requested range of users
	for _, key := range keys[startIndex:] {
		response.Users = append(response.Users, s.Users[key])
		if len(response.Users) == maxPageSize {
			break
		}
	}
	response.StartIndex = int32(startIndex + 1)
	response.ItemsPerPage = int32(len(response.Users))
	return response, nil
}

// CreateGroup creates a new group.
func (s *ClientMock) CreateGroup(ctx context.Context, group *Group) (*Group, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if group.ID == "" {
		group.ID = uuid.New().String()
	}

	if _, exists := s.Groups[group.ID]; exists {
		return nil, trace.BadParameter("group with ID %q already exists", group.ID)
	}
	s.Groups[group.ID] = group
	return group, nil
}

// DeleteGroup deletes a group.
func (s *ClientMock) DeleteGroup(ctx context.Context, id string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if _, exists := s.Groups[id]; !exists {
		return trace.NotFound("group with ID %q not found", id)
	}
	delete(s.Groups, id)
	return nil
}

// ListGroups lists all Groups.
func (s *ClientMock) ListGroups(ctx context.Context, queryOptions ...QueryOption) (*ListGroupResponse, error) {
	options := parseQueryOptions(queryOptions...)

	s.Mu.Lock()
	defer s.Mu.Unlock()

	startIndex, maxPageSize := options.extractRange(len(s.Groups))

	response := &ListGroupResponse{
		Schemas:      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		TotalResults: int32(len(s.Groups)),
	}

	// As per RFC7644 §3.4.2.4, a page size request of 0 should be interpreted
	// as a query for just the total number of groups.
	if maxPageSize == 0 {
		return response, nil
	}

	// Presort keys to force the map iteration order
	keys := slices.Collect(maps.Keys(s.Groups))
	slices.Sort(keys)

	// Pull out the requested range of groups
	for _, key := range keys[startIndex:] {
		response.Groups = append(response.Groups, s.Groups[key])
		if len(response.Groups) == maxPageSize {
			break
		}
	}
	response.StartIndex = int32(startIndex + 1)
	response.ItemsPerPage = int32(len(response.Groups))
	return response, nil
}

// ReplaceGroupName replaces a group's name.
func (s *ClientMock) ReplaceGroupName(ctx context.Context, group *Group) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	existingGroup, exists := s.Groups[group.ID]
	if !exists {
		return trace.NotFound("group with ID %q not found", group.ID)
	}
	existingGroup.DisplayName = group.DisplayName
	return nil
}

// ListGroupMembers returns the current members of a group.
func (s *ClientMock) ListGroupMembers(ctx context.Context, id string) ([]*GroupMember, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	group, exists := s.Groups[id]
	if !exists {
		return nil, trace.NotFound("group with ID %q not found", id)
	}
	return slices.Clone(group.Members), nil
}

// PatchGroupMembers updates a group's member list by applying the supplied
// [toAdd] and [toRemove] lists.
func (s *ClientMock) PatchGroupMembers(ctx context.Context, id string, toAdd, toRemove []*GroupMember) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	group, exists := s.Groups[id]
	if !exists {
		return trace.NotFound("group with ID %q not found", id)
	}

	// Remove all items in the toRemove list
	condemned := set.NewWithCapacity[string](len(toRemove))
	for _, m := range toRemove {
		condemned.Add(m.ExternalID)
	}
	group.Members = slices.DeleteFunc(group.Members, func(m *GroupMember) bool {
		return condemned.Contains(m.ExternalID)
	})

	for _, m := range toAdd {
		u, ok := s.Users[m.ExternalID]
		if ok {
			validMember := *m
			validMember.Display = u.DisplayName
			group.Members = append(group.Members, &validMember)
		}
	}
	return nil
}

// GetGroupByDisplayName returns a group by its display name.
func (s *ClientMock) GetGroupByDisplayName(ctx context.Context, displayName string) (*Group, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	for _, group := range s.Groups {
		if group.DisplayName == displayName {
			// Take a copy of the group in order to avoid data races while
			// examining the member list outside of the client mutex
			result := *group
			result.Members = slices.Clone(group.Members)
			return &result, nil
		}
	}
	return nil, trace.NotFound("group with display name %q not found", displayName)
}

// GetUserByUserName returns a user by its username.
func (s *ClientMock) GetUserByUserName(ctx context.Context, userName string) (*User, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	for _, user := range s.Users {
		if user.UserName == userName {
			return user, nil
		}
	}
	return nil, trace.NotFound("user with username %q not found", userName)
}

// Ping pings the SCIM service.
func (s *ClientMock) Ping(ctx context.Context) error {
	return nil
}
