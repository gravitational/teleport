package provisioning

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/stretchr/testify/require"

	headerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
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
		expectedCreateCount   int
		expectedDeleteCount   int
	}{
		{
			name:          "user-create",
			principalType: ssoadmintypes.PrincipalTypeUser,
			calculatedAssignments: []*pb.AccountAssignmentRef{
				pb.AccountAssignmentRef_builder{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"}.Build(),
				pb.AccountAssignmentRef_builder{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::0987654321:permissionSet/Admin"}.Build(),
			},
			expectedAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeUser},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin", PrincipalType: ssoadmintypes.PrincipalTypeUser},
			},
			expectedDeleteCount: 0,
			expectedCreateCount: 2,
		},
		{
			name:          "user-delete",
			principalType: ssoadmintypes.PrincipalTypeUser,
			calculatedAssignments: []*pb.AccountAssignmentRef{
				pb.AccountAssignmentRef_builder{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::0987654323:permissionSet/NetworkAdmin"}.Build(),
			},
			initialAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeUser},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin", PrincipalType: ssoadmintypes.PrincipalTypeUser},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654323:permissionSet/NetworkAdmin", PrincipalType: ssoadmintypes.PrincipalTypeUser},
			},
			expectedAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654323:permissionSet/NetworkAdmin", PrincipalType: ssoadmintypes.PrincipalTypeUser},
			},
			expectedDeleteCount: 2,
			expectedCreateCount: 0,
		},
		{
			name:          "group-create",
			principalType: ssoadmintypes.PrincipalTypeGroup,
			calculatedAssignments: []*pb.AccountAssignmentRef{
				pb.AccountAssignmentRef_builder{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"}.Build(),
				pb.AccountAssignmentRef_builder{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::0987654321:permissionSet/Admin"}.Build(),
			},
			expectedAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
			},
			expectedDeleteCount: 0,
			expectedCreateCount: 2,
		},
		{
			name:          "group-delete",
			principalType: ssoadmintypes.PrincipalTypeGroup,
			calculatedAssignments: []*pb.AccountAssignmentRef{
				pb.AccountAssignmentRef_builder{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::0987654323:permissionSet/NetworkAdmin"}.Build(),
			},
			initialAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
			},
			expectedAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654323:permissionSet/NetworkAdmin", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
			},
			expectedDeleteCount: 2,
			expectedCreateCount: 1,
		},
		{
			name:          "delete-all",
			principalType: ssoadmintypes.PrincipalTypeGroup,
			initialAssignments: []*icsdk.Assignment{
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::1234567890:permissionSet/ReadOnly", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654321:permissionSet/Admin", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:iam::0987654323:permissionSet/NetworkAdmin", PrincipalType: ssoadmintypes.PrincipalTypeGroup},
			},
			expectedDeleteCount: 3,
			expectedCreateCount: 0,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {

			// GIVEN a mock AWS client...
			sdkMockClient := icsdk.NewClientMock(nil /* custom mock data */)
			createCount := 0
			deleteCount := 0
			sdkMockClient.MonkeyPatch.CreateAccountAssignmentCounter = func() {
				createCount++
			}
			sdkMockClient.MonkeyPatch.DeleteAccountAssignmentCounter = func() {
				deleteCount++
			}
			// GIVEN a method for resetting principal account assignments in the mock
			// AWS system
			setMockAccountAssignments := func(pType ssoadmintypes.PrincipalType, id string, assignments []*icsdk.Assignment) {
				sdkMockClient.Mu.Lock()
				defer sdkMockClient.Mu.Unlock()

				var dst map[string][]*icsdk.Assignment

				switch pType {
				case ssoadmintypes.PrincipalTypeUser:
					dst = sdkMockClient.UserAssignments
					sdkMockClient.Users = append(sdkMockClient.Users,
						&icsdk.MockUser{
							User: identitystoretypes.User{
								UserId:   aws.String(id),
								UserName: aws.String("test-user-" + test.name),
							},
						})
				case ssoadmintypes.PrincipalTypeGroup:
					dst = sdkMockClient.GroupAssignments
					sdkMockClient.Groups = append(sdkMockClient.Groups,
						&icsdk.Group{
							DisplayName: "test-group-" + test.name,
							ID:          id,
						})

				default:
					require.FailNowf(t, "Test configuration error",
						"Unexpected Principal type: %v", pType)
				}

				dst[id] = assignments
			}

			assignmentService := &mockAssignmentService{
				UpdatePrincipalAssignmentFunc: func(ctx context.Context, assignment *pb.PrincipalAssignment) (*pb.PrincipalAssignment, error) {
					assignment.GetStatus().SetProvisioningState(pb.ProvisioningState_PROVISIONING_STATE_PROVISIONED)
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
			principal := pb.PrincipalAssignment_builder{
				Kind:    types.KindIdentityCenterPrincipalAssignment,
				Version: types.V1,
				Metadata: headerpb.Metadata_builder{
					Name: principalID,
				}.Build(),
				Spec: pb.PrincipalAssignmentSpec_builder{
					ExternalId:    externalID,
					PrincipalType: toPrincipalType(test.principalType),
				}.Build(),
				Status: pb.PrincipalAssignmentStatus_builder{
					Assignments:       test.calculatedAssignments,
					ProvisioningState: pb.ProvisioningState_PROVISIONING_STATE_STALE,
				}.Build(),
			}.Build()

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
			require.Equal(t, test.expectedCreateCount, createCount, "create account assignment attempt")
			require.Equal(t, test.expectedDeleteCount, deleteCount, "delete account assignment attempt")
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
				teleportAssignments: set.New(tc.localState...),
				awsAssignments:      set.New(tc.remoteState...),
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
	mockClient.Users = append(mockClient.Users, &icsdk.MockUser{
		User: identitystoretypes.User{
			UserId:   aws.String("user2"),
			UserName: aws.String("test-user-" + t.Name()),
		},
	})

	assignmentService := &mockAssignmentService{
		UpdatePrincipalAssignmentFunc: func(ctx context.Context, assignment *pb.PrincipalAssignment) (*pb.PrincipalAssignment, error) {
			assignment.GetStatus().SetProvisioningState(pb.ProvisioningState_PROVISIONING_STATE_PROVISIONED)
			return assignment, nil
		},
	}

	provisioner, err := NewAssignmentProvisioner(ProvisionerConfig{
		Assignment: assignmentService,
		SDKClient:  mockClient,
	})
	require.NoError(t, err)

	var assignees = []*pb.AccountAssignmentRef{
		pb.AccountAssignmentRef_builder{AccountId: "1111111111", PermissionSetArn: "arn:aws:iam::1234567890:permissionSet/ReadOnly"}.Build(),
	}

	principal := pb.PrincipalAssignment_builder{
		Spec: pb.PrincipalAssignmentSpec_builder{
			ExternalId:    "user2",
			PrincipalType: pb.PrincipalType_PRINCIPAL_TYPE_USER,
		}.Build(),
		Status: pb.PrincipalAssignmentStatus_builder{
			Assignments:       assignees,
			ProvisioningState: pb.ProvisioningState_PROVISIONING_STATE_STALE,
		}.Build(),
	}.Build()

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
