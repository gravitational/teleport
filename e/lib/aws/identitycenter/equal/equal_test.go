package equal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
)

func TestPermissionSetEqual(t *testing.T) {
	a := &identitycenterv1.PermissionSet{
		Metadata: &headerv1.Metadata{
			Name:        "PermissionSetA",
			Namespace:   "namespaceA",
			Description: "Test description A",
			Revision:    "1",
		},
		Spec: &identitycenterv1.PermissionSetSpec{
			Arn:         "arn:aws:iam::123456789012:role/ExampleRole",
			Name:        "ExampleRole",
			Description: "A test permission set",
		},
	}
	b := &identitycenterv1.PermissionSet{
		Metadata: &headerv1.Metadata{
			Name:        "PermissionSetA",
			Namespace:   "namespaceA",
			Description: "Test description A",
			Revision:    "2",
		},
		Spec: &identitycenterv1.PermissionSetSpec{
			Arn:         "arn:aws:iam::123456789012:role/ExampleRole",
			Name:        "ExampleRole",
			Description: "A test permission set",
		},
	}
	assert.True(t, PermissionSetEqual(a, b))
	assert.False(t, PermissionSetEqual(a, nil))
	assert.False(t, PermissionSetEqual(nil, b))
	assert.True(t, PermissionSetEqual(nil, nil))
}

func TestCompareStringSlices(t *testing.T) {
	a := []string{"appA", "appB", "appC"}
	b := []string{"appB", "appA", "appC"}

	assert.True(t, compareStringSlices(a, b))
	assert.False(t, compareStringSlices(a, nil))
	assert.False(t, compareStringSlices(nil, b))
	assert.True(t, compareStringSlices(nil, nil))
	assert.False(t, compareStringSlices(a, append(b, "appA")))
	assert.False(t, compareStringSlices(append(a, "appA"), b))
}

func TestPrincipalAssignmentEqual(t *testing.T) {
	a := &identitycenterv1.PrincipalAssignment{
		Metadata: &headerv1.Metadata{
			Name: "AssignmentA",
		},
		Spec: &identitycenterv1.PrincipalAssignmentSpec{
			PrincipalId:      "12345",
			PrincipalType:    identitycenterv1.PrincipalType_PRINCIPAL_TYPE_USER,
			ExternalId:       "ext123",
			ExternalIdSource: "sourceA",
		},
		Status: &identitycenterv1.PrincipalAssignmentStatus{
			ProvisioningState: identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			Applications:      []string{"appA", "appB"},
		},
	}
	b := &identitycenterv1.PrincipalAssignment{
		Metadata: &headerv1.Metadata{
			Name: "AssignmentA",
		},
		Spec: &identitycenterv1.PrincipalAssignmentSpec{
			PrincipalId:      "12345",
			PrincipalType:    identitycenterv1.PrincipalType_PRINCIPAL_TYPE_USER,
			ExternalId:       "ext123",
			ExternalIdSource: "sourceA",
		},
		Status: &identitycenterv1.PrincipalAssignmentStatus{
			ProvisioningState: identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE,
			Applications:      []string{"appB", "appA"},
		},
	}

	assert.True(t, PrincipalAssignmentEqual(nil, nil))
	assert.True(t, PrincipalAssignmentEqual(a, b))
	assert.False(t, PrincipalAssignmentEqual(a, nil))
	assert.False(t, PrincipalAssignmentEqual(nil, b))
}

func TestAccountAssignmentEqual(t *testing.T) {
	a := &identitycenterv1.AccountAssignment{
		Metadata: &headerv1.Metadata{
			Name: "AssignmentA",
		},
		Spec: &identitycenterv1.AccountAssignmentSpec{
			Display:     "DisplayA",
			AccountName: "AccountA",
			AccountId:   "123456789012",
			PermissionSet: &identitycenterv1.PermissionSetInfo{
				Arn:  "arn:aws:iam::123456789012:role/ExampleRole",
				Name: "ExampleRole",
				Role: "roleA",
			},
		},
	}
	b := &identitycenterv1.AccountAssignment{
		Metadata: &headerv1.Metadata{
			Name: "AssignmentA",
		},
		Spec: &identitycenterv1.AccountAssignmentSpec{
			Display:     "DisplayA",
			AccountName: "AccountA",
			AccountId:   "123456789012",
			PermissionSet: &identitycenterv1.PermissionSetInfo{
				Arn:  "arn:aws:iam::123456789012:role/ExampleRole",
				Name: "ExampleRole",
				Role: "roleA",
			},
		},
	}

	assert.True(t, AccountAssignmentEqual(nil, nil))
	assert.True(t, AccountAssignmentEqual(a, b))
	assert.False(t, AccountAssignmentEqual(a, nil))
	assert.False(t, AccountAssignmentEqual(nil, b))
}

func TestAccountEqual(t *testing.T) {
	a := &identitycenterv1.Account{
		Metadata: &headerv1.Metadata{
			Name:        "AccountA",
			Namespace:   "namespaceA",
			Description: "Test account",
		},
		Spec: &identitycenterv1.AccountSpec{
			Id:                  "123456789012",
			Arn:                 "arn:aws:iam::123456789012:account/ExampleAccount",
			Name:                "ExampleAccount",
			Description:         "A test account",
			StartUrl:            "https://example.com/start",
			IsOrganizationOwner: true,
			PermissionSetInfo: []*identitycenterv1.PermissionSetInfo{
				{Arn: "arn:aws:iam::123456789012:role/ExampleRole1", Name: "ExampleRole1"},
				{Arn: "arn:aws:iam::123456789012:role/ExampleRole2", Name: "ExampleRole2"},
			},
		},
	}
	b := &identitycenterv1.Account{
		Metadata: &headerv1.Metadata{
			Name:        "AccountA",
			Namespace:   "namespaceA",
			Description: "Test account",
		},
		Spec: &identitycenterv1.AccountSpec{
			Id:                  "123456789012",
			Arn:                 "arn:aws:iam::123456789012:account/ExampleAccount",
			Name:                "ExampleAccount",
			Description:         "A test account",
			StartUrl:            "https://example.com/start",
			IsOrganizationOwner: true,
			PermissionSetInfo: []*identitycenterv1.PermissionSetInfo{
				{Arn: "arn:aws:iam::123456789012:role/ExampleRole2", Name: "ExampleRole2"},
				{Arn: "arn:aws:iam::123456789012:role/ExampleRole1", Name: "ExampleRole1"},
			},
		},
	}

	assert.True(t, AccountEqual(a, b))

	assert.True(t, AccountEqual(nil, nil))
	assert.True(t, AccountEqual(a, b))
	assert.False(t, AccountEqual(a, nil))
	assert.False(t, AccountEqual(nil, b))

	b.Spec.PermissionSetInfo = []*identitycenterv1.PermissionSetInfo{
		{Arn: "arn:aws:iam::123456789012:role/ExampleRole2", Name: "ExampleRole2"},
	}
	assert.False(t, AccountEqual(a, b))
}
