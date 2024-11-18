package sdk

import (
	"context"
	"sync"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"
)

// NewClientMock creates and returns a new instance of ClientMock.
func NewClientMock(customMockData *MockedAWSStateType) *ClientMock {
	client := &ClientMock{
		MockedAWSStateType: NewMockedAWSState(),
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
		DescribeInstance   func(context.Context) (*InstanceInfo, error)
		ListPermissionSets func(context.Context) ([]*PermissionSet, error)
	}
}

// NewMockedAWSState returns a default mock state.
func NewMockedAWSState() MockedAWSStateType {
	return MockedAWSStateType{
		Info: InstanceInfo{
			OwnerAccountID:  "2222222222",
			Name:            "Mock Identity Center Instance",
			IdentityStoreID: "store1",
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
		Users: []*User{
			{ID: "user1", UserName: "user_one"},
			{ID: "user2", UserName: "user_two"},
		},
		Groups: []*Group{
			{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
			{DisplayName: "Group2", ID: "group2", IdentityStoreID: "store1"},
		},
		GroupMemberships: map[string][]*GroupMember{
			"group1": {
				{MemberID: "user1"},
				{MemberID: "user2"},
			},
			"group2": {
				{MemberID: "user2"},
			},
		},
		UserAssignments: map[string][]*Assignment{
			"user1": {
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
				{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
			},
			"user2": {
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
			},
		},
		GroupAssignments: map[string][]*Assignment{
			"group1": {
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
			},
			"group2": {
				{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
			},
		},
		AccountPermAssignments: map[string][]string{
			"1111111111": {"arn:aws:sso:::permissionSet/Admin", "arn:aws:sso:::permissionSet/ReadOnly"},
			"2222222222": {"arn:aws:sso:::permissionSet/ReadOnly"},
		},
	}
}

// DefaultMockedData defines a default MockedAWSStateType values used for
// testing mocked AWS state.
var DefaultMockedData = MockedAWSStateType{
	Info: InstanceInfo{
		OwnerAccountID:  "2222222222",
		Name:            "Mock Identity Center Instance",
		IdentityStoreID: "store1",
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
	Users: []*User{
		{ID: "user1", UserName: "user_one"},
		{ID: "user2", UserName: "user_two"},
	},
	Groups: []*Group{
		{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
		{DisplayName: "Group2", ID: "group2", IdentityStoreID: "store1"},
	},
	GroupMemberships: map[string][]*GroupMember{
		"group1": {
			{MemberID: "user1"},
			{MemberID: "user2"},
		},
		"group2": {
			{MemberID: "user2"},
		},
	},
	UserAssignments: map[string][]*Assignment{
		"user1": {
			{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
			{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
		},
		"user2": {
			{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
		},
	},
	GroupAssignments: map[string][]*Assignment{
		"group1": {
			{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
		},
		"group2": {
			{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
		},
	},
	AccountPermAssignments: map[string][]string{
		"1111111111": {"arn:aws:sso:::permissionSet/Admin", "arn:aws:sso:::permissionSet/ReadOnly"},
		"2222222222": {"arn:aws:sso:::permissionSet/ReadOnly"},
	},
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
	Users []*User
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
func (c *ClientMock) ListAccounts(ctx context.Context) ([]*Account, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.Accounts, nil
}

// ListAccountsWithAssignedPermissionSetARNs returns a list of mocked accounts with assigned permission set ARNs.
func (c *ClientMock) ListAccountsWithAssignedPermissionSetARNs(ctx context.Context) ([]*AccountWithPermissionSetARNs, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	var out []*AccountWithPermissionSetARNs
	for _, acc := range c.Accounts {
		assignedPermSets, ok := c.AccountPermAssignments[acc.ID]
		if !ok {
			assignedPermSets = []string{}
		}
		out = append(out, &AccountWithPermissionSetARNs{
			Account:           acc,
			PermissionSetARNs: assignedPermSets,
		})
	}
	return out, nil
}

// ListPermissionSetARNsForAccount returns a list of permission set ARNs assigned to an account.
func (c *ClientMock) ListPermissionSetARNsForAccount(ctx context.Context, accountID string) ([]string, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.AccountPermAssignments[accountID], nil
}

// ListPermissionSets returns a list of mocked permission sets.
func (c *ClientMock) ListPermissionSets(ctx context.Context) ([]*PermissionSet, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.ListPermissionSets != nil {
		return c.MonkeyPatch.ListPermissionSets(ctx)
	}
	return c.PermissionSets, nil
}

// ListGroups returns a list of mocked groups.
func (c *ClientMock) ListGroups(ctx context.Context) ([]*Group, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.Groups, nil
}

// ListGroupMemberships returns a list of group members for a given group ID.
func (c *ClientMock) ListGroupMemberships(ctx context.Context, groupID string) ([]*GroupMember, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.GroupMemberships[groupID], nil
}

// ListGroupsWithMembers returns a list of mocked groups with enlisted members.
func (c *ClientMock) ListGroupsWithMembers(ctx context.Context) ([]*GroupWithMembers, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	var out []*GroupWithMembers
	for _, group := range c.Groups {
		members := c.GroupMemberships[group.ID]
		out = append(out, &GroupWithMembers{
			Group:   group,
			Members: members,
		})
	}
	return out, nil
}

// ListGroupsWithAccountAndPermAssignment returns a list of mocked groups with account and permission assignments.
func (c *ClientMock) ListGroupsWithAccountAndPermAssignment(ctx context.Context) ([]*GroupWithAssignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	var out []*GroupWithAssignment
	for _, group := range c.Groups {
		assignments, ok := c.GroupAssignments[group.ID]
		if !ok {
			assignments = []*Assignment{}
		}
		out = append(out, &GroupWithAssignment{
			Group:       group,
			Assignments: assignments,
		})
	}
	return out, nil
}

// ListUsers returns a list of mocked users.
func (c *ClientMock) ListUsers(ctx context.Context) ([]*User, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.Users, nil
}

// ListUsersWithAccountAndPermAssignment returns a list of mocked users with account and permission assignments.
func (c *ClientMock) ListUsersWithAccountAndPermAssignment(ctx context.Context) ([]*UserWithAssignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	var out []*UserWithAssignment
	for _, user := range c.Users {
		assignments, ok := c.UserAssignments[user.ID]
		if !ok {
			assignments = []*Assignment{}
		}
		out = append(out, &UserWithAssignment{
			User:        user,
			Assignments: assignments,
		})
	}
	return out, nil
}

// ListUserAssignments returns a list of permission assignments for a given user ID.
func (c *ClientMock) ListUserAssignments(ctx context.Context, userID string) ([]*Assignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.UserAssignments[userID], nil
}

// ListGroupsAssignments returns a list of permission assignments for a given group ID.
func (c *ClientMock) ListGroupsAssignments(ctx context.Context, groupID string) ([]*Assignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.GroupAssignments[groupID], nil
}

// WaitForAccountAssignmentResult waits until the account assignment reaches a terminal state.
func (c *ClientMock) WaitForAccountAssignmentResult(ctx context.Context, requestID string) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return nil
}

// CreateAccountAssignment adds a new assignment based on the request parameters.
func (c *ClientMock) CreateAccountAssignment(ctx context.Context, req *CreateAccountAssignmentRequest) (*AccountAssignmentResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	switch req.PrincipalType {
	case ssoadmintypes.PrincipalTypeUser:
		c.UserAssignments[req.PrincipalID] = append(c.UserAssignments[req.PrincipalID], &Assignment{
			AccountID:        req.AccountID,
			PermissionSetARN: req.PermissionSetARN,
		})
	case ssoadmintypes.PrincipalTypeGroup:
		c.GroupAssignments[req.PrincipalID] = append(c.GroupAssignments[req.PrincipalID], &Assignment{
			AccountID:        req.AccountID,
			PermissionSetARN: req.PermissionSetARN,
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
func (c *ClientMock) DeleteAccountAssignment(ctx context.Context, req *DeleteAccountAssignmentRequest) (*AccountAssignmentResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

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
			curr[req.PrincipalID] = append(assignments[:i], assignments[i+1:]...)
			return &AccountAssignmentResponse{
				RequestID: "mockRequestID",
				Status:    ssoadmintypes.StatusValuesSucceeded,
			}, nil
		}
	}
	return nil, trace.NotFound("assignment not found")
}

func (c *ClientMock) ListAssignments(ctx context.Context, principalID string, principalType ssoadmintypes.PrincipalType) ([]*Assignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	switch principalType {
	case ssoadmintypes.PrincipalTypeUser:
		return c.UserAssignments[principalID], nil
	case ssoadmintypes.PrincipalTypeGroup:
		return c.GroupAssignments[principalID], nil
	default:
		return nil, trace.BadParameter("unsupported principal type %q", principalType)
	}
}
