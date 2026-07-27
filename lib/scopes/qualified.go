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
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package scopes

import (
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

// QualifiedNameSeparator is the separator between scope and name in a
// scope-qualified name. This separator must never appear in scope segments.
const QualifiedNameSeparator = "::"

// QualifiedName pairs a scope with a resource name to uniquely identify a scoped
// resource. The canonical form of a scope-qualified name (SQN) is "<scope>::<name>",
// e.g. "/staging/west::myrole". SQNs take the place of bare names in configuration
// resource specs, CLI interfaces, and user-facing messages (e.g. errors) where it
// is necessary to fully specify the unique identifier of a scoped resource. Internally,
// teleport APIs should generally continue to use separate scope and name fields, as
// should structured logs/events.
//
// A QualifiedName may be used in APIs that need to refer to a resource that
// may be scoped or unscoped. In these cases the Scope field may be empty, and
// WeakValidate and StrongValidate will return an error.
type QualifiedName struct {
	// Scope is the resource's scope path, e.g. "/staging/west".
	Scope string
	// Name is the resource's name within its scope, e.g. "myrole".
	Name string
}

// String returns the string representation of the QualifiedName.
// If the Scope is empty, the Name is returned verbatim.
func (q QualifiedName) String() string {
	if q.Scope == "" {
		return q.Name
	}
	return q.Scope + QualifiedNameSeparator + q.Name
}

// Set sets a possible scope qualified name. If the "::" separator is not present,
// then the scope qualified name will have an empty scope. This implements
// the flag/kingping Value interface.
func (q *QualifiedName) Set(val string) error {
	if !strings.Contains(val, QualifiedNameSeparator) {
		*q = QualifiedName{Name: val}
		return nil
	}
	sqn, err := ParseQualifiedName(val)
	if err != nil {
		return err
	}
	if err := sqn.StrongValidate(); err != nil {
		return err
	}
	*q = sqn
	return nil
}

// StrongValidate validates this QualifiedName using strong validation rules. This method
// *must* be called on all QualifiedName values derived from user input and/or cluster-external
// sources. Use [QualifiedName.WeakValidate] when checking values from the control plane in
// logic that may run agent-side.
func (q QualifiedName) StrongValidate() error {
	if err := StrongValidate(q.Scope); err != nil {
		return trace.BadParameter("scope-qualified name %q has invalid scope: %v", q, err)
	}

	if err := strongValidateName(q.Name); err != nil {
		return trace.BadParameter("scope-qualified name %q has invalid name: %v", q, err)
	}

	// as an extra precaution, also run all weak checks just to be certain we didn't accidentally
	// construct a weak check that rejects something that would otherwise pass a strong check.
	if err := q.WeakValidate(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// WeakValidate performs a weak form of validation on this QualifiedName. This is useful for
// ensuring that values received from trusted sources (e.g. the control plane) haven't been
// altered beyond our ability to reason effectively about them. Prefer [QualifiedName.StrongValidate]
// for values derived from external sources (e.g. user input).
func (q QualifiedName) WeakValidate() error {
	if err := WeakValidate(q.Scope); err != nil {
		return trace.BadParameter("scope-qualified name %q has invalid scope: %v", q, err)
	}

	if err := WeakValidateSegment(q.Name); err != nil {
		return trace.BadParameter("scope-qualified name %q has invalid name: %v", q, err)
	}

	return nil
}

// nameRegexp is the regular expression used to validate scoped resource names. It enforces
// the same character rules as segmentRegexp, but allows for single character names.
var nameRegexp = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-\_\.]*[a-z0-9])?$`)

// strongValidateName checks if a scoped resource name is valid according to scope character
// formatting rules. Unlike scope segments, names carry no length constraints.
func strongValidateName(name string) error {
	if name == "" {
		return trace.BadParameter("name is empty")
	}

	return trace.Wrap(strongValidateFormat(name, nameRegexp))
}

// ParseQualifiedName parses a scope-qualified name string into its scope and name
// components by splitting on the first occurrence of "::". Returns an error if the
// separator is absent or either component is empty. This function does not validate
// the format of the scope or name components; use [QualifiedName.StrongValidate] or
// [QualifiedName.WeakValidate] for validation.
func ParseQualifiedName(sqn string) (QualifiedName, error) {
	idx := strings.Index(sqn, QualifiedNameSeparator)
	if idx < 0 {
		return QualifiedName{}, trace.BadParameter("scope-qualified name %q missing %q separator", sqn, QualifiedNameSeparator)
	}

	scope := sqn[:idx]
	name := sqn[idx+len(QualifiedNameSeparator):]

	if scope == "" {
		return QualifiedName{}, trace.BadParameter("scope-qualified name %q has empty scope component", sqn)
	}

	if name == "" {
		return QualifiedName{}, trace.BadParameter("scope-qualified name %q has empty name component", sqn)
	}

	return QualifiedName{Scope: scope, Name: name}, nil
}

// StrongValidateQualifiedName validates a scope-qualified name string using strong validation
// rules. This function *must* be called on all scope-qualified name values received from
// user input and/or cluster-external sources. Use [WeakValidateQualifiedName] when
// checking values from the control plane in logic that may run agent-side.
//
// Prefer parsing with [ParseQualifiedName] and then calling [QualifiedName.StrongValidate]
// directly when the parsed value is needed, to avoid parsing twice.
func StrongValidateQualifiedName(sqn string) error {
	qn, err := ParseQualifiedName(sqn)
	if err != nil {
		return trace.Wrap(err)
	}
	return qn.StrongValidate()
}

// WeakValidateQualifiedName performs a weak form of validation on a scope-qualified name string.
// This is useful for ensuring that values received from trusted sources (e.g. the control
// plane) haven't been altered beyond our ability to reason effectively about them. Prefer
// [StrongValidateQualifiedName] for values received from external sources (e.g. user input).
//
// Prefer parsing with [ParseQualifiedName] and then calling [QualifiedName.WeakValidate]
// directly when the parsed value is needed, to avoid parsing twice.
func WeakValidateQualifiedName(sqn string) error {
	qn, err := ParseQualifiedName(sqn)
	if err != nil {
		return trace.Wrap(err)
	}
	return qn.WeakValidate()
}
