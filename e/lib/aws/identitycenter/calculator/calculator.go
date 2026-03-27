package calculator

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/equal"
	iciter "github.com/gravitational/teleport/e/lib/aws/identitycenter/iter"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter/principal"
	"github.com/gravitational/teleport/e/lib/provisioning"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/set"
)

// assignment holds the minimal set of information about an single account
// assignment during assignment calculation
type assignment struct {
	accountID        string
	permissionSetARN string
}

// ExternalIDGetter defines an interface for finding principal's external IDs
type ExternalIDGetter interface {
	GetUserExternalID(context.Context, string) (provisioning.ExternalID, error)
	GetAccessListExternalID(context.Context, string) (provisioning.ExternalID, error)
}

// RolesGetter is an abstraction ove a read-only service that provides Roles,
// defining only the operations that the Permissions Calculator service needs.
type RolesGetter interface {
	// GetRole returns role by name.
	GetRole(context.Context, string) (types.Role, error)
}

// AccountAssignmentGetter is an abstraction over fetching and listing
// Account Assignments
type AccountAssignmentGetter interface {
	// GetIdentityCenterAccountAssignment fetches a specific Identity Center Account Assignment
	GetIdentityCenterAccountAssignment(context.Context, string) (*identitycenterv1.AccountAssignment, error)

	// ListIdentityCenterAccountAssignments lists a page of IdentityCenterAccountAssignment records.
	ListIdentityCenterAccountAssignments(context.Context, int, string) ([]*identitycenterv1.AccountAssignment, string, error)
}

type LocksGetter interface {
	// GetLocks gets all/in-force locks that match at least one of the targets when specified.
	GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error)
}

type Config struct {
	// AccessRequestsSvc is used to fetches access requests of specific users
	// when calculating permission sets for users
	AccessRequestsSvc services.AccessRequestGetter

	// Clock is used to determine the validity of access requests
	Clock clockwork.Clock

	// ExternalIDGetter is used to map Teleport Users and Access Lists to their
	// Identity Center IDs
	ExternalIDGetter ExternalIDGetter

	// PrincipalAssignmentsSvc is used to write updated account assignments to a
	// principal's Principal Assignment record
	PrincipalAssignmentsSvc services.IdentityCenterPrincipalAssignments

	// AccountAssignmentCache is used to read info about account assignments
	// during permission calculation
	AccountAssignmentCache AccountAssignmentGetter

	// Logger is the logger used to write logging output. Optional. Defaults to
	// the system logger
	Logger *slog.Logger

	// RolesGetter lets the assignment calculator read roles held by users and
	// access in order to calculate the principal's effective assignment set.
	RolesGetter RolesGetter

	// LocksGetter lets the assignment calculator read locks held on roles and
	// access requests in order to calculate a principal's effective assignment
	// set.
	LocksGetter LocksGetter
}

func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.PrincipalAssignmentsSvc == nil {
		return trace.BadParameter("must supply identity center principal assignment service")
	}

	if cfg.ExternalIDGetter == nil {
		return trace.BadParameter("must supply external ID getter")
	}

	if cfg.AccessRequestsSvc == nil {
		return trace.BadParameter("must supply access requests service")
	}

	if cfg.RolesGetter == nil {
		return trace.BadParameter("must supply roles getter")
	}

	if cfg.LocksGetter == nil {
		return trace.BadParameter("must supply locks service")
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return nil
}

// AssignmentCalculator implements calculating permissions set assignments for a
// resource (with caching as necessary).
type AssignmentCalculator struct {
	Config
}

// New creates and returns a new AssignmentCalculator
func New(config Config) (*AssignmentCalculator, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &AssignmentCalculator{Config: config}, nil
}

// CalcAssignments determines the type of the principal resource (i.e. User or
// Access List) and invokes the correct assignment recalculation routine.
// Returns the updated PrincipalAssignment record
func (calc *AssignmentCalculator) CalcAssignments(ctx context.Context, principal types.Resource, principalAssignment *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error) {
	calc.Logger.Log(ctx, logutils.TraceLevel, "Recalculating", "principal", principal.GetName())
	switch p := principal.(type) {
	case *types.UserV2:
		return calc.calcUserAssignments(ctx, p, principalAssignment)
	case *accesslist.AccessList:
		return calc.calcAccessListAssignments(ctx, p, principalAssignment)
	default:
		return nil, trace.BadParameter("unsupported principal resource type %T", principal)
	}
}

// calcUserAssignments calculates the appropriate assignment set, taking into
// account
//   - their assigned roleset
//   - the roles they acquire from any active access requests
//
// We explicitly don't include any roles inherited from Access Lists, because
// the Access List role grants will be provisioned separately as Account
// Assignment on the corresponding AWS group will take care of that.
//
// If the calculated assignment set differs from one in the supplied
// PrincipalAssignment record, the record will be updated with the new assignment
// set, marked as stale, and written to the backend.
func (calc *AssignmentCalculator) calcUserAssignments(ctx context.Context, user *types.UserV2, principalAssignment *identitycenterv1.PrincipalAssignment) (*identitycenterv1.PrincipalAssignment, error) {
	log := calc.Logger.With("principal_id", principalAssignment.GetMetadata().GetName())
	log.DebugContext(ctx, "Recalculating user account assignments")

	extID := principal.GetExternalID(principalAssignment)
	if extID == "" {
		var err error
		extID, err = calc.ExternalIDGetter.GetUserExternalID(ctx, user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if extID == "" {
			return nil, trace.BadParameter("user %s has no known external id", user.GetName())
		}
	}
	log = calc.Logger.With("user", user.GetName())

	allRoles := set.New(user.GetRoles()...)
	allowedByRequest := set.New[assignment]()
	accessRequests, err := calc.getActiveAccessRequestsOnUser(ctx, user, log)
	if err != nil {
		return nil, trace.Wrap(err, "Fetching active access requests for user")
	}
	for _, req := range accessRequests {
		resources := req.GetAllRequestedResourceIDs()

		// Only add the assignments from roles if `req` is not a Resource Access
		// Request, otherwise the user will end up being given all requestable
		// roles, rather than just the set that have been specifically approved.
		if len(resources) == 0 {
			allRoles.Add(req.GetRoles()...)
		}

		assignments, err := accountAssignmentResources(ctx, req.GetAllRequestedResourceIDs(), calc.AccountAssignmentCache)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		allowedByRequest.Add(assignments...)
	}

	// build lists of possible account assignments rom the gathered roles. May
	// include duplicates and glob patterns.)
	allowExpressions, denyExpressions, err := calc.getAccountAssignmentsFromRoles(ctx, log, slices.Collect(maps.Keys(allRoles))...)
	if err != nil {
		return nil, trace.Wrap(err, "calculating account assignments")
	}

	// Reduce the allow and deny sets into a single set of allowed account
	// assignments. Handles pattern matching in the role spec. All other
	// account assignments are by definition disallowed
	allow, deny, err := calc.applyExpressions(ctx, allowExpressions, denyExpressions)
	if err != nil {
		return nil, trace.Wrap(err, "applying RBAC expressions")
	}

	// Combine the assignments allowed by both roles and access requests, and
	// then remove anything that has been denied.
	allow.Union(allowedByRequest)
	allow.Subtract(deny)

	log.DebugContext(ctx, "Writing user account assignments to record")

	updatedPrincipal, err := updatePrincipalAccountAssignments(ctx, allow, extID, principalAssignment, calc.PrincipalAssignmentsSvc)
	if err != nil {
		return nil, trace.Wrap(err, "failed to write account assignment")
	}

	log.DebugContext(ctx, "Wrote user account assignments to record",
		"state", updatedPrincipal.GetStatus().GetProvisioningState())

	return updatedPrincipal, nil
}

func accountAssignmentResources(
	ctx context.Context,
	resources []types.ResourceAccessID,
	assignmentSvc AccountAssignmentGetter,
) ([]assignment, error) {
	var result []assignment
	for _, id := range resources {
		rid := id.GetResourceID()
		if rid.Kind != types.KindIdentityCenterAccountAssignment {
			continue
		}

		asmt, err := assignmentSvc.GetIdentityCenterAccountAssignment(ctx, rid.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		result = append(result, assignment{
			accountID:        asmt.GetSpec().GetAccountId(),
			permissionSetARN: asmt.GetSpec().GetPermissionSet().GetArn(),
		})
	}
	return result, nil
}

// calcAccessListAssignments calculates the appropriate assignment set for the
// supplied access list. Only Access List membership is taken into account at
// this point; roles granted by Access List ownership are not considered.
//
// If the calculated assignment set differs from one in the supplied
// PrincipalAssignment record, the record will be updated with the new assignment
// set, marked as stale, and written to the backend.
//
// TODO: add support for roles granted by Access Request ownership
func (calc *AssignmentCalculator) calcAccessListAssignments(
	ctx context.Context,
	acl *accesslist.AccessList,
	principalAssignment *identitycenterv1.PrincipalAssignment,
) (*identitycenterv1.PrincipalAssignment, error) {
	extID := principal.GetExternalID(principalAssignment)
	if extID == "" {
		var err error
		extID, err = calc.ExternalIDGetter.GetAccessListExternalID(ctx, acl.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if extID == "" {
			return nil, trace.BadParameter("Access List %s has no known external id", acl.GetName())
		}
	}

	log := calc.Logger.With("access_list", acl.GetName())
	allowExpressions, denyExpressions, err := calc.getAccountAssignmentsFromRoles(ctx, log, acl.GetGrants().Roles...)
	if err != nil {
		return nil, trace.Wrap(err, "building account assignments")
	}

	// Reduce the allow and deny sets into a single set of allowed account
	// assignments. Handles pattern matching in the role spec. All other
	// account assignments are by definition disallowed
	allow, deny, err := calc.applyExpressions(ctx, allowExpressions, denyExpressions)
	if err != nil {
		return nil, trace.Wrap(err, "applying RBAC expressions")
	}
	allow.Subtract(deny)

	updatedPrincipal, err := updatePrincipalAccountAssignments(ctx, allow, extID, principalAssignment, calc.PrincipalAssignmentsSvc)
	if err != nil {
		return nil, trace.Wrap(err, "failed to write account assignment")
	}
	return updatedPrincipal, nil
}

// getActiveAccessRequestsOnUser fetches all Access Requests for a given
// user, excluding requests that are unapproved, locked or outside their
// specified time window.
func (calc *AssignmentCalculator) getActiveAccessRequestsOnUser(ctx context.Context, user *types.UserV2, log *slog.Logger) ([]types.AccessRequest, error) {
	var result []types.AccessRequest

	req := proto.ListAccessRequestsRequest{
		Limit: 100,
		Filter: &types.AccessRequestFilter{
			User:  user.GetName(),
			State: types.RequestState_APPROVED,
		},
	}
	for {
		response, err := calc.AccessRequestsSvc.ListAccessRequests(ctx, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		page := calc.filterOutAccessRequestsOutsideTimeWindow(ctx, response.AccessRequests, log)

		page, err = calc.filterOutLockedAccessRequests(ctx, page, log)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, ar := range page {
			result = append(result, ar)
		}

		if response.NextKey == "" {
			break
		}
		req.StartKey = response.NextKey
	}

	return result, nil
}

// selectLockedAccessRequests examines the supplied list of Access Requests and
// identifies which Requests have in-force locks against them, returning the
// locked Access Requests.
func (calc *AssignmentCalculator) selectLockedAccessRequests(ctx context.Context, requests []*types.AccessRequestV3) (set.Set[string], error) {
	if len(requests) == 0 {
		return nil, nil
	}

	lockTargets := make([]types.LockTarget, len(requests))
	for i, req := range requests {
		lockTargets[i] = types.LockTarget{AccessRequest: req.GetName()}
	}
	locks, err := calc.LocksGetter.GetLocks(ctx, true /* in force locks only */, lockTargets...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := set.New[string]()
	for _, lock := range locks {
		result.Add(lock.Target().AccessRequest)
	}

	return result, nil
}

// selectLockedRoles examines the supplied list of Role names and identifies which
// roles have in-force Locks against them, returning the locked roles as a Set
func (calc *AssignmentCalculator) selectLockedRoles(ctx context.Context, roles []string) (set.Set[string], error) {
	if len(roles) == 0 {
		return nil, nil
	}

	lockTargets := make([]types.LockTarget, len(roles))
	for i, role := range roles {
		lockTargets[i] = types.LockTarget{Role: role}
	}
	locks, err := calc.LocksGetter.GetLocks(ctx, true /* in force locks only */, lockTargets...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := set.New[string]()
	for _, lock := range locks {
		result.Add(lock.Target().Role)
	}

	return result, nil
}

// applyExpressions enumerates the known Account Assignments and applies the
// supplied expressions to generate a final set of allowed permission
// assignments
func (calc *AssignmentCalculator) applyExpressions(
	ctx context.Context,
	allowExpressions, denyExpressions []types.IdentityCenterAccountAssignment,
) (set.Set[assignment], set.Set[assignment], error) {

	allow := set.New[assignment]()
	deny := set.New[assignment]()

	for candidate, err := range iciter.AllAccountAssignments(ctx, calc.AccountAssignmentCache) {
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		allowMatches, err := assignmentMatchesExpressions(candidate, allowExpressions)
		if err != nil {
			return nil, nil, trace.Wrap(err, "testing account assignment match")
		}

		if allowMatches {
			spec := candidate.GetSpec()
			allow.Add(assignment{
				accountID:        spec.GetAccountId(),
				permissionSetARN: spec.GetPermissionSet().GetArn(),
			})
		}

		denyMatches, err := assignmentMatchesExpressions(candidate, denyExpressions)
		if err != nil {
			return nil, nil, trace.Wrap(err, "testing account assignment match")
		}

		if denyMatches {
			spec := candidate.GetSpec()
			deny.Add(assignment{
				accountID:        spec.GetAccountId(),
				permissionSetARN: spec.GetPermissionSet().GetArn(),
			})
		}
	}

	return allow, deny, nil
}

func sortAssignments(a, b *identitycenterv1.AccountAssignmentRef) int {
	n := strings.Compare(a.AccountId, b.AccountId)
	if n != 0 {
		return n
	}
	return strings.Compare(a.PermissionSetArn, b.PermissionSetArn)
}

// updatePrincipalAccountAssignments conditionally updates the supplied
// Principal Assignment record with the supplied assignment set, writing the
// updated record to the backend data service if there are any changes.
func updatePrincipalAccountAssignments(
	ctx context.Context,
	assignments set.Set[assignment],
	extID provisioning.ExternalID,
	principalAssignment *identitycenterv1.PrincipalAssignment,
	icSvc services.IdentityCenterPrincipalAssignments,
) (*identitycenterv1.PrincipalAssignment, error) {

	// Step 1: Unpack the assignment set into a sorted list of AccountAssignmentRef
	//         records. This is the format the assignments need to be in order
	//         to be written out to the back end with the rest of the
	//         PrincipalAssignment record.
	newAssignments := make([]*identitycenterv1.AccountAssignmentRef, 0, len(assignments))
	for a := range assignments {
		newAssignments = append(newAssignments, &identitycenterv1.AccountAssignmentRef{
			AccountId:        a.accountID,
			PermissionSetArn: a.permissionSetARN,
		})
	}
	slices.SortFunc(newAssignments, sortAssignments)

	// Step 2: Define a function that will check to see if the supplied
	//         assignment set is the same as the one in the PrincipalAssignment
	//         record, updating the record and marking it as stale if the
	//         assignment sets differ.
	setAssignments := func(pa *identitycenterv1.PrincipalAssignment) error {
		stale := false

		if pa.Spec.ExternalId != string(extID) {
			pa.Spec.ExternalId = string(extID)
			stale = true
		}

		if !slices.EqualFunc(newAssignments, pa.Status.Assignments, equal.AccountAssignmentRefEqual) {
			pa.Status.Assignments = newAssignments
			stale = true
		}

		if !stale {
			return principal.ErrNoUpdateRequired
		}

		pa.Status.ProvisioningState = identitycenterv1.ProvisioningState_PROVISIONING_STATE_STALE
		return nil
	}

	// Step 3: Give the supplied PrincipalAssignment and the mutator function to
	//         principal.Update(), which will use the mutator function to update
	//         the record, possibly invoking it multiple times as it handles
	//         retries due to optimistic locking.
	updatedPrincipal, err := principal.Update(ctx, icSvc, principalAssignment, setAssignments)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return updatedPrincipal, nil
}

func assignmentMatchesExpressions(
	candidate *identitycenterv1.AccountAssignment,
	expressions []types.IdentityCenterAccountAssignment,
) (bool, error) {
	for _, exp := range expressions {
		// Note: MatchString handles globbing and automatically caches the
		//       regular expressions it uses. Don't try and overthink it here.
		accountMatches, err := utils.MatchString(candidate.Spec.AccountId, exp.Account)
		if err != nil {
			return false, trace.Wrap(err, "checking account match")
		}
		if !accountMatches {
			continue
		}

		permissionSetMatches, err := utils.MatchString(
			candidate.Spec.PermissionSet.Arn, exp.PermissionSet)
		if err != nil {
			return false, trace.Wrap(err, "checking permission set match")
		}
		if !permissionSetMatches {
			continue
		}

		return true, nil
	}

	return false, nil
}

// getAccountAssignmentsFromRoles builds a list of account assignments that are
// granted or denied by the supplied list of roles.
//
// TODO: Handle account assignments generated with role templates
func (calc *AssignmentCalculator) getAccountAssignmentsFromRoles(ctx context.Context, log *slog.Logger, roles ...string) (allow, deny []types.IdentityCenterAccountAssignment, err error) {
	roles, err = calc.filterOutLockedRoles(ctx, roles, log)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	for _, roleName := range roles {
		role, err := calc.RolesGetter.GetRole(ctx, roleName)
		if err != nil {
			// We can't ignore failing to load a role: what if it had a deny rule?
			log.ErrorContext(ctx, "failed loading role", "error", err, "role", roleName)
			return nil, nil, trace.Wrap(err)
		}

		roleV6, ok := role.(*types.RoleV6)
		if !ok {
			return nil, nil, trace.BadParameter("unexpected Role type: %T", role)
		}
		allow = append(allow, roleV6.Spec.Allow.AccountAssignments...)
		deny = append(deny, roleV6.Spec.Deny.AccountAssignments...)
	}

	return allow, deny, nil
}

// filterOutLockedRoles examines the supplied list of role names, deleting any
// roles that have an in-force lock against them
func (calc *AssignmentCalculator) filterOutLockedRoles(ctx context.Context, roleNames []string, log *slog.Logger) ([]string, error) {
	lockedRoles, err := calc.selectLockedRoles(ctx, roleNames)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	isLocked := func(roleName string) bool {
		if lockedRoles.Contains(roleName) {
			log.Log(ctx, logutils.TraceLevel, "Excluding role due to lock", "role", roleName)
			return true
		}
		return false
	}

	return slices.DeleteFunc(roleNames, isLocked), nil
}

func (calc *AssignmentCalculator) filterOutAccessRequestsOutsideTimeWindow(ctx context.Context, accessRequests []*types.AccessRequestV3, log *slog.Logger) []*types.AccessRequestV3 {
	now := calc.Clock.Now()

	isOutsideTimeWindow := func(a *types.AccessRequestV3) bool {
		if startTime := a.GetAssumeStartTime(); startTime != nil {
			if now.Before(*startTime) {
				log.Log(ctx, logutils.TraceLevel, "Excluding access request; before start time", "access_request", a.GetName())
				return true
			}
		}
		if now.After(a.GetAccessExpiry()) {
			log.Log(ctx, logutils.TraceLevel, "Excluding access request; after expiry", "access_request", a.GetName())
			return true
		}
		return false
	}
	return slices.DeleteFunc(accessRequests, isOutsideTimeWindow)
}

func (calc *AssignmentCalculator) filterOutLockedAccessRequests(ctx context.Context, accessRequests []*types.AccessRequestV3, log *slog.Logger) ([]*types.AccessRequestV3, error) {
	lockedRequests, err := calc.selectLockedAccessRequests(ctx, accessRequests)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	isLocked := func(a *types.AccessRequestV3) bool {
		if lockedRequests.Contains(a.GetName()) {
			log.Log(ctx, logutils.TraceLevel, "Excluding locked access request", "access_request", a.GetName())
			return true
		}
		return false
	}
	return slices.DeleteFunc(accessRequests, isLocked), nil
}
