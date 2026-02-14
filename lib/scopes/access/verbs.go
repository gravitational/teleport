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
package access

import "github.com/gravitational/teleport/api/types"

// isAllowedScopedVerb returns true if the given verb is allowed for the given scoped resource kind. Scoped roles are not sane
// for use with all resource kind/verb combinations, so we restrict the allowed combinations here.
func isAllowedScopedRule(kind string, verb string) bool {
	switch kind {
	case KindScopedRole:
		// scoped roles can be read/written, and do not currently contain a concept of a secret.
		return isReadWriteNoSecrets(verb)
	case KindScopedRoleAssignment:
		// scoped role assignments can be read/written, and do not currently contain a concept of a secret.
		return isReadWriteNoSecrets(verb)
	case types.KindScopedToken:
		// scoped tokens can be read/written, and contain secrets.
		return isReadWriteWithSecrets(verb) || isReadWriteNoSecrets(verb)
	default:
		return false
	}
}

// isReadWriteNoSecrets returns true if the given verb conforms to the "read-write-no-secrets" category of verbs.
func isReadWriteNoSecrets(verb string) bool {
	return isReadNoSecrets(verb) || isWrite(verb)
}

// isReadWriteWithSecrets returns true if the given verb conforms to the "read-write-with-secrets" category of verbs.
func isReadWriteWithSecrets(verb string) bool {
	return isReadWithSecrets(verb) || isWrite(verb)
}

// isReadNoSecrets returns true if the given verb conforms to the "read-no-secrets" category of verbs.
func isReadNoSecrets(verb string) bool {
	switch verb {
	case types.VerbList, types.VerbReadNoSecrets:
		return true
	default:
		return false
	}
}

// isReadWithSecrets returns true if the given verb conforms to the "read-with-secrets" category of verbs.
func isReadWithSecrets(verb string) bool {
	switch verb {
	case types.VerbList, types.VerbRead:
		return true
	default:
		return false
	}
}

// isWrite returns true if the given verb conforms to the "write" category of verbs.
func isWrite(verb string) bool {
	switch verb {
	case types.VerbCreate, types.VerbUpdate, types.VerbDelete:
		return true
	default:
		return false
	}
}
