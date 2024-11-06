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
)

func TestAssignmentProvisioner_Provision_CreateAndDeleteAssignments(t *testing.T) {
	const (
		externalID = "test-external-id"
	)
	ctx := context.Background()
	sdkMockClient := icsdk.NewClientMock()

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

	principal := &pb.PrincipalAssignment{
		Spec: &pb.PrincipalAssignmentSpec{
			ExternalId:    externalID,
			PrincipalType: pb.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST,
		},
		Status: &pb.PrincipalAssignmentStatus{
			Assignments: []*pb.AccountAssignmentRef{
				{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
				{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::0987654321:permissionSet/Admin"},
			},
			ProvisioningState: pb.ProvisioningState_PROVISIONING_STATE_STALE,
		},
	}

	updatedPrincipal, err := provisioner.Provision(ctx, principal)
	require.NoError(t, err)
	require.Equal(t, pb.ProvisioningState_PROVISIONING_STATE_PROVISIONED, updatedPrincipal.GetStatus().GetProvisioningState())

	got, err := sdkMockClient.ListAssignments(ctx, externalID, ssoadmintypes.PrincipalTypeGroup)
	require.NoError(t, err)

	want := []*icsdk.Assigment{
		{
			AccountID:        "1111111111",
			PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly",
		},
		{
			AccountID:        "1111111111",
			PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin",
		},
	}

	require.Empty(t, cmp.Diff(want, got,
		cmpopts.SortSlices(func(a, b *icsdk.Assigment) bool {
			if a.AccountID != b.AccountID {
				return a.AccountID < b.AccountID
			}
			return a.PermissionSetARN < b.PermissionSetARN
		}),
	))
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
