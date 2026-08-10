/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// ScopedRef is the fully-resolved representation of a tctl resource reference,
// produced by ParseScopedRef from the two CLI positional arguments and encompasses
// both the more standard kind[/subkind]/name format, and the newer kind[/subkind] [scope::]name
// format.
//
// When Scope is empty the reference is unscoped and equivalent to a services.Ref,
// suitable for classic Handlers. When Scope is non-empty the reference is scope-qualified.
type ScopedRef struct {
	Kind    string
	SubKind string
	Name    string
	Scope   string // non-empty iff scope-qualified
}

// String returns the user-facing representation of the reference, approximately matching
// what the user originally typed on the command line (note that a user who typed in the
// newer space-separated format without a scope will see the older slash separated format
// since the scope qualification is what determines the format, not the original input).
func (sr ScopedRef) String() string {
	var b strings.Builder
	b.WriteString(sr.Kind)
	if sr.SubKind != "" {
		b.WriteByte('/')
		b.WriteString(sr.SubKind)
	}
	if sr.Scope != "" {
		b.WriteByte(' ')
		b.WriteString(scopes.QualifiedName{Scope: sr.Scope, Name: sr.Name}.String())
	} else if sr.Name != "" {
		b.WriteByte('/')
		b.WriteString(sr.Name)
	}
	return b.String()
}

// Ref converts the ScopedRef to a services.Ref for use with classic (unscoped)
// Handlers. Only meaningful when Scope is empty.
func (sr ScopedRef) Ref() services.Ref {
	return services.Ref{Kind: sr.Kind, SubKind: sr.SubKind, Name: sr.Name}
}

// SQN returns the scope-qualified name for use with ScopedHandlers.
// Only meaningful when Scope is non-empty.
func (sr ScopedRef) SQN() scopes.QualifiedName {
	return scopes.QualifiedName{Scope: sr.Scope, Name: sr.Name}
}

// ParseScopedRef resolves the two tctl positional CLI arguments into a ScopedRef. The ref and id
// positional arguments must be decoded together as we support two different formats and resource name
// may appear in either argument depending on the format.
//
// - Older/Unscoped Format: <kind>[/<subkind>]/<name>
// - Newer/Scoped Format: <kind>[/<subkind>] [<scope>::]<name>
//
// The resulting ScopedRef will have Scope set iff the input was the newer/scoped format *and* scope qualification
// was not present. When scope qualification is absent, the two formats convey the same information.
func ParseScopedRef(ref, id string) (ScopedRef, error) {
	// start with older/unscoped format parssing. this should succeed for both formats, but the older
	// style may have misinterpreted the subkind as a name if the caller is using the newer format.
	r, err := services.ParseRef(ref)
	if err != nil {
		return ScopedRef{}, trace.Wrap(err)
	}
	if id == "" {
		return ScopedRef{Kind: r.Kind, SubKind: r.SubKind, Name: r.Name}, nil
	}

	if r.SubKind != "" {
		// Three-segment first arg (kind/subkind/name) with a second arg is one
		// identifier too many.
		return ScopedRef{}, trace.BadParameter(
			"arguments must take the form '<kind>[/<subkind>] [<scope>::]<name>' or '<kind>[/<subkind>]/<name>'",
		)
	}

	// the old format always treats token/token as kind/name, but if id was set then the second token
	// is actually a subkind in the new format.
	subKind := r.Name
	if strings.Contains(id, scopes.QualifiedNameSeparator) {
		qn, err := scopes.ParseQualifiedName(id)
		if err != nil {
			return ScopedRef{}, trace.Wrap(err)
		}

		// A user may provide the token name as either <token_name> OR <token_name>:<encoded_secret>.
		// Both formats are supported to improve UX, however, only the token name is consumed
		// for tctl commands to operate properly. Strip the secret after parsing the SQN so
		// that the colon in the SQN separator is not mistaken for the token/secret separator.
		if r.Kind == types.KindScopedToken {
			qn.Name, _, _ = strings.Cut(qn.Name, ":")
		}

		// A bot instance is identified by <bot_name>/<uuid> under the bot's scope,
		// so for bot_instance refs the name component of the SQN may itself be a
		// two-part identifier containing a '/'. Validate the parts individually
		// since whole-name validation would reject the separator.
		if r.Kind == types.KindBotInstance {
			if botName, instanceID, ok := strings.Cut(qn.Name, "/"); ok {
				if err := scopes.StrongValidate(qn.Scope); err != nil {
					return ScopedRef{}, trace.BadParameter("scope-qualified name %q has invalid scope: %v", qn, err)
				}
				for _, part := range []string{botName, instanceID} {
					if err := scopes.StrongValidateSegment(part); err != nil {
						return ScopedRef{}, trace.BadParameter("scope-qualified name %q has invalid name: %v", qn, err)
					}
				}
				return ScopedRef{Kind: r.Kind, SubKind: subKind, Scope: qn.Scope, Name: qn.Name}, nil
			}
		}

		if err := qn.StrongValidate(); err != nil {
			return ScopedRef{}, trace.Wrap(err)
		}
		return ScopedRef{Kind: r.Kind, SubKind: subKind, Scope: qn.Scope, Name: qn.Name}, nil
	}
	return ScopedRef{Kind: r.Kind, SubKind: subKind, Name: id}, nil
}
