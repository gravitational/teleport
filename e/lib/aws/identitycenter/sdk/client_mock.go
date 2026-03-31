package sdk

import (
	"context"
	"slices"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"

	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

// NewClientMock creates and returns a new instance of ClientMock.
func NewClientMock(customMockData *MockedAWSStateType) *ClientMock {
	client := &ClientMock{
		MockedAWSStateType: NewMockedAWSState(WithDefaultUsersAndGroups),
	}
	if customMockData != nil {
		client.MockedAWSStateType = *customMockData
	}
	return client
}

// ClientMock is a mock implementation of the Client interface for testing purposes.
type ClientMock struct {
	Mu sync.Mutex
	MockedAWSStateType

	// MonkeyPatch allows tests to override the default behavior of a mock
	// instance.
	MonkeyPatch struct {
		DescribeInstance                     func(context.Context) (*InstanceInfo, error)
		ListPermissionSets                   func(context.Context) ([]*PermissionSet, error)
		CreateAccountAssignment              func(context.Context, *CreateAccountAssignmentRequest) (*AccountAssignmentResponse, error)
		DeleteAccountAssignment              func(context.Context, *DeleteAccountAssignmentRequest) (*AccountAssignmentResponse, error)
		CreateAccountAssignmentCounter       func()
		DeleteAccountAssignmentCounter       func()
		WaitForCreateAccountAssignmentResult func(context.Context, string) error
		WaitForDeleteAccountAssignmentResult func(context.Context, string) error
	}
}

// MockStateOption describes an option application function for constructing
// custom mocked AWS Identity Center states
type MockStateOption func(*MockedAWSStateType)

// WithAccounts sets the AWS account information for the mocked AWS Identity
// Center state
func WithAccounts(accts ...*Account) MockStateOption {
	return func(s *MockedAWSStateType) {
		s.Accounts = accts
	}
}

// UserOption is the signature for an option application function for use when
// constructing mock Identity Center users in a mock Identity Center state
type UserOption func(*MockUser)

// WithDisplayName sets the mock user's display name field
func WithDisplayName(n string) UserOption {
	return func(u *MockUser) {
		u.DisplayName = aws.String(n)
	}
}

// WithUser adds a new, optionally customized user to the mocked Identity Center
// state
func WithUser(id, username string, userOptions ...UserOption) MockStateOption {
	return func(s *MockedAWSStateType) {
		s.AddUserToState(id, username, userOptions...)
	}
}

// WithUsers overwrites the entire mocked Identity Center user list with the
// supplied users
func WithUsers(users ...*User) MockStateOption {
	return func(s *MockedAWSStateType) {
		s.Users = sliceutils.Map(users, s.toMockUser)
	}
}

// WithDefaultUsersAndGroups populates the mock Identity Center state with a
// default set of users and groups
func WithDefaultUsersAndGroups(s *MockedAWSStateType) {
	s.Users = []*MockUser{
		{
			Active: true,
			User: identitystoretypes.User{
				IdentityStoreId: aws.String(string(s.Info.IdentityStoreID)),
				UserId:          aws.String("user1"),
				UserName:        aws.String("user_one"),
			},
		},
		{
			Active: true,
			User: identitystoretypes.User{
				IdentityStoreId: aws.String(string(s.Info.IdentityStoreID)),
				UserId:          aws.String("user2"),
				UserName:        aws.String("user_two"),
			},
		},
	}

	s.UserAssignments = map[string][]*Assignment{
		"user1": {
			{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
			{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
		},
		"user2": {
			{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
		},
	}

	s.Groups = []*Group{
		{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
		{DisplayName: "Group2", ID: "group2", IdentityStoreID: "store1"},
	}

	s.GroupAssignments = map[string][]*Assignment{
		"group1": {
			{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
		},
		"group2": {
			{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
		},
	}

	s.GroupMemberships = map[string][]*GroupMember{
		"group1": {
			{MemberID: "user1"},
			{MemberID: "user2"},
		},
		"group2": {
			{MemberID: "user2"},
		},
	}
}

func toGroupMember(uID string) *GroupMember {
	return &GroupMember{MemberID: uID}
}

// WithGroup adds a new group to the mocked Identity Center state, with an
// optional member list.
func WithGroup(gID, displayName string, members ...string) MockStateOption {
	return func(s *MockedAWSStateType) {
		s.Groups = append(s.Groups, &Group{
			IdentityStoreID: string(s.Info.IdentityStoreID),
			ID:              gID,
			DisplayName:     displayName,
		})
		s.GroupMemberships[gID] = sliceutils.Map(members, toGroupMember)
	}
}

// WithGroups overwrites the entire group list in the mock Identity Center state
// with the supplied groups
func WithGroups(groups ...*Group) MockStateOption {
	return func(s *MockedAWSStateType) {
		s.Groups = groups
	}
}

// WithGroupMembers sets the members of a given group
func WithGroupMembers(gID string, members ...string) MockStateOption {
	return func(s *MockedAWSStateType) {
		s.GroupMemberships[gID] = sliceutils.Map(members, toGroupMember)
	}
}

// WithGroupMemberships overwrites the mock Identity Center's entire group
// membership database with the supplied membership info.
func WithGroupMemberships(members map[string][]*GroupMember) MockStateOption {
	return func(s *MockedAWSStateType) {
		s.GroupMemberships = members
	}
}

// WithPermissionSets overwrites the mock Identity Center state's list of
// Permission Sets with the supplied values.
func WithPermissionSets(pss ...*PermissionSet) MockStateOption {
	return func(s *MockedAWSStateType) {
		s.PermissionSets = pss
	}
}

// WithGroupAssignments overwrites the mock Identity Center's entire group
// permission assignment database with the supplied assignment map.
func WithGroupAssignments(assignments map[string][]*Assignment) MockStateOption {
	return func(s *MockedAWSStateType) {
		s.GroupAssignments = assignments
	}
}

func toSDKUser(u *MockUser) *User {
	return &User{
		ID:       aws.ToString(u.UserId),
		UserName: aws.ToString(u.UserName),
	}
}

func (s *MockedAWSStateType) toMockUser(u *User) *MockUser {
	return &MockUser{
		Active: true,
		User: identitystoretypes.User{
			IdentityStoreId: aws.String(string(s.Info.IdentityStoreID)),
			UserId:          aws.String(u.ID),
			UserName:        aws.String(u.UserName),
		},
	}
}

// NewMockedAWSState returns a default mock state.
func NewMockedAWSState(options ...MockStateOption) MockedAWSStateType {
	const defaultIdentityStoreID = "store1"

	state := MockedAWSStateType{
		Info: InstanceInfo{
			OwnerAccountID:  "2222222222",
			Name:            "Mock Identity Center Instance",
			IdentityStoreID: defaultIdentityStoreID,
			Status:          ssoadmintypes.InstanceStatusActive,
		},
		Accounts: []*Account{
			{Name: "Account1", ID: "1111111111", ARN: "arn:aws:iam::1111111111:account/Account1"},
			{Name: "Account2", ID: "2222222222", ARN: "arn:aws:iam::2222222222:account/Account2"},
		},
		PermissionSets: []*PermissionSet{
			{Name: "Admin", ARN: "arn:aws:sso:::permissionSet/Admin", Description: "Admin permissions"},
			{Name: "ReadOnly", ARN: "arn:aws:sso:::permissionSet/ReadOnly", Description: "Read-only permissions"},
		},
		GroupMemberships: make(map[string][]*GroupMember),
		UserAssignments:  make(map[string][]*Assignment),
		GroupAssignments: make(map[string][]*Assignment),
		AccountPermAssignments: map[string][]string{
			"1111111111": {"arn:aws:sso:::permissionSet/Admin", "arn:aws:sso:::permissionSet/ReadOnly"},
			"2222222222": {"arn:aws:sso:::permissionSet/ReadOnly"},
		},
	}

	for _, applyOption := range options {
		applyOption(&state)
	}

	return state
}

// clonePtrSlice clones a slice of pointers to T, making 1-level-deep copies of
// the pointed-to objects.
func clonePtrSlice[T any](s []*T) []*T {
	if s == nil {
		return nil
	}
	cloneItem := func(src *T) *T {
		cpy := *src
		return &cpy
	}
	return sliceutils.Map(s, cloneItem)
}

type MockUser struct {
	identitystoretypes.User
	Active bool
}

// MockedAWSStateType is a struct that holds the mocked AWS state.
type MockedAWSStateType struct {
	// Info holds information about the Identoty Center instance
	Info InstanceInfo
	// Accounts is a list of mocked accounts.
	Accounts []*Account
	// PermissionSets is a list of mocked permission sets.
	PermissionSets []*PermissionSet
	// Users is a list of mocked users.
	Users []*MockUser
	// Groups is a list of mocked groups.
	Groups []*Group
	// GroupMemberships is a map of group ID to a list of group members.
	GroupMemberships map[string][]*GroupMember
	// UserAssignments is a map of user ID to a list of assignments.
	UserAssignments map[string][]*Assignment
	// GroupAssignments is a map of group ID to a list of assignments.
	GroupAssignments map[string][]*Assignment
	// AccountPermAssignments is a map of account ID to a list of permission set ARNs.
	AccountPermAssignments map[string][]string
}

// AddUserToState adds a new [MockUser] to tge mocked Identity Center state
func (s *MockedAWSStateType) AddUserToState(userID, username string, options ...UserOption) *MockUser {
	user := &MockUser{
		Active: true,
		User: identitystoretypes.User{
			IdentityStoreId: aws.String(string(s.Info.IdentityStoreID)),
			UserId:          aws.String(userID),
			UserName:        aws.String(username),
		},
	}
	for _, applyOption := range options {
		applyOption(user)
	}
	s.Users = append(s.Users, user)
	return user
}

// AddGroupToState adds a new [Group] to the mocked Identity Center state
func (s *MockedAWSStateType) AddGroupToState(groupID, displayName string) *Group {
	group := &Group{
		IdentityStoreID: string(s.Info.IdentityStoreID),
		ID:              groupID,
		DisplayName:     displayName,
	}
	s.Groups = append(s.Groups, group)
	return group
}

// DescribeInstance returns a mocked InstanceInfo
func (c *ClientMock) DescribeInstance(ctx context.Context) (*InstanceInfo, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.DescribeInstance != nil {
		return c.MonkeyPatch.DescribeInstance(ctx)
	}
	info := c.Info
	return &info, nil
}

// ListAccounts returns a list of mocked accounts.
func (c *ClientMock) ListAccounts(_ context.Context) ([]*Account, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return clonePtrSlice(c.Accounts), nil
}

// ListPermissionSetARNsForAccount returns a list of permission set ARNs assigned to an account.
func (c *ClientMock) ListPermissionSetARNsForAccount(_ context.Context, accountID string) ([]string, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return slices.Clone(c.AccountPermAssignments[accountID]), nil
}

// ListPermissionSets returns a list of mocked permission sets.
func (c *ClientMock) ListPermissionSets(ctx context.Context) ([]*PermissionSet, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.ListPermissionSets != nil {
		return c.MonkeyPatch.ListPermissionSets(ctx)
	}
	return clonePtrSlice(c.PermissionSets), nil
}

// ListGroups returns a list of mocked groups.
func (c *ClientMock) ListGroups(ctx context.Context) ([]*Group, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return clonePtrSlice(c.Groups), nil
}

// ListGroupMemberships returns a list of group members for a given group ID.
func (c *ClientMock) ListGroupMemberships(_ context.Context, groupID string) ([]*GroupMember, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return clonePtrSlice(c.GroupMemberships[groupID]), nil
}

// ListUsers returns a list of mocked users.
func (c *ClientMock) ListUsers(context.Context) ([]*User, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return sliceutils.Map(c.Users, toSDKUser), nil
}

// ListUserAssignments returns a list of permission assignments for a given user ID.
func (c *ClientMock) ListUserAssignments(_ context.Context, userID string) ([]*Assignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if u := c.getUserByID(userID); u == nil {
		return nil, trace.NotFound("No such user %s", userID)
	}
	return clonePtrSlice(c.UserAssignments[userID]), nil
}

// ListGroupsAssignments returns a list of permission assignments for a given group ID.
func (c *ClientMock) ListGroupsAssignments(_ context.Context, groupID string) ([]*Assignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if g := c.getGroupByID(groupID); g == nil {
		return nil, trace.NotFound("No such group %s", groupID)
	}
	return clonePtrSlice(c.GroupAssignments[groupID]), nil
}

// WaitForDeleteAccountAssignmentResult waits until the account assignment creation reaches a terminal state.
func (c *ClientMock) WaitForCreateAccountAssignmentResult(ctx context.Context, requestID string) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.WaitForCreateAccountAssignmentResult != nil {
		return c.MonkeyPatch.WaitForCreateAccountAssignmentResult(ctx, requestID)
	}
	return nil
}

// CreateAccountAssignment adds a new assignment based on the request parameters.
func (c *ClientMock) CreateAccountAssignment(_ context.Context, req *CreateAccountAssignmentRequest) (*AccountAssignmentResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.CreateAccountAssignmentCounter != nil {
		c.MonkeyPatch.CreateAccountAssignmentCounter()
	}

	switch req.PrincipalType {
	case ssoadmintypes.PrincipalTypeUser:
		c.UserAssignments[req.PrincipalID] = append(c.UserAssignments[req.PrincipalID], &Assignment{
			AccountID:        req.AccountID,
			PermissionSetARN: req.PermissionSetARN,
			PrincipalType:    req.PrincipalType,
		})
	case ssoadmintypes.PrincipalTypeGroup:
		c.GroupAssignments[req.PrincipalID] = append(c.GroupAssignments[req.PrincipalID], &Assignment{
			AccountID:        req.AccountID,
			PermissionSetARN: req.PermissionSetARN,
			PrincipalType:    req.PrincipalType,
		})
	default:
		return nil, trace.BadParameter("unsupported principal type")
	}

	return &AccountAssignmentResponse{
		RequestID: "mockRequestID",
		Status:    ssoadmintypes.StatusValuesSucceeded,
	}, nil
}

// DeleteAccountAssignment removes an assignment for the specified user.
func (c *ClientMock) DeleteAccountAssignment(_ context.Context, req *DeleteAccountAssignmentRequest) (*AccountAssignmentResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.DeleteAccountAssignmentCounter != nil {
		c.MonkeyPatch.DeleteAccountAssignmentCounter()
	}

	var principalAssignments map[string][]*Assignment
	var curr map[string][]*Assignment
	switch req.PrincipalType {
	case ssoadmintypes.PrincipalTypeUser:
		principalAssignments = c.UserAssignments
		curr = c.UserAssignments
	case ssoadmintypes.PrincipalTypeGroup:
		principalAssignments = c.GroupAssignments
		curr = c.GroupAssignments
	default:
		return nil, trace.BadParameter("unsupported principal target type %T", req.PrincipalType)
	}

	assignments, ok := principalAssignments[req.PrincipalID]
	if !ok {
		return nil, trace.BadParameter("assignment not found")
	}

	for i, assignment := range assignments {
		if assignment.PermissionSetARN == req.PermissionSetARN && assignment.AccountID == req.AccountID {
			curr[req.PrincipalID] = slices.Delete(assignments, i, i+1)
			return &AccountAssignmentResponse{
				RequestID: "mockRequestID",
				Status:    ssoadmintypes.StatusValuesSucceeded,
			}, nil
		}
	}
	return nil, trace.NotFound("assignment not found")
}

// WaitForDeleteAccountAssignmentResult waits until the account assignment deletion reaches a terminal state.
func (c *ClientMock) WaitForDeleteAccountAssignmentResult(ctx context.Context, requestID string) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.WaitForDeleteAccountAssignmentResult != nil {
		return c.MonkeyPatch.WaitForDeleteAccountAssignmentResult(ctx, requestID)
	}
	return nil
}

func (c *ClientMock) ListAssignments(_ context.Context, principalID string, principalType ssoadmintypes.PrincipalType) ([]*Assignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	switch principalType {
	case ssoadmintypes.PrincipalTypeUser:
		if u := c.getUserByID(principalID); u == nil {
			return nil, trace.NotFound("No such user %s", principalID)
		}

		var userAssignments []*Assignment
		for _, v := range c.UserAssignments[principalID] {
			assignment := *v
			assignment.PrincipalType = ssoadmintypes.PrincipalTypeUser
			userAssignments = append(userAssignments, &assignment)
		}
		for groupID, members := range c.GroupMemberships {
			for _, member := range members {
				if member.MemberID == principalID {
					for _, v := range c.GroupAssignments[groupID] {
						assignment := *v
						assignment.PrincipalType = ssoadmintypes.PrincipalTypeGroup
						userAssignments = append(userAssignments, &assignment)
					}
				}
			}
		}
		return userAssignments, nil
	case ssoadmintypes.PrincipalTypeGroup:
		if g := c.getGroupByID(principalID); g == nil {
			return nil, trace.NotFound("no such group %q", principalID)
		}
		var groupAssignments []*Assignment
		for _, v := range c.GroupAssignments[principalID] {
			assignment := *v
			assignment.PrincipalType = ssoadmintypes.PrincipalTypeGroup
			groupAssignments = append(groupAssignments, &assignment)
		}
		return groupAssignments, nil
	default:
		return nil, trace.BadParameter("unsupported principal type %q", principalType)
	}
}

// ValidateResourceSyncCredential only validates DescribeInstance permission.
func (c *ClientMock) ValidateResourceSyncCredential(ctx context.Context) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	if c.MonkeyPatch.DescribeInstance != nil {
		if _, err := c.MonkeyPatch.DescribeInstance(ctx); err != nil {
			return err
		}
	}

	return nil
}

// GetMockAccount fetches a mock AWS account from the mock's backing state.
// Returns nil if no such account exists
func (c *ClientMock) GetMockAccount(id string) *Account {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.getAccountByID(id)
}

// RenameMockAccount looks up a mock AWS Account by ID and, if present, renames
// it
func (c *ClientMock) RenameMockAccount(id, newName string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if a := c.getAccountByID(id); a != nil {
		a.Name = newName
	}
}

// DeleteMockGroup removes a group and its account assignments from the mocked
// data set.
func (c *ClientMock) DeleteMockGroup(id string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.Groups = slices.DeleteFunc(c.Groups, byGroupID(id))
	delete(c.GroupMemberships, id)
	delete(c.GroupAssignments, id)
}

// DeleteMockUser removes a user and their account assignments from the mocked
// data set.
func (c *ClientMock) DeleteMockUser(id string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	c.Users = slices.DeleteFunc(c.Users, byUserID(id))
	delete(c.UserAssignments, id)
}

func (c *ClientMock) getAccountByID(id string) *Account {
	if i := slices.IndexFunc(c.Accounts, byAccountID(id)); i != -1 {
		return c.Accounts[i]
	}
	return nil
}

func (c *ClientMock) getUserByID(id string) *MockUser {
	if i := slices.IndexFunc(c.Users, byUserID(id)); i != -1 {
		return c.Users[i]
	}
	return nil
}

func (c *ClientMock) getGroupByID(id string) *Group {
	if i := slices.IndexFunc(c.Groups, byGroupID(id)); i != -1 {
		return c.Groups[i]
	}
	return nil
}

func byAccountID(id string) func(*Account) bool {
	return func(a *Account) bool { return a.ID == id }
}

func byUserID(id string) func(*MockUser) bool {
	return func(u *MockUser) bool { return aws.ToString(u.UserId) == id }
}

func byGroupID(id string) func(*Group) bool {
	return func(g *Group) bool { return g.ID == id }
}
