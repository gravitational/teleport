package sdk

import (
	ssoAdminTypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
)

// PermissionSet represents permission set from Identity Center.
type PermissionSet struct {
	// Name is the name of a permission set.
	Name string
	// ARN is an ARN value of a permission set.
	ARN string
	// Description is the description of a permission set.
	Description string
}

// PermissionSetMap is map of permission set where map key is
// permission set ARN.
type PermissionSetMap map[string]*PermissionSet

// Account represents Identity Center account.
type Account struct {
	// Name is the name of an account.
	Name string
	// ID is an ID of an account.
	ID string
	// ARN is an ARN value of an account.
	ARN string
}

// AccountMap is map of Identity Center account where map key is account ID.
type AccountMap map[string]*Account

// AccountWithPermissionSetARNs represents Identity Center account with
// permission sets.
type AccountWithPermissionSetARNs struct {
	*Account
	// PermissionSets is a list of permission set assigned to an account.
	PermissionSetARNs []string
}

// Assigment represents permission assignment (permission set + account).
type Assigment struct {
	// AccountID is the ID of an assigned account.
	AccountID string
	// PermissionSetARN is an ARN value of the assigned permission set.
	PermissionSetARN string
}

// User represents Identity Center users.
type User struct {
	// ID is the user ID.
	ID string
	// UserName is the user's username.
	UserName string
}

// Group represents Identity Center group.
type Group struct {
	// DisplayName is the display name of a group.
	DisplayName string
	// ID is the group ID.
	ID string
	// IdentityStoreID is the identity store ID of a group.
	IdentityStoreID string
}

// GroupWithAssignment represents Identity Center group with
// permission assignment.
type GroupWithAssignment struct {
	*Group
	// Assignments is a list of account name and permission set
	// assigned to a group.
	Assignments []*Assigment
}

// GroupWithMembers represents identity Center groups with
// enlisted members ID (i.e. users).
type GroupWithMembers struct {
	*Group
	// Members is list of group members.
	Members []*GroupMember
}

// GroupMember represents Identify Center group member.
type GroupMember struct {
	// MemberID is an ID of a member assigned to a group.
	MemberID string
}

// UserWithAssignment represents Identity Center user with
// permission assignment.
type UserWithAssignment struct {
	*User
	// Assignments is a list of account name and permission set
	// assigned to a group.
	Assignments []*Assigment
}

// IdentityStoreID holds an AWS Identity Store ID
type IdentityStoreID string

// InstanceInfo describes information about the configured Identity Center
// instance
type InstanceInfo struct {
	OwnerAccountID  string
	Name            string
	IdentityStoreID IdentityStoreID
	Status          ssoAdminTypes.InstanceStatus
}
