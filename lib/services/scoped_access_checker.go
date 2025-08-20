package services

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/constants"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/lib/scopes/pinning"
	"github.com/gravitational/trace"
)

// ScopedAccessChecker is similar to AccessChecker, but performs scoped checks. Note that usage patterns differ with scoped
// access checks in comparison to standard access checkers since scoped access checkers can be set up "at" various scopes, and
// the scope of subsequent checks may depend on the scope at which an allow was reached in a previous check. For example, if an
// allow decision was reached for ssh access at a particular node, then checks for parameters (e.g. x11 forwarding) must be performed
// at that same scope, even if that scope is a parent of the node's scope rather than the node's scope itself. On the other hand,
// a ListNodes operation should have read permssions for each individual node evaluated starting from the root scope, and descending
// to the node's scope if necessary. In general, a decision *about* access to a resource always happens at the highest scope along
// the resource's scope path at which primary access to the resource is allowed.
type ScopedAccessChecker struct {
	scope   string
	checker *accessChecker
}

// NewScopedAccessCheckerAtScope creates a scoped access checker at the specified scope. Note that we rarely actually want to
// manually create a scoped access checker at a scope other than root. Scoped access checks must halt at the first scope that
// meets an explicit decision. The only currently known exception is pinned certificate creation, which requires creating an
// access checker at the pinned scope directly in order to determine the certificate parameters that the user should have at
// that scope.
func NewScopedAccessCheckerAtScope(ctx context.Context, scope string, info *AccessInfo, localCluster string, reader ScopedRoleReader) (*ScopedAccessChecker, error) {
	if info.ScopePin == nil {
		return nil, trace.BadParameter("cannot create scoped access checker at scope %q for unscoped identity", scope)
	}

	if len(info.AllowedResourceIDs) != 0 {
		return nil, trace.BadParameter("cannot create scoped access checker for identity with active resource IDs")
	}

	// validate that the scope pin is well-formed
	if err := pinning.WeakValidate(info.ScopePin); err != nil {
		return nil, trace.Errorf("cannot create scoped access checker at scope %q: %w", scope, err)
	}

	// validate that the resource scope is subject to the scope pin
	if !pinning.PinAppliesToResourceScope(info.ScopePin, scope) {
		return nil, trace.AccessDenied("an identity pinned to scope %q cannot be used to access resources at scope %q", info.ScopePin.GetScope(), scope)
	}

	// get an iterator of all assignments relevant to the scope of access being checked
	assignments, err := pinning.AssignmentsForResourceScope(info.ScopePin, scope)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// fetch all assigned scoped roles and convert them to classic roles to drive the underlying access checks.
	var roles []types.Role
	for scope, assigned := range assignments {
		if len(assigned.GetRoles()) == 0 {
			continue
		}

		rolesForScope, err := fetchAndConvertScopedRoles(ctx, scope, assigned.GetRoles(), reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		roles = append(roles, rolesForScope...)
	}

	checker := newAccessChecker(info, localCluster, NewRoleSet(roles...))

	return &ScopedAccessChecker{
		scope:   scope,
		checker: checker,
	}, nil
}

func (c *ScopedAccessChecker) ScopePin() *scopesv1.Pin {
	return c.checker.info.ScopePin
}

func (c *ScopedAccessChecker) Traits() wrappers.Traits {
	// identical in scoped/unscoped contexts generally (there is no concept of
	// scoped traits currently, and none is planned or would be feasible at least
	// until we've fully migrated to PDP and deprecated certificate-based traits).
	return c.checker.Traits()
}

func (c *ScopedAccessChecker) CheckAccessToRemoteCluster(cluster types.RemoteCluster) error {
	// remote cluster access is never permitted for scoepd identities
	// NOTE: it is unclear wether or not this method should even be implemented for the scoped access checker. it may be more sensible
	// to force outer enforcement logic to grapple with the fact that a scoped checker does not support remote clusters at the
	// type-level. this has been implemented experimentally to explore the pattern of having the scoped access checker implement
	// methods that always deny for unsupported features.
	return trace.AccessDenied("remote cluster access is not permitted for scoped identities")
}

func (c *ScopedAccessChecker) CheckLoginDuration(ttl time.Duration) ([]string, error) {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.CheckLoginDuration(ttl)
}

func (c *ScopedAccessChecker) AdjustSessionTTL(ttl time.Duration) time.Duration {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.AdjustSessionTTL(ttl)
}

func (c *ScopedAccessChecker) PrivateKeyPolicy(defaultPolicy keys.PrivateKeyPolicy) (keys.PrivateKeyPolicy, error) {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.PrivateKeyPolicy(defaultPolicy)
}

func (c *ScopedAccessChecker) PinSourceIP() bool {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.PinSourceIP()
}

func (c *ScopedAccessChecker) CanPortForward() bool {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.

	// NOTE: internally this method relies upon the SSHPortForwardMode() method. if future work
	// on the scoped access checker causes us to change the behavior of that method, we will
	// need to rework this method as well to ensure that it behaves consistently.
	return c.checker.CanPortForward()
}

func (c *ScopedAccessChecker) CanForwardAgents() bool {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.CanForwardAgents()
}

func (c *ScopedAccessChecker) PermitX11Forwarding() bool {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.PermitX11Forwarding()
}

func (c *ScopedAccessChecker) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	// the naive implementation of this method for scopes may have problematic interactions with
	// cert parameter generation. see ../scopes/access/compat.go for more detailed discussion.
	return c.checker.LockingMode(defaultMode)
}

func (c *ScopedAccessChecker) AccessInfo() *AccessInfo {
	return c.checker.info
}

// fetchAndConvertScopedRoles fetches scoped roles by name and converts them to classic roles.
func fetchAndConvertScopedRoles(ctx context.Context, scope string, names []string, reader ScopedRoleReader) ([]types.Role, error) {
	roles := make([]types.Role, 0, len(names))
	for _, name := range names {
		rsp, err := reader.GetScopedRole(ctx, &scopedaccessv1.GetScopedRoleRequest{
			Name: name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		role, err := scopedaccess.ScopedRoleToRole(rsp.Role, scope)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// TODO(fspmarshall): figure out how/when we want to support trait interpolation in scoped
		// roles. When we do, that will likely need to be done here. Wether we should perform trait
		// interpolation on the per-conversion scoped role and add support piecemeal, or inherit
		// identical trait interpolation from classic role behavior isn't clear yet, so at this
		// stage we're just opting to skip entirely.

		roles = append(roles, role)
	}

	return roles, nil
}
