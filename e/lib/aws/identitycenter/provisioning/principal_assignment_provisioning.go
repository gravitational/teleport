package provisioning

import (
	"context"
	"errors"
	"fmt"
	"sync"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/proto"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
)

// concurrentCreateAccountAssignmentLimit is the maximum number of concurrent
// Create- and DeleteAccountAssignment calls that can be made. AWS currently has
// a hard limit of 15 concurrent operations in flight, which cannot be changed.
// See: https://docs.aws.amazon.com/singlesignon/latest/userguide/limits.html#ssothrottlelimits
const concurrentCreateAccountAssignmentLimit = 15

// MissingPrincipalError is returned when a principal (user or group) is not found
// in AWS Identity Center, indicating it has been deleted externally.
type MissingPrincipalError struct {
	// Principal is the Assignment Record for the principal that is missing from
	// the downstream Identity Center instance
	Principal *pb.PrincipalAssignment
}

func (e *MissingPrincipalError) Error() string {
	spec := e.Principal.GetSpec()
	return fmt.Sprintf("%v principal %q with external ID %q is missing from AWS Identity Center",
		spec.GetPrincipalType(), spec.GetPrincipalId(), spec.GetExternalId())
}

// AsMissingPrincipalError checks if an error is a missingPrincipalError,
// indicating that a principal is not found in AWS Identity Center.
func AsMissingPrincipalError(err error) (*MissingPrincipalError, bool) {
	if err == nil {
		return nil, false
	}

	var mpe *MissingPrincipalError
	if errors.As(err, &mpe) {
		return mpe, true
	}
	return nil, false
}

// NewAssignmentProvisioner creates a new AssignmentProvisioner.
func NewAssignmentProvisioner(cfg ProvisionerConfig) (*AssignmentProvisioner, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &AssignmentProvisioner{
		ProvisionerConfig:         cfg,
		knownAccounts:             set.New[services.IdentityCenterAccountID](),
		createAssignmentSemaphore: semaphore.NewWeighted(concurrentCreateAccountAssignmentLimit),
	}, nil
}

// AssignmentProvisioner provisions the principal assignment in AWS Identity Center.
type AssignmentProvisioner struct {
	ProvisionerConfig

	accountLock   sync.RWMutex
	knownAccounts set.Set[services.IdentityCenterAccountID]

	// createAssignmentSemaphore is a semaphore to control the number of concurrent
	// Create- and DeleteAccountAssignment calls that can be made. In Teleport,
	// both operations draw from the same pool of concurrent calls, but it's not
	// definitively known if that is also how AWS enforces the limit.
	createAssignmentSemaphore *semaphore.Weighted
}

func (a *AssignmentProvisioner) SetKnownAccounts(accts ...services.IdentityCenterAccountID) {
	a.accountLock.Lock()
	defer a.accountLock.Unlock()
	a.knownAccounts = set.New(accts...)
}

func (a *AssignmentProvisioner) isKnownAccount(acct services.IdentityCenterAccountID) bool {
	a.accountLock.RLock()
	defer a.accountLock.RUnlock()
	return a.knownAccounts.Contains(acct)
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

	log.DebugContext(ctx, "Fetching existing Account Assignments")

	awsAssignments, err := a.fetchAWSAssignments(ctx, externalID, principalType)
	if err != nil {
		if trace.IsNotFound(err) {
			log.WarnContext(ctx, "Downstream principal not found")

			// Use a missingPrincipalError to indicate to the caller that the
			// AWS principal is missing and needs re-provisioning
			return nil, &MissingPrincipalError{Principal: principal}
		}
		return nil, trace.Wrap(err)
	}
	teleportAssignments := convertToICSDKAssignments(principal.GetStatus().GetAssignments(), principalType)

	diffCalc := assignmentDiffCalculator{
		teleportAssignments: set.New(dereferenceSlice(teleportAssignments)...),
		awsAssignments:      set.New(dereferenceSlice(awsAssignments)...),
	}

	var g errGroup
	g.SetLimit(a.MaxConcurrentRequests)

	for item := range diffCalc.assignmentsToCreate() {
		g.Go(func() error {
			err := a.createAssignment(ctx, externalID, item.PermissionSetARN, item.AccountID, principalType)
			if err != nil {
				log.WarnContext(ctx, "Failed to create AWS IC assignment",
					"permission_set_arn", item.PermissionSetARN,
					"account_id", item.AccountID,
					"error", err,
				)
				return trace.Wrap(err)
			}
			return nil
		})
	}
	for item := range diffCalc.assignmentsToDelete() {

		if !a.isKnownAccount(services.IdentityCenterAccountID(item.AccountID)) {
			continue
		}

		g.Go(func() error {
			if err := a.deleteAssignment(ctx, externalID, item.PermissionSetARN, item.AccountID, principalType); err != nil {
				log.WarnContext(ctx, "Failed to delete AWS IC assignment",
					"permission_set_arn", item.PermissionSetARN,
					"account_id", item.AccountID,
					"error", err,
				)
				return trace.Wrap(err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
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

	if err := a.createAssignmentSemaphore.Acquire(ctx, 1); err != nil {
		return trace.Wrap(err)
	}
	defer a.createAssignmentSemaphore.Release(1)

	resp, err := a.SDKClient.CreateAccountAssignment(ctx, &icsdk.CreateAccountAssignmentRequest{
		PrincipalID:      externalID,
		PermissionSetARN: permissionSetARN,
		AccountID:        accountID,
		PrincipalType:    principalType,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.SDKClient.WaitForCreateAccountAssignmentResult(ctx, resp.RequestID); err != nil {
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

	if err := a.createAssignmentSemaphore.Acquire(ctx, 1); err != nil {
		return trace.Wrap(err)
	}
	defer a.createAssignmentSemaphore.Release(1)

	resp, err := a.SDKClient.DeleteAccountAssignment(ctx, &icsdk.DeleteAccountAssignmentRequest{
		PrincipalID:      externalID,
		PermissionSetARN: permissionSetARN,
		AccountID:        accountID,
		PrincipalType:    principalType,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.SDKClient.WaitForDeleteAccountAssignmentResult(ctx, resp.RequestID); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// fetchAWSAssignments retrieves the assignments from AWS Identity Center
// and filters them by the principal type if necessary.
func (a *AssignmentProvisioner) fetchAWSAssignments(ctx context.Context, externalID string, principalType ssoadmintypes.PrincipalType) ([]*icsdk.Assignment, error) {
	awsAssignments, err := a.SDKClient.ListAssignments(ctx, externalID, principalType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return filterAssignmentsByType(awsAssignments, principalType), nil
}

// filterAssignmentsByType filters a list of assignments by the given principal type.
func filterAssignmentsByType(in []*icsdk.Assignment, principalType ssoadmintypes.PrincipalType) []*icsdk.Assignment {
	out := make([]*icsdk.Assignment, 0, len(in))
	for _, v := range in {
		if v.PrincipalType == principalType {
			out = append(out, v)
		}
	}
	return out
}

type assignmentDiffCalculator struct {
	teleportAssignments set.Set[icsdk.Assignment]
	awsAssignments      set.Set[icsdk.Assignment]
}

// assignmentsToCreate calculates  the set of Identity Center assignment that
// need to be created in order to sync the Teleport assignment set with AWS.
func (d *assignmentDiffCalculator) assignmentsToCreate() set.Set[icsdk.Assignment] {
	return d.teleportAssignments.Clone().Subtract(d.awsAssignments)
}

// assignmentsToDelete calculates the set of Identity Center assignments that
// need to be deleted in order to sync the Teleport assignment set with AWS.
func (d *assignmentDiffCalculator) assignmentsToDelete() set.Set[icsdk.Assignment] {
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

// Note: icsdk.Assignment struct returned by this function should always be aligned with
// icsdk.Assignment type returned from icsdk ListAssignments method. Otherwise the difference in
// struct field will make diff calculator produce incorrect diff value.
func convertToICSDKAssignments(in []*pb.AccountAssignmentRef, principalType ssoadmintypes.PrincipalType) []*icsdk.Assignment {
	out := make([]*icsdk.Assignment, 0, len(in))
	for _, v := range in {
		out = append(out, &icsdk.Assignment{
			AccountID:        v.GetAccountId(),
			PermissionSetARN: v.GetPermissionSetArn(),
			PrincipalType:    principalType,
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

// errGroup captures and aggregates errors from concurrent tasks.
// it is a wrapper around errgroup.Group that captures only the first error and cancels the rest.
// were wrapped errors are aggregated and wait for all tasks to finish.
type errGroup struct {
	errGroup errgroup.Group
	err      []error
	mtx      sync.Mutex
}

func (g *errGroup) SetLimit(limit int) {
	g.errGroup.SetLimit(limit)
}

func (g *errGroup) Go(fn func() error) {
	g.errGroup.Go(func() error {
		err := fn()
		if err != nil {
			g.mtx.Lock()
			g.err = append(g.err, err)
			g.mtx.Unlock()
		}
		return nil
	})
}

func (g *errGroup) Wait() error {
	//  discard g.Wait's error as we're collecting all errors manually
	_ = g.errGroup.Wait()
	return trace.NewAggregate(g.err...)
}

// dereferenceSlice takes a slice of pointers to T and returns a slice of T values
func dereferenceSlice[T any](ptrs []*T) []T {
	result := make([]T, len(ptrs))
	for i, ptr := range ptrs {
		if ptr != nil {
			result[i] = *ptr
		}
	}
	return result
}
