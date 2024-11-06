package provisioning

import (
	"context"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/lib/utils"
)

// NewAssignmentProvisioner creates a new AssignmentProvisioner.
func NewAssignmentProvisioner(cfg ProvisionerConfig) (*AssignmentProvisioner, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &AssignmentProvisioner{
		ProvisionerConfig: cfg,
	}, nil
}

// AssignmentProvisioner provisions the principal assignment in AWS Identity Center.
type AssignmentProvisioner struct {
	ProvisionerConfig
}

// Provision provisions the principal assignment in AWS Identity Center.
// It compares the assignments in the Teleport Backend with the assignments in the AWS Identity Center.
// If the assignment is present in the Teleport Backend but not in the AWS Identity Center, it creates the assignment.
// If the assignment is present in the AWS Identity Center but not in the Teleport Backend, it deletes the assignment.
// If the assignment is present in both the Teleport Backend and the AWS Identity Center, it does nothing.
// It updates the assignment on the Teleport Backend to reflect the provisioning state.
func (a *AssignmentProvisioner) Provision(ctx context.Context, principal *pb.PrincipalAssignment) (*pb.PrincipalAssignment, error) {
	if isAssignmentProvisioned(principal) {
		return principal, nil
	}

	principalType, err := toSSOAdminPrincipalType(principal)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	externalID := principal.GetSpec().GetExternalId()

	log := a.Log.With(
		"external_id", externalID,
		"principal_type", principalType,
		"principal_assignment", principal.GetMetadata().GetName(),
	)

	awsAssignments, err := a.SDKClient.ListAssignments(ctx, externalID, principalType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	teleportAssignments := convertToICSDKAssignments(principal.GetStatus().GetAssignments())

	diffCalc := assignmentDiffCalculator{
		teleportAssignments: utils.NewSet(teleportAssignments...),
		awsAssignments:      utils.NewSet(awsAssignments...),
	}
	for item := range diffCalc.assignmentsToCreate() {
		err := a.createAssignment(ctx, externalID, item.PermissionSetARN, item.AccountID, principalType)
		if err != nil {
			log.WarnContext(ctx, "Failed to create AWS IC assignment", "permission_set_arn", item.PermissionSetARN, "account_id", item.AccountID)
			return nil, trace.Wrap(err)
		}
	}
	for item := range diffCalc.assignmentsToDelete() {
		if err := a.deleteAssignment(ctx, externalID, item.PermissionSetARN, item.AccountID, principalType); err != nil {
			log.WarnContext(ctx, "Failed to delete AWS IC assignment", "permission_set_arn", item.PermissionSetARN, "account_id", item.AccountID)
			return nil, trace.Wrap(err)
		}
	}

	// Update the assignment on the Teleport backend to reflect the provisioning state.
	// If operation fails due to CAS revision mismatch, it means that the assignment
	// has been updated while we were processing on it. In this case just return
	// and wait for the next sync cycle, where the assignment be updated assignment
	// will be updated.
	updatedAssignment, err := a.Assignment.UpdatePrincipalAssignment(ctx, markAssignmentAsProvisioned(principal))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updatedAssignment, nil
}

// createAssignment creates an Identity Center account assignment for the
// given (principal, account, permission_set) triple, and waits for confirmation
// that the assignment was created.
func (a *AssignmentProvisioner) createAssignment(ctx context.Context, externalID, permissionSetARN, accountID string, principalType ssoadmintypes.PrincipalType) error {
	ctx, cancel := context.WithTimeout(ctx, a.ProvisioningTimeout)
	defer cancel()
	resp, err := a.SDKClient.CreateAccountAssignment(ctx, &icsdk.CreateAccountAssignmentRequest{
		PrincipalID:      externalID,
		PermissionSetARN: permissionSetARN,
		AccountID:        accountID,
		PrincipalType:    principalType,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.SDKClient.WaitForAccountAssignmentResult(ctx, resp.RequestID); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// createAssignment deletes an existing Identity Center account assignment for
// the given (principal, account, permission_set) triple, and waits for confirmation
// that the assignment was deleted.
func (a *AssignmentProvisioner) deleteAssignment(ctx context.Context, externalID, permissionSetARN, accountID string, principalType ssoadmintypes.PrincipalType) error {
	ctx, cancel := context.WithTimeout(ctx, a.ProvisioningTimeout)
	defer cancel()
	resp, err := a.SDKClient.DeleteAccountAssignment(ctx, &icsdk.DeleteAccountAssignmentRequest{
		PrincipalID:      externalID,
		PermissionSetARN: permissionSetARN,
		AccountID:        accountID,
		PrincipalTarget:  principalType,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.SDKClient.WaitForAccountAssignmentResult(ctx, resp.RequestID); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type assignmentDiffCalculator struct {
	teleportAssignments utils.Set[*icsdk.Assigment]
	awsAssignments      utils.Set[*icsdk.Assigment]
}

// assignmentsToCreate calculates  the set of Identity Center assignment that
// need to be created in order to sync the Teleport assignment set with AWS.
func (d *assignmentDiffCalculator) assignmentsToCreate() utils.Set[*icsdk.Assigment] {
	return d.teleportAssignments.Clone().Subtract(d.awsAssignments)
}

// assignmentsToDelete calculates the set of Identity Center assignments that
// need to be deleted in order to sync the Teleport assignment set with AWS.
func (d *assignmentDiffCalculator) assignmentsToDelete() utils.Set[*icsdk.Assigment] {
	return d.awsAssignments.Clone().Subtract(d.teleportAssignments)
}

func isAssignmentProvisioned(principal *pb.PrincipalAssignment) bool {
	return principal.GetStatus().GetProvisioningState() == pb.ProvisioningState_PROVISIONING_STATE_PROVISIONED
}

func toSSOAdminPrincipalType(principal *pb.PrincipalAssignment) (ssoadmintypes.PrincipalType, error) {
	switch principal.GetSpec().GetPrincipalType() {
	case pb.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST:
		return ssoadmintypes.PrincipalTypeGroup, nil
	case pb.PrincipalType_PRINCIPAL_TYPE_USER:
		return ssoadmintypes.PrincipalTypeUser, nil
	}
	return "", trace.BadParameter("unsupported principal type %q", principal.GetSpec().GetPrincipalType())
}

func convertToICSDKAssignments(in []*pb.AccountAssignmentRef) []*icsdk.Assigment {
	out := make([]*icsdk.Assigment, 0, len(in))
	for _, v := range in {
		out = append(out, &icsdk.Assigment{
			AccountID:        v.GetAccountId(),
			PermissionSetARN: v.GetPermissionSetArn(),
		})
	}
	return out
}

func markAssignmentAsProvisioned(principal *pb.PrincipalAssignment) *pb.PrincipalAssignment {
	cpy := proto.Clone(principal).(*pb.PrincipalAssignment)
	cpy.GetStatus().ProvisioningState = pb.ProvisioningState_PROVISIONING_STATE_PROVISIONED
	cpy.GetStatus().Error = ""
	return cpy
}
