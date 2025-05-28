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

package tokens

import (
	"github.com/gravitational/trace"

	scopedtokenv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopedtoken/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

const (

	// KindScopedToken is the kind of a scoped token resource.
	KindScopedToken = "scoped_token"
)

// StrongValidateToken performs robust validation of a token to ensure it complies with all expected constraints. Prefer
// using this function for validating tokens loaded from "external" sources (e.g. user input), and [scopes.WeakValidateResource] for
// validating tokens loaded from "internal" sources (e.g. backend/control-plane).
func StrongValidateToken(token *scopedtokenv1.ScopedToken) error {
	if err := scopes.ValidateScopedResource(token, KindScopedToken, types.V1); err != nil {
		return trace.Wrap(err)
	}

	if err := scopes.StrongValidateSegment(token.GetMetadata().GetName()); err != nil {
		return trace.BadParameter("scoped token name %q does not conform to segment naming rules: %v", token.GetMetadata().GetName(), err)
	}

	if err := scopes.StrongValidate(token.GetScope()); err != nil {
		return trace.BadParameter("scoped token %q has invalid scope: %v", token.GetMetadata().GetName(), err)
	}

	if len(token.GetSpec().GetAssignedScope()) == 0 {
		return trace.BadParameter("scoped token %q does not have any assignable scopes", token.GetMetadata().GetName())
	}

	if err := scopes.StrongValidateGlob(token.GetSpec().GetAssignedScope()); err != nil {
		return trace.BadParameter("scoped token %q has invalid assignable scope %q: %v", token.GetMetadata().GetName(), token.GetSpec().GetAssignedScope(), err)
	}

	if !scopes.Glob(token.GetSpec().GetAssignedScope()).IsSubjectToPolicyResourceScope(token.GetScope()) {
		return trace.BadParameter("scoped token %q has assignable scope %q that is not a sub-scope of the token's scope %q", token.GetMetadata().GetName(), token.GetSpec().GetAssignedScope(), token.GetScope())
	}

	return nil
}
