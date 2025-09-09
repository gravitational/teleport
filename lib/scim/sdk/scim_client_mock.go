package scimsdk

import (
	"context"
	"slices"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
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
	s.Mu.Lock()
	defer s.Mu.Unlock()

	var userList []*User
	for _, user := range s.Users {
		userList = append(userList, user)
	}
	return &ListUserResponse{Users: userList}, nil
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
	s.Mu.Lock()
	defer s.Mu.Unlock()

	var groupList []*Group
	for _, group := range s.Groups {
		groupList = append(groupList, group)
	}
	return &ListGroupResponse{Groups: groupList}, nil
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

// ReplaceGroupMembers replaces a group's members.
func (s *ClientMock) ReplaceGroupMembers(ctx context.Context, id string, members []*GroupMember) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	group, exists := s.Groups[id]
	if !exists {
		return trace.NotFound("group with ID %q not found", id)
	}
	validMembers := make([]*GroupMember, 0, len(members))
	for _, m := range members {
		u, ok := s.Users[m.ExternalID]
		if ok {
			validMember := *m
			validMember.Display = u.DisplayName
			validMembers = append(validMembers, &validMember)
		}
	}
	group.Members = validMembers
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
