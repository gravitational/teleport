package awsicui

import (
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
)

// FetchICResourceRequest defines type for fetch IC resource request.
type FetchICResourceRequest struct {
	IntegrationName string `json:"integrationName"`
	Arn             string `json:"arn"`
	Region          string `json:"region"`
}

// PermissionSet represents permission set from Identity Center.
type PermissionSet struct {
	// Name is the name of a permission set.
	Name string `json:"name"`
	// ARN is an ARN value of a permission set.
	ARN string `json:"arn"`
	// Description is the description of a permission set.
	Description string `json:"description"`
}

// PermissionSets transforms icsdk PermissionSet to awsicui PermissionSet.
func PermissionSets(in []*icsdk.PermissionSet) []*PermissionSet {
	out := make([]*PermissionSet, 0, len(in))
	for _, p := range in {
		out = append(out, &PermissionSet{
			Name:        p.Name,
			ARN:         p.ARN,
			Description: p.Description,
		})
	}
	return out
}

// PermissionSetsFromARN transforms slice of permission set ARN to awsicui PermissionSet.
func PermissionSetsFromARN(in []string, permMap icsdk.PermissionSetMap) []*PermissionSet {
	out := make([]*PermissionSet, 0, len(in))
	for _, arn := range in {
		out = append(out, &PermissionSet{
			Name:        permMap[arn].Name,
			ARN:         permMap[arn].ARN,
			Description: permMap[arn].Description,
		})
	}
	return out
}

// Account represents Identity Center account.
type Account struct {
	// Name is the name of an account.
	Name string `json:"name"`
	// ID is an ID of an account.
	ID string `json:"id"`
	// ARN is an ARN value of an account.
	ARN string `json:"arn"`
}

// Accounts transforms icsdk Account to awsicui Account.
func Accounts(in []icsdk.Account) []Account {
	out := make([]Account, 0, len(in))
	for _, a := range in {
		out = append(out, Account{
			Name: a.Name,
			ARN:  a.ARN,
			ID:   a.ID,
		})
	}
	return out
}

// AccountWithPermissionSets represents Identity Center account with
// permission sets.
type AccountWithPermissionSet struct {
	*Account
	// PermissionSets is a list of permission set assigned to an account.
	PermissionSets []*PermissionSet `json:"permissionSets"`
}

// AccountWithPermissionSets transforms icsdk AccountWithPermissionSetARNs to awsicui AccountWithPermissionSet.
func AccountWithPermissionSets(in []*icsdk.AccountWithPermissionSetARNs, permMap icsdk.PermissionSetMap) []*AccountWithPermissionSet {
	out := make([]*AccountWithPermissionSet, 0, len(in))
	for _, a := range in {
		out = append(out, &AccountWithPermissionSet{
			Account: &Account{
				Name: a.Name,
				ID:   a.ID,
				ARN:  a.ARN,
			},
			PermissionSets: PermissionSetsFromARN(a.PermissionSetARNs, permMap),
		})
	}
	return out
}

// AccountAndPermAssignment represents permission assignment (permission set + account).
type AccountAndPermAssignment struct {
	// AccountName is the name of an assigned account.
	AccountName string `json:"accountName"`
	// AccountID is the ID of an assigned account.
	AccountID string `json:"accountID"`
	// PermissionSetName is the name of the assigned permission set.
	PermissionSetName string `json:"permissionSetName"`
	// PermissionSetARN is an ARN value of the assigned permission set.
	PermissionSetARN string `json:"permissionSetARN"`
}

// AccountAndPermAssignments transforms icsdk Assigment to awsicui AccountAndPermAssignment
// with enriched account and permission set data.
func AccountAndPermAssignments(in []*icsdk.Assigment, accountMap icsdk.AccountMap, permSetMap icsdk.PermissionSetMap) []*AccountAndPermAssignment {
	out := make([]*AccountAndPermAssignment, 0, len(in))
	for _, a := range in {
		out = append(out, &AccountAndPermAssignment{
			AccountName:       accountMap[a.AccountID].Name,
			AccountID:         a.AccountID,
			PermissionSetName: permSetMap[a.PermissionSetARN].Name,
			PermissionSetARN:  a.PermissionSetARN,
		})
	}
	return out
}

// User represents Identity Center users.
type User struct {
	// ID is the user ID.
	ID string
	// UserName is the user's username.
	UserName string
}

// Users transforms icsdk User to awsicui User.
func Users(in []icsdk.User) []User {
	out := make([]User, 0, len(in))
	for _, u := range in {
		out = append(out, User{
			UserName: u.UserName,
			ID:       u.ID,
		})
	}
	return out
}

// Group represents Identity Center group.
type Group struct {
	// Name is the display name of a group.
	Name string `json:"name"`
	// ID is the group ID.
	ID string `json:"id"`
}

// Groups transforms icsdk Group to awsicui Group.
func Groups(in []*icsdk.Group) []*Group {
	out := make([]*Group, 0, len(in))
	for _, g := range in {
		out = append(out, &Group{
			Name: g.DisplayName,
			ID:   g.ID,
		})
	}
	return out
}

// GroupWithAccountAndPermAssignment represents Identity Center group with
// permission assignment.
type GroupWithAccountAndPermAssignment struct {
	*Group
	// Assignments is a list of account name and permission set
	// assigned to a group.
	Assignments []*AccountAndPermAssignment `json:"assignments"`
}

// GroupAccountAndPermAssigments transforms icsdk GroupWithAssignment to awsicui GroupWithAccountAndPermAssignment with enriched
// account and permission set data.
func GroupAccountAndPermAssigments(in []*icsdk.GroupWithAssignment, accountMap icsdk.AccountMap, permSetMap icsdk.PermissionSetMap) []*GroupWithAccountAndPermAssignment {
	out := make([]*GroupWithAccountAndPermAssignment, 0, len(in))
	for _, g := range in {
		out = append(out, &GroupWithAccountAndPermAssignment{
			Group: &Group{
				Name: g.DisplayName,
				ID:   g.ID,
			},
			Assignments: AccountAndPermAssignments(g.Assignments, accountMap, permSetMap),
		})
	}
	return out
}

// GroupMember represents Identiy Center group members.
type GroupMember struct {
	// MemberID is an ID of a member assigned to a group.
	MemberID string `json:"memberID"`
}

// GroupMembers transforms icsdk GroupMember to awsicui GroupMember.
func GroupMembers(in []*icsdk.GroupMember) []*GroupMember {
	out := make([]*GroupMember, 0, len(in))
	for _, m := range in {
		out = append(out, &GroupMember{
			MemberID: m.MemberID,
		})
	}
	return out
}

// GroupWithMembers represents identity Center groups with
// enslited members (i.e. users).
type GroupWithMembers struct {
	*Group
	// Members is list of group members.
	Members []*GroupMember
}

// GroupsWithMembers transforms icsdk GroupWithMembers to awsicui GroupWithMembers.
func GroupsWithMembers(in []icsdk.GroupWithMembers) []GroupWithMembers {
	out := make([]GroupWithMembers, 0, len(in))
	for _, g := range in {
		out = append(out, GroupWithMembers{
			Group: &Group{
				Name: g.DisplayName,
				ID:   g.ID,
			},
			Members: GroupMembers(g.Members),
		})
	}
	return out
}
