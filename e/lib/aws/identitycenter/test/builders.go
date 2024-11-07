package test

import (
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
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
	return &identitycenterv1.PermissionSet{
		Kind:    types.KindIdentityCenterPermissionSet,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: ps.ID,
			Labels: map[string]string{
				common.OriginLabel: common.OriginAWSIdentityCenter,
			},
		},
		Spec: &identitycenterv1.PermissionSetSpec{
			Arn:         ps.ARN,
			Name:        ps.Name,
			Description: ps.Description,
		},
	}
}

// Account is a builder for IdentityCenterAccount instances used in tests
type Account struct {
	ID             services.IdentityCenterAccountID
	Name           string
	ARN            string
	IsOwner        bool
	PermissionSets iter.Seq[*identitycenterv1.PermissionSet]
}

func (a Account) Build() services.IdentityCenterAccount {
	account := services.IdentityCenterAccount{
		Account: &identitycenterv1.Account{
			Kind:    types.KindIdentityCenterAccount,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: string(a.ID),
				Labels: map[string]string{
					common.OriginLabel: common.OriginAWSIdentityCenter,
				},
			},
			Spec: &identitycenterv1.AccountSpec{
				Id:                  string(a.ID),
				Name:                a.Name,
				Arn:                 a.ARN,
				IsOrganizationOwner: a.IsOwner,
			},
			Status: &identitycenterv1.AccountStatus{},
		},
	}

	sortedPSs := slices.Collect(a.PermissionSets)
	slices.SortFunc(sortedPSs, func(a, b *identitycenterv1.PermissionSet) int {
		return strings.Compare(a.GetMetadata().GetName(), b.GetMetadata().GetName())
	})

	for _, ps := range sortedPSs {
		account.GetSpec().PermissionSetInfo = append(
			account.GetSpec().PermissionSetInfo,
			&identitycenterv1.PermissionSetInfo{
				Name: ps.GetSpec().GetName(),
				Arn:  ps.GetSpec().GetArn(),
			})
	}

	return account
}

type AccountAssignmentRole struct {
	Name             string
	AccountID        services.IdentityCenterAccountID
	PermissionSetARN string
}

func (a AccountAssignmentRole) Build(t *testing.T) *types.RoleV6 {
	t.Helper()

	role := &types.RoleV6{
		Kind:    types.KindRole,
		SubKind: "aws_identity_center",
		Version: types.V7,
		Metadata: types.Metadata{
			Name: a.Name,
			Labels: map[string]string{
				common.OriginLabel: common.OriginAWSIdentityCenter,
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

func (a AccountAssignment) Build() services.IdentityCenterAccountAssignment {
	return services.IdentityCenterAccountAssignment{
		AccountAssignment: &identitycenterv1.AccountAssignment{
			Kind:    types.KindIdentityCenterAccountAssignment,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: a.ID,
				Labels: map[string]string{
					types.OriginLabel: common.OriginAWSIdentityCenter,
				},
			},
			Spec: &identitycenterv1.AccountAssignmentSpec{
				Display: a.DisplayName,
				PermissionSet: &identitycenterv1.PermissionSetInfo{
					Arn:  a.PermissionSetARN,
					Name: a.PermissionSetName,
				},
				AccountName: a.AccountName,
				AccountId:   string(a.AccountID),
			},
		},
	}
}
