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

package backend

import (
	"github.com/gravitational/trace"
)

// ErrConditionFailed is returned from AtomicWrite when one or more conditions failed to hold.
var ErrConditionFailed = &trace.CompareFailedError{Message: "condition failed, one or more resources were concurrently created|modified|deleted; please reload the current state and try again"}

// ConditionKind marks the kind of condition to be evaluated.
type ConditionKind int

const (
	// KindWhatever indicates that no condition should be evaluated.
	KindWhatever ConditionKind = 1 + iota

	// KindExists asserts that an item exists at the target key.
	KindExists

	// KindNotExists asserts that no item exists at the target key.
	KindNotExists

	// KindRevision asserts the exact current revision of the target key.
	KindRevision
)

// Condition specifies some requirement that a backend item must meet.
type Condition struct {
	// Revision is a specific revision to be asserted (only used when Kind is KindRevision).
	Revision string

	// Kind is the kind of condition represented.
	Kind ConditionKind
}

// IsZero checks if this condition appears unspecified.
func (c *Condition) IsZero() bool {
	return c.Kind == 0
}

// Check verifies that a the condition in well-formed.
func (c *Condition) Check() error {
	switch c.Kind {
	case KindWhatever, KindExists, KindNotExists, KindRevision:
	default:
		return trace.BadParameter("unexpected condition kind %v", c.Kind)
	}

	return nil
}

// Whatever builds a condition that matches any current key state.
func Whatever() Condition {
	return Condition{
		Kind: KindWhatever,
	}
}

// Exists builds a condition that asserts the target key exists.
func Exists() Condition {
	return Condition{
		Kind: KindExists,
	}
}

// NotExists builds a condition that asserts the target key does not exist.
func NotExists() Condition {
	return Condition{
		Kind: KindNotExists,
	}
}

// Revision builds a condition that asserts the target key has the specified revision.
func Revision(r string) Condition {
	return Condition{
		Kind:     KindRevision,
		Revision: r,
	}
}

// ActionKind marks the kind of an action to be taken.
type ActionKind int

const (
	// KindNop indicates that no action should be taken.
	KindNop ActionKind = 1 + iota

	// KindPut indicates that the associated item should be written to the target key.
	KindPut

	// KindDelete indicates that any item at the target key should be removed.
	KindDelete
)

// Action specifies an action to be taken against a backend item.
type Action struct {
	// Item is the item to be written (only used when Kind is Put).
	Item Item

	// Kind is the kind of action represented.
	Kind ActionKind
}

// IsZero checks if this action appears unspecified.
func (a *Action) IsZero() bool {
	return a.Kind == 0
}

// Check verifies that the action is well-formed.
func (a *Action) Check() error {
	switch a.Kind {
	case KindNop, KindDelete:
	case KindPut:
		if len(a.Item.Value) == 0 {
			return trace.BadParameter("missing required put parameter Item.Value")
		}
	default:
		return trace.BadParameter("unexpected action kind %v", a.Kind)
	}

	return nil
}

// IsWrite checks if this action performs a write.
func (a *Action) IsWrite() bool {
	switch a.Kind {
	case KindPut, KindDelete:
		return true
	default:
		return false
	}
}

// Nop builds an action that does nothing.
func Nop() Action {
	return Action{
		Kind: KindNop,
	}
}

// Put builds an action that writes the provided item to the target key.
func Put(item Item) Action {
	return Action{
		Kind: KindPut,
		Item: item,
	}
}

// Delete builds an action that removes the target key.
func Delete() Action {
	return Action{
		Kind: KindDelete,
	}
}

// ConditionalAction specifies a condition and an action associated with a given key. The condition
// must hold for the action to be taken.
type ConditionalAction struct {
	// Key is the key against which the associated condition and action are to
	// be applied.
	Key Key

	// Condition must be one of Exists|NotExists|Revision(<revision>)|Whatever
	Condition Condition

	// Action must be one of Put(<item>)|Delete|Nop
	Action Action
}

// Check validates the basic correctness of the conditional action.
func (c *ConditionalAction) Check() error {
	if len(c.Key.s) == 0 {
		return trace.BadParameter("conditional action missing required parameter 'Key'")
	}

	if c.Condition.IsZero() {
		return trace.BadParameter("conditional action for %q missing required parameter 'Condition'", c.Key)
	}

	if c.Action.IsZero() {
		return trace.BadParameter("conditional action for %q missing required parameter 'Action'", c.Key)
	}

	if err := c.Condition.Check(); err != nil {
		return trace.BadParameter("conditional action for key %q contains malformed condition: %v", c.Key, err)
	}

	if err := c.Action.Check(); err != nil {
		return trace.BadParameter("conditional action for key %q contains malformed action: %v", c.Key, err)
	}

	if c.Condition.Kind == KindWhatever && c.Action.Kind == KindNop {
		return trace.BadParameter("conditional action for %q is ineffectual (Condition=Whatever,Action=Nop)", c.Key)
	}

	return nil
}

const (
	// MaxAtomicWriteSize is the maximum number of conditional actions that may
	// be applied via a single atomic write. The exact number is subject to change
	// but must always be less than the minimum value supported across all backends.
	MaxAtomicWriteSize = 64
)

// AtomicWriterBackend was used to extend the backend interface with the AtomicWrite method prior to all backends
// implementing AtomicWrite. This alias can be safely deleted once it is no longer referenced by enterprise backend logic.
type AtomicWriterBackend = Backend

// ValidateAtomicWrite verifies that the supplied group of conditional actions are a valid input for atomic
// application. This means both verifying that each individual conditional action is well-formed, and also
// that no two conditional actions targets the same key.
func ValidateAtomicWrite(condacts []ConditionalAction) error {
	if len(condacts) > MaxAtomicWriteSize {
		return trace.BadParameter("too many conditional actions for atomic application (len=%d, max=%d)", len(condacts), MaxAtomicWriteSize)
	}

	if len(condacts) == 0 {
		return trace.BadParameter("empty conditional action list")
	}

	keys := make(map[string]struct{}, len(condacts))

	var containsWrite bool

	for i := range condacts {
		if err := condacts[i].Check(); err != nil {
			return trace.Wrap(err)
		}

		containsWrite = containsWrite || condacts[i].Action.IsWrite()

		key := condacts[i].Key.String()

		if _, ok := keys[key]; ok {
			return trace.BadParameter("multiple conditional actions target key %q", key)
		}

		keys[key] = struct{}{}
	}

	if !containsWrite {
		return trace.BadParameter("no conditional actions contain writes")
	}

	return nil
}
