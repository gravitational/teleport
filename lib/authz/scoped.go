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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// ScopedAuthorizer provides an equivalent to the Authorizer.Authorize method intended for use
// with scoped identities.
type ScopedAuthorizer interface {

	// AuthorizeScoped authorizes a scoped identity. Note that this a prototype implementation and
	// does not have feature parity with standard Authorize. Scopes are a work in progress feature.
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

// AuthorizeScoped authorizes a scoped identity. This function should generally only be called by methods that
// implement scoping support, and only if [ErrScopedIdentity] is returned by [Authorize].
// XXX: this is a protoype implementation and does not have feature parity with standard Authorize. Scopes are
// a work in progress feature. This method does not function without the unstable scope flag being set, and must
// remain behind the feature flag until core functionality is finalized.
func (a *authorizer) AuthorizeScoped(ctx context.Context) (scopedCtx *ScopedContext, err error) {
	defer func() {
		if err != nil {
			err = a.convertAuthorizerError(err)
		}
	}()

	if !scopes.FeatureEnabled() {
		return nil, trace.AccessDenied("cannot authorize scoped operation, scoping is not enabled for this cluster")
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

	scopedCtx, err = scopedContextForLocalUser(ctx, user, a.scopedRoleReader, a.clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(fspmarshall/scopes): include controls like locks/device enforcement here or (better), refactor to
	// have enforcement use a common implementation across scoped/unscoped authorize variants.

	return scopedCtx, nil
}

func scopedContextForLocalUser(ctx context.Context, user LocalUser, reader services.ScopedRoleReader, clusterName string) (*ScopedContext, error) {
	accessInfo, err := services.AccessInfoFromLocalTLSIdentity(user.Identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	checkerContext, err := services.NewScopedAccessCheckerContext(ctx, accessInfo, clusterName, reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ScopedContext{
		CheckerContext: checkerContext,
	}, nil
}

// ScopedContext is a scope-enabled authorization context.
type ScopedContext struct {
	CheckerContext *services.ScopedAccessCheckerContext
}
