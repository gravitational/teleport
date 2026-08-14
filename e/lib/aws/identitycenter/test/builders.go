package test

import (
	"fmt"
	"iter"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/lib/services"
)

// PermissionSet is a builder for PermissionSet instances used in tests
type PermissionSet struct {
	ID          string
	Name        string
	Description string
	ARN         string
}

func (ps PermissionSet) Build() *identitycenterv1.PermissionSet {
	return identitycenterv1.PermissionSet_builder{
		Kind:    types.KindIdentityCenterPermissionSet,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: ps.ID,
			Labels: map[string]string{
				common.OriginLabel: common.OriginAWSIdentityCenter,
			},
		}.Build(),
		Spec: identitycenterv1.PermissionSetSpec_builder{
			Arn:         ps.ARN,
			Name:        ps.Name,
			Description: ps.Description,
		}.Build(),
	}.Build()
}

// Account is a builder for IdentityCenterAccount instances used in tests
type Account struct {
	ID             services.IdentityCenterAccountID
	Name           string
	ARN            string
	SSORegion      string
	IsOwner        bool
	PermissionSets iter.Seq[*identitycenterv1.PermissionSet]
	StartURL       string
}

func (a Account) Build() *identitycenterv1.Account {
	if a.StartURL == "" {
		a.StartURL = fmt.Sprintf("https://store1.awsapps.com/start/#/console?account_id=%s", a.ID)
	}

	if a.PermissionSets == nil {
		a.PermissionSets = slices.Values(([]*identitycenterv1.PermissionSet)(nil))
	}

	account := identitycenterv1.Account_builder{
		Kind:    types.KindIdentityCenterAccount,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:        string(a.ID),
			Description: a.Name,
			Labels: map[string]string{
				common.OriginLabel:        common.OriginAWSIdentityCenter,
				types.AWSAccountIDLabel:   string(a.ID),
				types.AWSAccountNameLabel: a.Name,
				types.AWSSSORegionLabel:   a.SSORegion,
			},
		}.Build(),
		Spec: identitycenterv1.AccountSpec_builder{
			Id:                  string(a.ID),
			Name:                a.Name,
			Arn:                 a.ARN,
			IsOrganizationOwner: a.IsOwner,
			StartUrl:            a.StartURL,
		}.Build(),
		Status: &identitycenterv1.AccountStatus{},
	}.Build()

	sortedPSs := slices.Collect(a.PermissionSets)
	slices.SortFunc(sortedPSs, func(a, b *identitycenterv1.PermissionSet) int {
		return strings.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
	})

	for _, ps := range sortedPSs {
		account.GetSpec().SetPermissionSetInfo(append(
			account.GetSpec().GetPermissionSetInfo(),
			identitycenterv1.PermissionSetInfo_builder{
				Name: ps.GetSpec().GetName(),
				Arn:  ps.GetSpec().GetArn(),
			}.Build()))
	}

	return account
}

type AccountAssignmentRole struct {
	Name             string
	AccountID        services.IdentityCenterAccountID
	PermissionSetARN string
	customizations   []func(*types.RoleV6)
}

func (a AccountAssignmentRole) Customize(f func(*types.RoleV6)) AccountAssignmentRole {
	a.customizations = append(a.customizations, f)
	return a
}

func (a AccountAssignmentRole) Build(t *testing.T) *types.RoleV6 {
	t.Helper()

	role := &types.RoleV6{
		Kind:    types.KindRole,
		SubKind: types.KindIdentityCenter,
		Version: types.V7,
		Metadata: types.Metadata{
			Name: a.Name,
			Labels: map[string]string{
				"teleport.internal/account_id": string(a.AccountID),
				"teleport.internal/created_by": types.KindIdentityCenter,
			},
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				AccountAssignments: []types.IdentityCenterAccountAssignment{
					{
						Account:       string(a.AccountID),
						PermissionSet: a.PermissionSetARN,
					},
				},
			},
		},
	}
	require.NoError(t, role.CheckAndSetDefaults())

	for _, f := range a.customizations {
		f(role)
	}

	return role
}

type AccountAssignment struct {
	ID                string
	DisplayName       string
	AccountName       string
	AccountID         services.IdentityCenterAccountID
	PermissionSetName string
	PermissionSetARN  string
}

func (a AccountAssignment) Build() *identitycenterv1.AccountAssignment {
	return identitycenterv1.AccountAssignment_builder{
		Kind:    types.KindIdentityCenterAccountAssignment,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: a.ID,
			Labels: map[string]string{
				types.OriginLabel:         common.OriginAWSIdentityCenter,
				types.AWSAccountIDLabel:   string(a.AccountID),
				types.AWSAccountNameLabel: a.AccountName,
			},
		}.Build(),
		Spec: identitycenterv1.AccountAssignmentSpec_builder{
			Display: a.DisplayName,
			PermissionSet: identitycenterv1.PermissionSetInfo_builder{
				Arn:  a.PermissionSetARN,
				Name: a.PermissionSetName,
			}.Build(),
			AccountName: a.AccountName,
			AccountId:   string(a.AccountID),
		}.Build(),
	}.Build()
}

type AccessList struct {
	Name          string
	Title         string
	Owners        []types.User
	GrantsMembers []types.Role
	GrantsOwners  []types.Role
	Labels        map[string]string
}

func (a AccessList) Build(t *testing.T) *accesslist.AccessList {
	var owners []accesslist.Owner
	for _, owner := range a.Owners {
		owners = append(owners, accesslist.Owner{Name: owner.GetName()})
	}

	ownerGrants := make([]string, 0, len(a.GrantsOwners))
	for _, g := range a.GrantsOwners {
		ownerGrants = append(ownerGrants, g.GetName())
	}

	membershipGrants := make([]string, 0, len(a.GrantsMembers))
	for _, g := range a.GrantsMembers {
		membershipGrants = append(membershipGrants, g.GetName())
	}

	acl, err := accesslist.NewAccessList(
		header.Metadata{
			Name:   a.Name,
			Labels: a.Labels,
		},
		accesslist.Spec{
			Title:  a.Title,
			Owners: owners,
			OwnerGrants: accesslist.Grants{
				Roles:  ownerGrants,
				Traits: trait.Traits{},
			},
			Grants: accesslist.Grants{
				Roles:  membershipGrants,
				Traits: trait.Traits{},
			},
		})
	require.NoError(t, err)
	return acl
}

type AccessListMember struct {
	Member     types.User
	AccessList *accesslist.AccessList
	Timestamp  time.Time
	AddedBy    types.User
}

func (a AccessListMember) Build(t *testing.T) *accesslist.AccessListMember {
	if a.Timestamp.IsZero() {
		a.Timestamp = time.Now()
	}

	aclMember, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: a.Member.GetName(),
		},
		accesslist.AccessListMemberSpec{
			AccessList: a.AccessList.GetName(),
			Name:       a.Member.GetName(),
			Joined:     a.Timestamp,
			AddedBy:    a.AddedBy.GetName(),
		})
	require.NoError(t, err)

	return aclMember
}

type AccessRequest struct {
	Name            string
	User            types.User
	Roles           []types.Role
	ResourceIDs     []types.ResourceID
	State           types.RequestState
	AssumeStartTime time.Time
	Expiry          time.Time
}

func (a AccessRequest) Build(t *testing.T) types.AccessRequest {
	t.Helper()

	if a.Name == "" {
		a.Name = uuid.NewString()
	}

	if a.Expiry.IsZero() {
		a.Expiry = time.Now().Add(time.Hour)
	}

	require.NotNil(t, a.User, "target User must be supplied")

	roleNames := make([]string, len(a.Roles))
	for i, r := range a.Roles {
		roleNames[i] = r.GetName()
	}

	accessRequest, err := types.NewAccessRequestWithResources(
		a.Name,
		a.User.GetName(),
		roleNames,
		types.ResourceIDsToResourceAccessIDs(a.ResourceIDs))
	require.NoError(t, err, "invalid access request")

	if !a.AssumeStartTime.IsZero() {
		accessRequest.SetAssumeStartTime(a.AssumeStartTime)
	}
	accessRequest.SetAccessExpiry(a.Expiry)
	accessRequest.SetState(a.State)

	return accessRequest
}
