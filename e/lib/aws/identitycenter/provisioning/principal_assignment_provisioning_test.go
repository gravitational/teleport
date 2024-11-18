package provisioning

import (
	"context"
	"testing"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAssignmentProvisioner_Provision_CreateAndDeleteAssignments(t *testing.T) {
	const (
		externalID = "test-external-id"
	)
	ctx := context.Background()
	sdkMockClient := icsdk.NewClientMock(nil /* custom mock data */)

	assignmentService := &mockAssignmentService{
		UpdatePrincipalAssignmentFunc: func(ctx context.Context, assignment *pb.PrincipalAssignment) (*pb.PrincipalAssignment, error) {
			assignment.Status.ProvisioningState = pb.ProvisioningState_PROVISIONING_STATE_PROVISIONED
			return assignment, nil
		},
	}

	provisioner, err := NewAssignmentProvisioner(ProvisionerConfig{
		Assignment: assignmentService,
		SDKClient:  sdkMockClient,
	})
	require.NoError(t, err)

	var assignees = []*pb.AccountAssignmentRef{
		{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
		{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::0987654321:permissionSet/Admin"},
	}

	principal := &pb.PrincipalAssignment{
		Spec: &pb.PrincipalAssignmentSpec{
			ExternalId:    externalID,
			PrincipalType: pb.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST,
		},
		Status: &pb.PrincipalAssignmentStatus{
			Assignments:       assignees,
			ProvisioningState: pb.ProvisioningState_PROVISIONING_STATE_STALE,
		},
	}

	updatedPrincipal, err := provisioner.Provision(ctx, principal)
	require.NoError(t, err)
	require.Equal(t, pb.ProvisioningState_PROVISIONING_STATE_PROVISIONED, updatedPrincipal.GetStatus().GetProvisioningState())

	got, err := sdkMockClient.ListAssignments(ctx, externalID, ssoadmintypes.PrincipalTypeGroup)
	require.NoError(t, err)

	want := []*icsdk.Assignment{
		{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
		{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin"},
	}

	assertAssignments(t, want, got)

	_, err = sdkMockClient.CreateAccountAssignment(ctx, &icsdk.CreateAccountAssignmentRequest{
		PrincipalID:      externalID,
		PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/Custom",
		AccountID:        "22222222222",
		PrincipalType:    ssoadmintypes.PrincipalTypeGroup,
	})
	require.NoError(t, err)

	principal.Status.ProvisioningState = pb.ProvisioningState_PROVISIONING_STATE_STALE
	principal.Status.Assignments = []*pb.AccountAssignmentRef{
		{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
	}

	_, err = provisioner.Provision(ctx, principal)
	require.NoError(t, err)

	got, err = sdkMockClient.ListAssignments(ctx, externalID, ssoadmintypes.PrincipalTypeGroup)
	require.NoError(t, err)
	want = []*icsdk.Assignment{
		{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
	}
	assertAssignments(t, want, got)
}

func TestAssignmentDiffCalculator(t *testing.T) {
	tests := []struct {
		name        string
		localState  []icsdk.Assignment
		remoteState []icsdk.Assignment

		wantToDelete []icsdk.Assignment
		wantToCreate []icsdk.Assignment
	}{
		{
			name: "no diff",
			localState: []icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
			},
			remoteState: []icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
			},
			wantToDelete: nil,
			wantToCreate: nil,
		},
		{
			name:         "no diff nil objects",
			localState:   nil,
			remoteState:  nil,
			wantToDelete: nil,
			wantToCreate: nil,
		},
		{
			name: "delete and update",
			localState: []icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/Admin"},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/Custom"},
			},
			remoteState: []icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
				{AccountID: "3333333333", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/Custom"},
			},
			wantToDelete: []icsdk.Assignment{
				{AccountID: "3333333333", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/Custom"},
			},
			wantToCreate: []icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/Admin"},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/Custom"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diff := &assignmentDiffCalculator{
				teleportAssignments: utils.NewSet(tc.localState...),
				awsAssignments:      utils.NewSet(tc.remoteState...),
			}
			gotToDelete := diff.assignmentsToDelete()
			require.ElementsMatch(t, tc.wantToDelete, gotToDelete.Elements())

			gotToCreate := diff.assignmentsToCreate()
			require.ElementsMatch(t, tc.wantToCreate, gotToCreate.Elements())
		})
	}
}

type mockAssignmentService struct {
	UpdatePrincipalAssignmentFunc func(ctx context.Context, assignment *pb.PrincipalAssignment) (*pb.PrincipalAssignment, error)
	services.IdentityCenterPrincipalAssignments
}

func (m *mockAssignmentService) UpdatePrincipalAssignment(ctx context.Context, assignment *pb.PrincipalAssignment) (*pb.PrincipalAssignment, error) {
	if m.UpdatePrincipalAssignmentFunc != nil {
		return m.UpdatePrincipalAssignmentFunc(ctx, assignment)
	}
	return assignment, nil
}

func assertAssignments(t *testing.T, want, got []*icsdk.Assignment) {
	t.Helper()
	require.Empty(t, cmp.Diff(want, got,
		cmpopts.SortSlices(func(a, b *icsdk.Assignment) bool {
			if a.AccountID != b.AccountID {
				return a.AccountID < b.AccountID
			}
			return a.PermissionSetARN < b.PermissionSetARN
		}),
	))
}
