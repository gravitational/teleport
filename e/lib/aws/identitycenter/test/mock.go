package test

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
)

// UnifiedClientMock defines a mock for interacting with AWS Identity Center
// via bth the AWS API abd SCIM that manipulates the same data set
type UnifiedClientMock struct {
	icsdk.ClientMock
}

// scimClientMock wraps a UnifiedClientMock to expose the [scimsdk.Client]
// interface without clashing with the [icsdk.Client] interface.
type scimClientMock UnifiedClientMock

// NewUnifiedMockClient creates a mock IC client that can manipulate the same
// mocked Identity Center data via SCIM or the AWS API client interfaces
func NewUnifiedMockClient(state icsdk.MockedAWSStateType) *UnifiedClientMock {
	if state.GroupMemberships == nil {
		state.GroupMemberships = make(map[string][]*icsdk.GroupMember)
	}

	c := &UnifiedClientMock{
		ClientMock: icsdk.ClientMock{
			MockedAWSStateType: state,
		},
	}
	return c
}

// ViaSCIM returns a [scimsdk.Client] that can interact with the [UnifiedClientMock]'s
// backing data
func (s *UnifiedClientMock) ViaSCIM() scimsdk.Client {
	return (*scimClientMock)(s)
}

// ViaSCIM returns a [icsdk.Client] that can interact with the [UnifiedClientMock]'s
// backing data
func (s *UnifiedClientMock) ViaAPI() *icsdk.ClientMock {
	return &s.ClientMock
}

func byGroupID(id string) func(*icsdk.Group) bool {
	return func(g *icsdk.Group) bool {
		return g.ID == id
	}
}

func byUserID(id string) func(*icsdk.User) bool {
	return func(u *icsdk.User) bool {
		return u.ID == id
	}
}

func byUserName(username string) func(*icsdk.User) bool {
	return func(u *icsdk.User) bool {
		return u.UserName == username
	}
}

func memberIsUser(userID string) func(*icsdk.GroupMember) bool {
	return func(m *icsdk.GroupMember) bool {
		return m.MemberID == userID
	}
}

func (s *scimClientMock) getUserByID(id string) *icsdk.User {
	i := slices.IndexFunc(s.Users, byUserID(id))
	if i == -1 {
		return nil
	}
	return s.Users[i]
}

func (s *scimClientMock) getGroupByID(id string) *icsdk.Group {
	i := slices.IndexFunc(s.Groups, byGroupID(id))
	if i == -1 {
		return nil
	}
	return s.Groups[i]
}

func (s *scimClientMock) UpdateGroup(_ context.Context, scimGroup *scimsdk.Group) (*scimsdk.Group, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	icGroup := s.getGroupByID(scimGroup.ID)
	if icGroup == nil {
		return nil, trace.NotFound("group with ID %q not found", scimGroup.ID)
	}
	icGroup.DisplayName = scimGroup.DisplayName
	return scimGroup, nil
}

func (s *scimClientMock) GetUser(_ context.Context, id string) (*scimsdk.User, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	icUser := s.getUserByID(id)
	if icUser == nil {
		return nil, trace.NotFound("user with ID %q not found", id)
	}

	return s.toSCIMUser(icUser), nil
}

func (s *scimClientMock) toSCIMUser(icUser *icsdk.User) *scimsdk.User {
	return &scimsdk.User{
		ID:       icUser.ID,
		UserName: icUser.UserName,
	}
}

func (s *scimClientMock) GetGroup(_ context.Context, id string) (*scimsdk.Group, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	icGroup := s.getGroupByID(id)
	if icGroup == nil {
		return nil, trace.NotFound("group with ID %q not found", id)
	}

	return s.toSCIMGroup(icGroup), nil
}

func (s *scimClientMock) toSCIMGroup(icGroup *icsdk.Group) *scimsdk.Group {
	var scimMembers []*scimsdk.GroupMember
	for _, m := range s.GroupMemberships[icGroup.ID] {
		memberUser := s.getUserByID(m.MemberID)
		if memberUser == nil {
			continue
		}

		scimMembers = append(scimMembers, &scimsdk.GroupMember{
			ExternalID: m.MemberID,
			Display:    memberUser.UserName,
			Type:       scimsdk.ResourceTypeUser,
		})
	}

	scimGroup := &scimsdk.Group{
		ID:          icGroup.ID,
		DisplayName: icGroup.DisplayName,
		Members:     scimMembers,
	}

	return scimGroup
}

// CreateUser creates a new user.
func (s *scimClientMock) CreateUser(_ context.Context, user *scimsdk.User) (*scimsdk.User, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	if u := s.getUserByID(user.ID); u != nil {
		return nil, trace.BadParameter("user with ID %q already exists", user.ID)
	}

	icUser := &icsdk.User{
		ID:       user.ID,
		UserName: user.UserName,
	}
	s.Users = append(s.Users, icUser)
	return user, nil
}

// DeleteUser deletes a user.
func (s *scimClientMock) DeleteUser(_ context.Context, id string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	i := slices.IndexFunc(s.Users, byUserID(id))
	if i == -1 {
		return trace.Wrap(trace.NotFound("user with ID %q not found", id))
	}
	s.Users = slices.Delete(s.Users, i, i+1)

	for gid, members := range s.GroupMemberships {
		s.GroupMemberships[gid] = slices.DeleteFunc(members, memberIsUser(id))
	}
	return nil
}

// UpdateUser updates a user.
func (s *scimClientMock) UpdateUser(_ context.Context, user *scimsdk.User) (*scimsdk.User, error) {
	return nil, trace.NotImplemented("UnifiedClientMock.UpdateUser")
}

// ListUsers lists all Users.
func (s *scimClientMock) ListUsers(_ context.Context, queryOptions ...scimsdk.QueryOption) (*scimsdk.ListUserResponse, error) {
	return nil, trace.NotImplemented("UnifiedClientMock.ListUsers")
}

// CreateGroup creates a new group.
func (s *scimClientMock) CreateGroup(_ context.Context, group *scimsdk.Group) (*scimsdk.Group, error) {
	return nil, trace.NotImplemented("UnifiedClientMock.CreateGroup")
}

// DeleteGroup deletes a group.
func (s *scimClientMock) DeleteGroup(_ context.Context, id string) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	icGroup := s.getGroupByID(id)
	if icGroup == nil {
		return trace.NotFound("group with ID %q not found", id)
	}

	s.Groups = slices.DeleteFunc(s.Groups, func(g *icsdk.Group) bool { return g == icGroup })
	delete(s.GroupMemberships, id)
	return nil
}

// ListGroups lists all Groups.
func (s *scimClientMock) ListGroups(_ context.Context, queryOptions ...scimsdk.QueryOption) (*scimsdk.ListGroupResponse, error) {
	return nil, trace.NotImplemented("UnifiedClientMock.ListGroups")
}

// ReplaceGroupName replaces a group's name.
func (s *scimClientMock) ReplaceGroupName(_ context.Context, group *scimsdk.Group) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	existingGroup := s.getGroupByID(group.ID)
	if existingGroup == nil {
		return trace.NotFound("group with ID %q not found", group.ID)
	}
	existingGroup.DisplayName = group.DisplayName

	return nil
}

// ReplaceGroupMembers replaces a group's members.
func (s *scimClientMock) ReplaceGroupMembers(_ context.Context, id string, members []*scimsdk.GroupMember) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	icGroup := s.getGroupByID(id)
	if icGroup == nil {
		return trace.NotFound("group with ID %q not found", id)
	}

	icMembers := make([]*icsdk.GroupMember, 0, len(members))
	for _, scimMember := range members {
		icMember := s.getUserByID(scimMember.ExternalID)
		if icMember != nil {
			icMembers = append(icMembers, &icsdk.GroupMember{MemberID: icMember.ID})
		}
	}
	s.GroupMemberships[id] = icMembers
	return nil
}

// GetGroupByDisplayName returns a group by its display name.
func (s *scimClientMock) GetGroupByDisplayName(_ context.Context, displayName string) (*scimsdk.Group, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	i := slices.IndexFunc(s.Groups, func(g *icsdk.Group) bool { return g.DisplayName == displayName })
	if i == -1 {
		return nil, trace.NotFound("group with display name %q not found", displayName)
	}
	return s.toSCIMGroup(s.Groups[i]), nil
}

// GetUserByUserName returns a user by its username.
func (s *scimClientMock) GetUserByUserName(_ context.Context, userName string) (*scimsdk.User, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	i := slices.IndexFunc(s.Users, byUserName(userName))
	if i == -1 {
		return nil, trace.NotFound("user with username %q not found", userName)
	}
	return s.toSCIMUser(s.Users[i]), nil
}

// Ping pings the SCIM service.
func (s *scimClientMock) Ping(_ context.Context) error {
	return nil
}
