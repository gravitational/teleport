/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package decision

import (
	"context"
	"errors"

	decision "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/trace"
)

var (
	errNoIdentity = errors.New("authorization decision request missing identity")
)

// DecisionServiceConfig configures the DecisionService.
type DecisionServiceConfig struct {
	// Authorizer is a helper used to authorize various operations typically performed by
	// teleport TLS identities.
	Authorizer authz.Authorizer
}

// contextForAuthorizer creates a context with which to present authz.Authorizer with the supplied identity. The returned
// context inherets deadline/cancellation from the supplied parent but does not inherit any values, ensuring that we don't
// corrupt the decision with state originating outside of the decision request message.
func contextForAuthorizer(ctx context.Context, protoIdentity *decision.Identity) context.Context {
	// start with background context to ensure we don't carry-over any values from the parent context
	dctx := context.Background()

	// ensure decision context has same deadline as parent context
	if deadline, ok := ctx.Deadline(); ok {
		dctx, _ = context.WithDeadline(dctx, deadline)
	}

	// ensure decision context is canceled if parent context is canceled
	if cancel := ctx.Done(); cancel != nil {
		var dctxCancel context.CancelFunc
		dctx, dctxCancel = context.WithCancel(dctx)
		go func() {
			<-cancel
			dctxCancel()
		}()
	}

	identity := IdentityFromProto(protoIdentity)

	_ = identity

	// TODO: refactor auth.Middleware.WrapContextWithUser s.t. we can reuse as much of
	// it as possible in the setup here so that we have a single source of truth for how we map identities
	// into authz context info.

	// XXX: alternatively, should we refactor authz.Authorizer to *not* rely on context variables, and instead
	// provide a wrapper that extracts params from a context? It might be more correct to have the core impl not
	// rely on weird context magic.

	panic("not implemented")
}

var _ decision.DecisionServiceServer = (*DecisionService)(nil)

// DecisionService is a service that evaluates authorization decision requests. This is the core abstraction/boundary layer
// that separates Policy Decision Point (PDP) and Policy Enforcement Point (PEP) in most teleport access-control systems.
type DecisionService struct {
	decision.UnimplementedDecisionServiceServer
	cfg DecisionServiceConfig
}

func (s *DecisionService) EvaluateSSHAccess(ctx context.Context, req *decision.EvaluateSSHAccessRequest) (*decision.EvaluateSSHAccessResponse, error) {

	// TODO(fspmarshall): finish refactoring srv.IdentityContext so we have a suitable conversion target for decision.SSHIdentity

	panic("not implemented")
}

func (s *DecisionService) EvaluateDatabaseAccess(ctx context.Context, req *decision.EvaluateDatabaseAccessRequest) (*decision.EvaluateDatabaseAccessResponse, error) {
	if req.User == nil {
		return nil, trace.Wrap(errNoIdentity)
	}

	authCtx, err := s.cfg.Authorizer.Authorize(contextForAuthorizer(ctx, req.User))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_ = authCtx

	panic("not implemented")
}
