package provisioning

import (
	"context"
	"testing"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/stretchr/testify/require"

	headerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAssignmentProvisioner_Provision_CreateAndDeleteAssignments(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testCases := []struct {
		name                  string
		principalType         ssoadmintypes.PrincipalType
		calculatedAssignments []*pb.AccountAssignmentRef
		initialAssignments    []*icsdk.Assignment
		expectedAssignments   []*icsdk.Assignment
	}{
		{
			name:          "user-create",
			principalType: ssoadmintypes.PrincipalTypeUser,
			calculatedAssignments: []*pb.AccountAssignmentRef{
				{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
				{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::0987654321:permissionSet/Admin"},
			},
			expectedAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeUser},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin", PrincipalType: ssoadmintypes.PrincipalTypeUser},
			},
		},
		{
			name:          "user-delete",
			principalType: ssoadmintypes.PrincipalTypeUser,
			calculatedAssignments: []*pb.AccountAssignmentRef{
				{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
			},
			initialAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeUser},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin", PrincipalType: ssoadmintypes.PrincipalTypeUser},
			},
			expectedAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeUser},
			},
		},
		{
			name:          "group-create",
			principalType: ssoadmintypes.PrincipalTypeGroup,
			calculatedAssignments: []*pb.AccountAssignmentRef{
				{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
				{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::0987654321:permissionSet/Admin"},
			},
			expectedAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
			},
		},
		{
			name:          "group-delete",
			principalType: ssoadmintypes.PrincipalTypeGroup,
			calculatedAssignments: []*pb.AccountAssignmentRef{
				{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
			},
			initialAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
			},
			expectedAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
			},
		},
		{
			name:          "delete-all",
			principalType: ssoadmintypes.PrincipalTypeGroup,
			initialAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {

			// GIVEN a mock AWS client...
			sdkMockClient := icsdk.NewClientMock(nil /* custom mock data */)

			// GIVEN a method for resetting principal account assignments in the mock
			// AWS system
			setMockAccountAssignments := func(pType ssoadmintypes.PrincipalType, id string, assignments []*icsdk.Assignment) {
				sdkMockClient.Mu.Lock()
				defer sdkMockClient.Mu.Unlock()
				dst := sdkMockClient.GroupAssignments
				if pType == ssoadmintypes.PrincipalTypeUser {
					dst = sdkMockClient.UserAssignments
				}
				dst[id] = assignments
			}

			assignmentService := &mockAssignmentService{
				UpdatePrincipalAssignmentFunc: func(ctx context.Context, assignment *pb.PrincipalAssignment) (*pb.PrincipalAssignment, error) {
					assignment.Status.ProvisioningState = pb.ProvisioningState_PROVISIONING_STATE_PROVISIONED
					return assignment, nil
				},
			}

			// GIVEN an Assignment Provisioner to test, configured with a known account
			provisioner, err := NewAssignmentProvisioner(ProvisionerConfig{
				Assignment: assignmentService,
				SDKClient:  sdkMockClient,
			})
			require.NoError(t, err)
			provisioner.SetKnownAccounts("1111111111")

			// GIVEN an existing principal with pre-calculated account assignments
			principalID := "test-" + test.name
			externalID := "ext-" + test.name
			principal := &pb.PrincipalAssignment{
				Kind:    types.KindIdentityCenterPrincipalAssignment,
				Version: types.V1,
				Metadata: &headerpb.Metadata{
					Name: principalID,
				},
				Spec: &pb.PrincipalAssignmentSpec{
					ExternalId:    externalID,
					PrincipalType: toPrincipalType(test.principalType),
				},
				Status: &pb.PrincipalAssignmentStatus{
					Assignments:       test.calculatedAssignments,
					ProvisioningState: pb.ProvisioningState_PROVISIONING_STATE_STALE,
				},
			}

			// GIVEN a mock remote account with a known set of existing account
			// assignments
			setMockAccountAssignments(test.principalType, externalID, test.initialAssignments)

			// WHEN I provision the principal's account assignments...
			updatedPrincipal, err := provisioner.Provision(ctx, principal)

			// EXPECT that the principal assignment record has been marked as
			// PROVISIONED
			require.NoError(t, err)
			require.Equal(t,
				pb.ProvisioningState_PROVISIONING_STATE_PROVISIONED,
				updatedPrincipal.GetStatus().GetProvisioningState())

			// EXPECT that the provisioner has created the correct assignments
			actualAssignments, err := sdkMockClient.ListAssignments(ctx, externalID, test.principalType)
			require.NoError(t, err)
			assertAssignments(t, test.expectedAssignments, actualAssignments)
		})
	}
}

func toPrincipalType(t ssoadmintypes.PrincipalType) pb.PrincipalType {
	switch t {
	case ssoadmintypes.PrincipalTypeGroup:
		return pb.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST

	case ssoadmintypes.PrincipalTypeUser:
		return pb.PrincipalType_PRINCIPAL_TYPE_USER

	default:
		return pb.PrincipalType_PRINCIPAL_TYPE_UNSPECIFIED
	}
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

func TestFetchAWSAssignments_UserType(t *testing.T) {
	mockClient := &icsdk.ClientMock{
		MockedAWSStateType: icsdk.NewMockedAWSState(),
	}

	assignmentService := &mockAssignmentService{
		UpdatePrincipalAssignmentFunc: func(ctx context.Context, assignment *pb.PrincipalAssignment) (*pb.PrincipalAssignment, error) {
			assignment.Status.ProvisioningState = pb.ProvisioningState_PROVISIONING_STATE_PROVISIONED
			return assignment, nil
		},
	}

	provisioner, err := NewAssignmentProvisioner(ProvisionerConfig{
		Assignment: assignmentService,
		SDKClient:  mockClient,
	})
	require.NoError(t, err)

	var assignees = []*pb.AccountAssignmentRef{
		{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"},
	}

	principal := &pb.PrincipalAssignment{
		Spec: &pb.PrincipalAssignmentSpec{
			ExternalId:    "user2",
			PrincipalType: pb.PrincipalType_PRINCIPAL_TYPE_USER,
		},
		Status: &pb.PrincipalAssignmentStatus{
			Assignments:       assignees,
			ProvisioningState: pb.ProvisioningState_PROVISIONING_STATE_STALE,
		},
	}

	_, err = provisioner.Provision(context.Background(), principal)
	require.NoError(t, err)
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
	require.ElementsMatch(t, want, got)
}
