package test

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	scimschema "github.com/elimity-com/scim/schema"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/utils/set"
)

const (
	maxPageSize = 10
)

// UnifiedClientMock defines a mock for interacting with AWS Identity Center
// via bth the AWS API abd SCIM that manipulates the same data set
type UnifiedClientMock struct {
	icsdk.ClientMock

	// IncludeMembersInGroupListing some SCIM servers include a member list
	// in their group listing, others don't. Switch this behavior on and of as
	// the test demands.
	IncludeMembersInGroupListing bool
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
	if state.UserAssignments == nil {
		state.UserAssignments = make(map[string][]*icsdk.Assignment)
	}
	if state.GroupAssignments == nil {
		state.GroupAssignments = make(map[string][]*icsdk.Assignment)
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

func (s *UnifiedClientMock) AddUserToState(uID, username string) *icsdk.MockUser {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.ClientMock.MockedAWSStateType.AddUserToState(uID, username)
}

func byGroupID(id string) func(*icsdk.Group) bool {
	return func(g *icsdk.Group) bool {
		return g.ID == id
	}
}

func byUserID(id string) func(*icsdk.MockUser) bool {
	return func(u *icsdk.MockUser) bool {
		return aws.ToString(u.UserId) == id
	}
}

func byUserName(username string) func(*icsdk.MockUser) bool {
	return func(u *icsdk.MockUser) bool {
		return aws.ToString(u.User.UserName) == username
	}
}

func memberIsUser(userID string) func(*icsdk.GroupMember) bool {
	return func(m *icsdk.GroupMember) bool {
		return m.MemberID == userID
	}
}

func (s *scimClientMock) getUserByID(id string) *icsdk.MockUser {
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

func (s *scimClientMock) GetGroup(_ context.Context, id string) (*scimsdk.Group, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	icGroup := s.getGroupByID(id)
	if icGroup == nil {
		return nil, trace.NotFound("group with ID %q not found", id)
	}

	return s.toSCIMGroup(icGroup), nil
}

func (s *scimClientMock) toICGroup(scimGroup *scimsdk.Group) *icsdk.Group {
	return &icsdk.Group{
		DisplayName:     scimGroup.DisplayName,
		ID:              scimGroup.ID,
		IdentityStoreID: string(s.Info.IdentityStoreID),
	}
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
			Display:    aws.ToString(memberUser.UserName),
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

func (s *scimClientMock) toSCIMUser(mockUser *icsdk.MockUser) *scimsdk.User {
	return &scimsdk.User{
		ID:       aws.ToString(mockUser.UserId),
		UserName: aws.ToString(mockUser.UserName),
	}
}

func (s *scimClientMock) toMockUser(u *scimsdk.User) *icsdk.MockUser {
	mockUser := &icsdk.MockUser{
		Active: u.Active,
		User: identitystoretypes.User{
			IdentityStoreId: aws.String(string(s.Info.IdentityStoreID)),
			UserId:          aws.String(u.ID),
			UserName:        aws.String(u.UserName),
			DisplayName:     aws.String(u.DisplayName),
		},
	}

	if u.Name != nil {
		mockUser.User.Name = &identitystoretypes.Name{
			FamilyName: aws.String(u.Name.FamilyName),
			GivenName:  aws.String(u.Name.GivenName),
		}
	}

	return mockUser
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

	mockUser := s.toMockUser(user)
	s.Users = append(s.Users, mockUser)
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
	s.Mu.Lock()
	defer s.Mu.Unlock()

	icUser := s.getUserByID(user.ID)
	if icUser == nil {
		return nil, trace.NotFound("No such user")
	}

	update := s.toMockUser(user)
	(*icUser) = *update

	return s.toSCIMUser(icUser), nil
}

// ListUsers lists all Users.
func (s *scimClientMock) ListUsers(_ context.Context, queryOptions ...scimsdk.QueryOption) (*scimsdk.ListUserResponse, error) {
	var options scimsdk.QueryOptions
	for _, opt := range queryOptions {
		opt(&options)
	}

	if _, hasFilter := options.Filter(); hasFilter {
		return nil, trace.BadParameter("UnifiedClientMock SCIM ListUsers does not support filtering")
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	icUsers, startIndex := clipSlice(s.Users, &options)
	var scimUsers []*scimsdk.User
	for _, icUser := range icUsers {
		scimUsers = append(scimUsers, s.toSCIMUser(icUser))
	}

	response := &scimsdk.ListUserResponse{
		Schemas:      []string{scimschema.CoreGroupSchema().ID},
		TotalResults: int32(len(s.Users)),
		StartIndex:   int32(startIndex),
		ItemsPerPage: maxPageSize,
		Users:        scimUsers,
	}

	return response, nil
}

// CreateGroup creates a new group.
func (s *scimClientMock) CreateGroup(_ context.Context, scimGroup *scimsdk.Group) (*scimsdk.Group, error) {
	g := s.toICGroup(scimGroup)
	g.ID = uuid.NewString()

	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.Groups = append(s.Groups, g)

	return s.toSCIMGroup(g), nil
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

// ListGroups fetches the list of known groups from the server
func (s *scimClientMock) ListGroups(_ context.Context, queryOptions ...scimsdk.QueryOption) (*scimsdk.ListGroupResponse, error) {
	options := scimsdk.QueryOptions{}
	for _, fn := range queryOptions {
		fn(&options)
	}

	if _, hasFilter := options.Filter(); hasFilter {
		return nil, trace.BadParameter("UnifiedClientMock SCIM ListGroups does not support filtering")
	}

	s.Mu.Lock()
	defer s.Mu.Unlock()

	srcPage, startIndex := clipSlice(s.Groups, &options)

	// Convert the page of IC groups into SCIM groups
	dstPage := make([]*scimsdk.Group, len(srcPage))
	for i, icGroup := range srcPage {
		scimGroup := s.toSCIMGroup(icGroup)
		if !s.IncludeMembersInGroupListing {
			scimGroup.Members = nil
		}
		dstPage[i] = scimGroup
	}

	response := &scimsdk.ListGroupResponse{
		Schemas:      []string{scimschema.CoreGroupSchema().ID},
		TotalResults: int32(len(s.Groups)),
		StartIndex:   int32(startIndex),
		ItemsPerPage: maxPageSize,
		Groups:       dstPage,
	}

	return response, nil
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

// ListGroupMembers returns the current members of a group using the IC group
// membership data.
func (s *scimClientMock) ListGroupMembers(_ context.Context, id string) ([]*scimsdk.GroupMember, error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.getGroupByID(id) == nil {
		return nil, trace.NotFound("group with ID %q not found", id)
	}

	icMembers := s.GroupMemberships[id]
	members := make([]*scimsdk.GroupMember, 0, len(icMembers))
	for _, m := range icMembers {
		members = append(members, &scimsdk.GroupMember{
			ExternalID: m.MemberID,
			Type:       scimsdk.ResourceTypeUser,
		})
	}
	return members, nil
}

// PatchGroupMembers replaces a group's members applies the supplied [toAdd] and
// [toRemove] lists to the mocked AWS state
func (s *scimClientMock) PatchGroupMembers(_ context.Context, id string, toAdd, toRemove []*scimsdk.GroupMember) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	icGroup := s.getGroupByID(id)
	if icGroup == nil {
		return trace.NotFound("group with ID %q not found", id)
	}
	icMembers := s.GroupMemberships[id]

	// Remove all items in the toRemove list
	condemned := set.NewWithCapacity[string](len(toRemove))
	for _, m := range toRemove {
		condemned.Add(m.ExternalID)
	}
	icMembers = slices.DeleteFunc(icMembers, func(m *icsdk.GroupMember) bool {
		return condemned.Contains(m.MemberID)
	})

	// Add all of the members in the toAdd list
	for _, scimMember := range toAdd {
		icMember := s.getUserByID(scimMember.ExternalID)
		if icMember != nil {
			icMembers = append(icMembers, &icsdk.GroupMember{MemberID: aws.ToString(icMember.UserId)})
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

func clipSlice[T any](src []T, options *scimsdk.QueryOptions) ([]T, int) {
	// cget the page bounds from the query
	startIndex := 1
	pageSize := maxPageSize

	if i, ok := options.StartIndex(); ok {
		startIndex = i
	}
	startIndex-- // SCIM start indices are 1-based, converting to slice index

	if c, ok := options.Count(); ok {
		pageSize = min(maxPageSize, c)
	}

	// Clip the group list to the requested range
	var page []T
	if startIndex < len(src) {
		page = src[startIndex:]
	}
	return page[:min(pageSize, len(page))], startIndex + 1
}
