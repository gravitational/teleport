// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package joining

import (
	"time"

	"github.com/gravitational/trace"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

var rolesSupportingScopes = types.SystemRoles{
	types.RoleNode,
}

var joinMethodsSupportingScopes = map[string]struct{}{
	string(types.JoinMethodToken): {},
}

// StrongValidateToken checks if the scoped token is well-formed according to
// all scoped token rules. This function *must* be used to validate any scoped
// token being created from scratch. When validating existing scoped token
// resources, this function should be avoided in favor of the
// [WeakValidateToken] function.
func StrongValidateToken(token *joiningv1.ScopedToken) error {
	if expected, actual := types.KindScopedToken, token.GetKind(); expected != actual {
		return trace.BadParameter("expected kind %v, got %q", expected, actual)
	}
	if expected, actual := types.V1, token.GetVersion(); expected != actual {
		return trace.BadParameter("expected version %v, got %q", expected, actual)
	}
	if expected, actual := "", token.GetSubKind(); expected != actual {
		return trace.BadParameter("expected sub_kind %v, got %q", expected, actual)
	}
	if name := token.GetMetadata().GetName(); name == "" {
		return trace.BadParameter("missing name")
	}

	if token.GetScope() == "" {
		return trace.BadParameter("scoped token must have a scope assigned")
	}

	spec := token.GetSpec()
	if spec == nil {
		return trace.BadParameter("spec must not be nil")
	}

	if err := scopes.StrongValidate(token.GetScope()); err != nil {
		return trace.Wrap(err, "validating scoped token resource scope")
	}

	if err := scopes.StrongValidate(spec.AssignedScope); err != nil {
		return trace.Wrap(err, "validating scoped token assigned scope")
	}

	if !scopes.ResourceScope(spec.AssignedScope).IsSubjectToPolicyScope(token.GetScope()) {
		return trace.BadParameter("scoped token assigned scope must be descendant of its resource scope")
	}

	if _, ok := joinMethodsSupportingScopes[spec.JoinMethod]; !ok {
		return trace.BadParameter("join method %q does not support scoping", spec.JoinMethod)
	}

	if token.GetStatus().GetSecret() == "" && types.JoinMethod(spec.JoinMethod) == types.JoinMethodToken {
		return trace.BadParameter("secret value must be defined for a scoped token when using the token join method")
	}

	if len(spec.Roles) == 0 {
		return trace.BadParameter("scoped token must have at least one role")
	}

	roles, err := types.NewTeleportRoles(spec.Roles)
	if err != nil {
		return trace.Wrap(err, "validating scoped token roles")
	}

	for _, role := range roles {
		if !rolesSupportingScopes.Include(role) {
			return trace.BadParameter("role %q does not support scoping", role)
		}
	}

	return nil
}

// WeakValidateToken performs a weak form of validation on a scoped token. This
// function is intended to catch bugs/incompatibilites that might have resulted
// in a scoped token too malformed for us to safely reason about (e.g. due to
// significant version drift). Use this function to validate scoped tokens
// propagated from the control plane. Prefer using [StrongValidateToken] when
// building a new scoped token from scratch.
func WeakValidateToken(token *joiningv1.ScopedToken) error {
	if token == nil {
		return trace.BadParameter("missing scoped token")
	}

	if err := scopes.WeakValidate(token.GetScope()); err != nil {
		return trace.Wrap(err, "validating scoped token resource scope")
	}

	if err := scopes.WeakValidate(token.GetSpec().GetAssignedScope()); err != nil {
		return trace.Wrap(err, "validating scoped token assigned scope")
	}

	if len(token.GetSpec().GetRoles()) == 0 {
		return trace.BadParameter("scoped token must have at least one role")
	}

	if _, ok := joinMethodsSupportingScopes[token.GetSpec().GetJoinMethod()]; !ok {
		return trace.BadParameter("join method %q does not support scoping", token.GetSpec().GetJoinMethod())
	}

	return nil
}

// ValidateTokenForUse checks if a given scoped token can be used for
// provisioning.
func ValidateTokenForUse(token *joiningv1.ScopedToken) error {
	if err := WeakValidateToken(token); err != nil {
		return trace.Wrap(err)
	}

	ttl := token.GetMetadata().GetExpires()
	if ttl == nil || ttl.AsTime().IsZero() {
		return nil
	}

	now := time.Now().UTC()
	if ttl.AsTime().Before(now) {
		return trace.LimitExceeded("scoped token is expired")
	}

	return nil
}

// Token wraps a [joiningv1.ScopedToken] such that it can be used to provision
// resources.
type Token struct {
	scoped     *joiningv1.ScopedToken
	joinMethod types.JoinMethod
	roles      types.SystemRoles
}

// NewToken returns the wrapped version of the given [joiningv1.ScopedToken].
// It will return an error if the configured join method is not a valid
// [types.JoinMethod] or if any of the configured roles are not a valid
// [types.SystemRole]. The validated join method and roles are cached on the
// [Scoped] wrapper itself so they can be read without repeating validation.
func NewToken(token *joiningv1.ScopedToken) (*Token, error) {
	joinMethod := types.JoinMethod(token.GetSpec().GetJoinMethod())
	if err := types.ValidateJoinMethod(joinMethod); err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := types.NewTeleportRoles(token.GetSpec().GetRoles())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Token{scoped: token, joinMethod: joinMethod, roles: roles}, nil
}

// GetName returns the name of a [joiningv1.ScopedToken].
func (t *Token) GetName() string {
	if t == nil {
		return ""
	}

	return t.scoped.GetMetadata().GetName()
}

// GetJoinMethod returns the cached [types.JoinMethod] generated when the
// [joiningv1.ScopedToken] was wrapped.
func (t *Token) GetJoinMethod() types.JoinMethod {
	if t == nil {
		return types.JoinMethodUnspecified
	}

	return t.joinMethod
}

// GetRoles returns the cached [types.SystemRoles] generated when the
// [joiningv1.ScopedToken] was wrapped.
func (t *Token) GetRoles() types.SystemRoles {
	if t == nil {
		return nil
	}
	return t.roles
}

// GetSafeName returns the name the santiized name of the scoped token. Because
// scoped token names are not secret, this is just an alias for [GetName].
func (t *Token) GetSafeName() string {
	return t.GetName()
}

// Expiry returns the [time.Time] representing when the wrapped
// [joiningv1.ScopedToken] will expire.
func (t *Token) Expiry() time.Time {
	expiry := t.scoped.GetMetadata().GetExpires()
	if expiry == nil {
		return time.Time{}
	}

	return expiry.AsTime()
}

// GetBotName returns an empty string because scoped tokens do not currently
// support configuring a bot name.
func (t *Token) GetBotName() string {
	return ""
}

// GetAssignedScope returns the scope that will be assigned to resources
// provisioned using the wrapped [joiningv1.ScopedToken].
func (t *Token) GetAssignedScope() string {
	return t.scoped.GetSpec().GetAssignedScope()
}

// GetAllowRules returns the list of allow rules.
func (t *Token) GetAllowRules() []*types.TokenRule {
	return nil
}

// GetAWSIIDTTL returns the TTL of EC2 IIDs
func (t *Token) GetAWSIIDTTL() types.Duration {
	return types.NewDuration(0)
}

// GetIntegration returns the Integration field which is used to provide
// credentials that will be used when validating the AWS Organization if required by an IAM Token.
func (t *Token) GetIntegration() string {
	return ""
}

// GetSecret returns the token's secret value.
func (t *Token) GetSecret() (string, bool) {
	return t.scoped.GetStatus().GetSecret(), t.GetJoinMethod() == types.JoinMethodToken
}
