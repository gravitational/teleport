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

import (
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// Verb represents a verb in an access check. This type is intended to help code abstract over scoped and
// unscoped verbs, which differ slightly in their meanings. Unscoped access treats 'read' as secrets-inclusive
// and has a separate 'readnosecrets' verb.  Scoped access treats 'read' as secrets-exclusive and has a
// separate 'secrets' verb.
type Verb int

const (
	// unspecifiedVerb is the zero value and not a valid verb.
	unspecifiedVerb Verb = iota

	// List grants the ability to list resources of a kind.
	List
	// Read grants secrets-exclusive read access (corresponds to 'readnosecrets' in classic RBAC).
	Read
	// Secrets grants secrets-inclusive read access (corresponds to 'read' in classic RBAC).
	Secrets
	// Create grants the ability to create resources of a kind.
	Create
	// Update grants the ability to update resources of a kind.
	Update
	// Delete grants the ability to delete resources of a kind.
	Delete

	// lastVerb is used for iteration and not a valid verb.
	lastVerb
)

// verbStrings holds the two string encodings of a [Verb].
type verbStrings struct {
	// scoped is the string the verb is written as in a scoped role spec.
	scoped string
	// classic is the classic RBAC verb the verb compiles to (in a spec) and encodes to (in a check).
	classic string
}

// strings returns the string encodings of v, or false if v is not a valid verb. This switch is the
// single source of truth for the mapping between the verb type and its scoped/unscoped string representations.
func (v Verb) strings() (verbStrings, bool) {
	switch v {
	case List:
		return verbStrings{scoped: "list", classic: types.VerbList}, true
	case Read:
		return verbStrings{scoped: "read", classic: types.VerbReadNoSecrets}, true
	case Secrets:
		return verbStrings{scoped: "secrets", classic: types.VerbRead}, true
	case Create:
		return verbStrings{scoped: "create", classic: types.VerbCreate}, true
	case Update:
		return verbStrings{scoped: "update", classic: types.VerbUpdate}, true
	case Delete:
		return verbStrings{scoped: "delete", classic: types.VerbDelete}, true
	default:
		return verbStrings{}, false
	}
}

// String returns the string used to express this verb in a scoped role spec. It is *not* the classic
// RBAC verb string, see [Verb.ClassicVerb].
func (v Verb) String() string {
	s, ok := v.strings()
	if !ok {
		return fmt.Sprintf("Verb(%d)", int(v))
	}
	return s.scoped
}

// ClassicVerb returns the classic RBAC verb string that this verb encodes to at check time. It
// exists for the scoped access checker; code performing access checks should pass verbs to the
// checker rather than encoding them itself.
func (v Verb) ClassicVerb() (string, error) {
	s, ok := v.strings()
	if !ok {
		return "", trace.BadParameter("invalid scoped verb %d (this is a bug)", int(v))
	}
	return s.classic, nil
}

// EncodeScopedVerbs encodes typed verbs into the verb strings used by scoped role specs.
func EncodeScopedVerbs(verbs ...Verb) []string {
	out := make([]string, 0, len(verbs))
	for _, v := range verbs {
		out = append(out, v.String())
	}
	return out
}

// verbByScopedString parses a scoped-role-spec verb string.
func verbByScopedString(scoped string) (Verb, bool) {
	for v := unspecifiedVerb + 1; v < lastVerb; v++ {
		if s, ok := v.strings(); ok && s.scoped == scoped {
			return v, true
		}
	}
	return unspecifiedVerb, false
}

// isAllowedScopedRule returns true if the given scoped spec verb is allowed for the given scoped resource kind. Scoped roles
// are not sane for use with all resource kind/verb combinations, so we restrict the allowed combinations here.
func isAllowedScopedRule(kind string, verb string) bool {
	switch kind {
	case KindScopedRole:
		// scoped roles can be read/written, and do not currently contain a concept of a secret.
		return isReadWriteNoSecrets(verb)
	case KindScopedRoleAssignment:
		// scoped role assignments can be read/written, and do not currently contain a concept of a secret.
		return isReadWriteNoSecrets(verb)
	case types.KindScopedToken:
		// scoped tokens can be read/written, and carry a join secret.
		return isReadWriteWithSecrets(verb)
	case types.KindBot:
		// bots can be read/written, and do not currently contain a concept of a secret.
		return isReadWriteNoSecrets(verb)
	case types.KindBotInstance:
		// bot instances can be read/written, and do not currently contain a concept of a secret.
		return isReadWriteNoSecrets(verb)
	case types.KindWorkloadIdentity:
		// workload identities can be read/written, and do not contain a concept of a secret.
		return isReadWriteNoSecrets(verb)
	case types.KindKubernetesCluster:
		// kube clusters can be read/written, and may carry a kubeconfig payload that grants access to the cluster.
		return isReadWriteWithSecrets(verb)
	case types.KindAppServer:
		// app servers can be read/written, and embed an app resource that carries no credential material.
		return isReadWriteNoSecrets(verb)
	case types.KindAccessList:
		// access lists can be read/written, and do not contain a concept of a secret.
		// permissions for access list members are conferred by permissions for
		// access list themselves, and/or list ownership.
		return isReadWriteNoSecrets(verb)
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
	return isReadWriteNoSecrets(verb) || verb == Secrets.String()
}

// isReadNoSecrets returns true if the given verb conforms to the "read-no-secrets" category of verbs.
func isReadNoSecrets(verb string) bool {
	switch verb {
	case List.String(), Read.String():
		return true
	default:
		return false
	}
}

// isWrite returns true if the given verb conforms to the "write" category of verbs.
func isWrite(verb string) bool {
	switch verb {
	case Create.String(), Update.String(), Delete.String():
		return true
	default:
		return false
	}
}

// compileScopedVerb returns the classic RBAC verb that the given scoped spec verb compiles to for the
// given kind, or false if the combination isn't allowed (and should therefore be dropped at compile time).
func compileScopedVerb(kind, scopedVerb string) (string, bool) {
	if !isAllowedScopedRule(kind, scopedVerb) {
		return "", false
	}
	v, ok := verbByScopedString(scopedVerb)
	if !ok {
		return "", false
	}
	classic, err := v.ClassicVerb()
	return classic, err == nil
}
