/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package authz

import (
	"context"
	"errors"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// ScopedAuthorizer provides an equivalent to the Authorizer.Authorize method intended for use
// with scoped identities.
type ScopedAuthorizer interface {
	// AuthorizeScoped authorizes a potentially scoped identity and returns a [ScopedContext]. Under the hood, this may
	// be either a scoped or unscoped authorization depending on the identity provided in the context.
	// NOTE: the scoped authorization pathway is a prototype and does not yet have feature parity with the
	// unscoped authorization pathway. this method is intended to be safe to convert existing uses of
	// Authorizer.Authorize without losing enforcement of controls for unscoped identities, but care must be
	// taken to ensure that any required controls are implemented for scoped identities before switching
	// to use of this method.
	AuthorizeScoped(ctx context.Context) (*ScopedContext, error)
}

// NewScopedAuthorizer returns a new scoped authorizer
func NewScopedAuthorizer(opts AuthorizerOpts) (ScopedAuthorizer, error) {
	// TODO(fspmarshall/scopes): either fully bifurcate the authorizer and scoped authorizer, or
	// refactor s.t. they share a common builder and implementation.
	if opts.ScopedRoleReader == nil {
		return nil, trace.BadParameter("scoped role reader is required to create a scoped authorizer")
	}

	return newAuthorizer(opts)
}

func (a *authorizer) AuthorizeScoped(ctx context.Context) (splitCtx *ScopedContext, err error) {
	authCtx, err := a.Authorize(ctx)
	if err != nil {
		if errors.Is(err, ErrScopedIdentity) {
			return a.authorizeScoped(ctx)
		}
		return nil, trace.Wrap(err)
	}
	return ScopedContextFromUnscopedContext(authCtx), nil
}

// ScopedContextFromUnscopedContext constructs a ScopedContext from an existing unscoped Context. Useful in places
// where unscoped logic needs to invoke scoped logic.
func ScopedContextFromUnscopedContext(authCtx *Context) *ScopedContext {
	return &ScopedContext{
		User:            authCtx.User,
		CheckerContext:  services.NewUnscopedSplitAccessCheckerContext(authCtx.Checker),
		unscopedContext: authCtx,
	}
}

// authorizeScoped authorizes a scoped identity. This function should generally only be called by methods that
// implement scoping support, and only if [ErrScopedIdentity] is returned by [Authorize].
// XXX: this is a protoype implementation and does not have feature parity with standard Authorize. Scopes are
// a work in progress feature. This method does not function without the unstable scope flag being set, and must
// remain behind the feature flag until core functionality is finalized.
func (a *authorizer) authorizeScoped(ctx context.Context) (scopedCtx *ScopedContext, err error) {
	defer func() {
		if err != nil {
			err = a.convertAuthorizerError(err)
		}
	}()

	if !scopes.FeatureEnabled() {
		return nil, trace.AccessDenied("cannot authorize scoped identity, scoping is not enabled for this cluster")
	}

	if ctx == nil {
		return nil, trace.AccessDenied("missing authentication context")
	}

	userI, err := UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user, ok := userI.(LocalUser)
	if !ok {
		return nil, trace.AccessDenied("scoped authorization is only supported for local users, got %T", userI)
	}

	if user.Identity.ScopePin == nil {
		return nil, trace.AccessDenied("scoped authorization is not supported for unscoped identities")
	}

	if a.scopedRoleReader == nil {
		return nil, trace.AccessDenied("authorizer not configured for scoped authorization")
	}

	scopedCtx, err = scopedContextForLocalUser(ctx, user, a.accessPoint, a.scopedRoleReader, a.clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(fspmarshall/scopes): include controls like locks/device enforcement here or (better), refactor to
	// have enforcement use a common implementation across scoped/unscoped authorize variants.

	return scopedCtx, nil
}

func scopedContextForLocalUser(ctx context.Context, u LocalUser, accessPoint AuthorizerAccessPoint, reader services.ScopedRoleReader, clusterName string) (*ScopedContext, error) {
	user, accessInfo, err := resolveLocalUser(ctx, u, accessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	checkerContext, err := services.NewScopedAccessCheckerContext(ctx, accessInfo, clusterName, reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ScopedContext{
		User:           user,
		CheckerContext: services.NewScopedSplitAccessCheckerContext(checkerContext),
	}, nil
}

// ScopedContext is the scoped authorization context returned by [ScopedAuthorizer.AuthorizeScoped]. It provides
// access-checking materials for use in making authorization decisions in contexts where the caller may be using
// a scoped identity. This type does not yet have feature parity with the unscoped [Context] type, and should be
// used with care until scopes as a feature is more mature.
type ScopedContext struct {
	// User describes the authenticated user.
	User types.User
	// CheckerContext is the top-level access checker for the authenticated identity. This types serves a similar
	// purpose to [services.AccessChecker] but requires different usage patterns to accommodate the more complex
	// scoped decision model.
	CheckerContext *services.SplitAccessCheckerContext
	// unscopedContext is the context derived from unscoped authorization, if available. This will be nil
	// if the calling identity was scoped.
	unscopedContext *Context
}

// UnscopedContext returns the unscoped authorization context if available. In general, it is best to avoid
// relying on this method as it breaks the abstraction of scoped authorization. However, this may be useful
// for specific features/methods that have not yet been adapted to support scoped identities, but are implemented
// in a context that has already begun being ported over to being scope-aware.
func (s *ScopedContext) UnscopedContext() (*Context, bool) {
	return s.unscopedContext, s.unscopedContext != nil
}

// RuleContext returns the standard services.Context used for resource-independent rule
// evaluation.
func (s *ScopedContext) RuleContext() services.Context {
	return services.Context{
		User: s.User,
	}
}
