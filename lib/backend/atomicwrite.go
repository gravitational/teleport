package backend

import (
	"bytes"
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"
)

// ErrConditionFailed is returned from AtomicWrite when one or more conditions failed to hold.
var ErrConditionFailed = &trace.CompareFailedError{Message: "condition failed, one or more resources were concurrently created|modified|deleted; please work from the latest state"}

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
	// Kind is the kind of condition represented.
	Kind ConditionKind

	// Revision is a specific revision to be asserted (only used when Kind is KindRevision).
	Revision string
}

// IsZero checks if this condition appears unspecified.
func (c *Condition) IsZero() bool {
	return c.Kind == 0
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
	// Kind is the kind of action represented.
	Kind ActionKind

	// Item is the item to be written (only used when Kind is Put).
	Item Item
}

// IsZero checks if this action appears unspecified.
func (a *Action) IsZero() bool {
	return a.Kind == 0
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
	Key []byte

	// Condition must be one of Exists|NotExists|Revision(<revision>)|Whatever
	Condition Condition

	// Action must be one of Put(<item>)|Delete|Nop
	Action Action
}

// Check validates the basic correctness of the conditional action.
func (c *ConditionalAction) Check() error {
	if len(c.Key) == 0 {
		return trace.BadParameter("conditional action missing required parameter 'key'")
	}

	if c.Condition.IsZero() {
		return trace.BadParameter("conditional action for %q missing required parameter 'condition'", c.Key)
	}

	if c.Action.IsZero() {
		return trace.BadParameter("conditional action for %q missing required parameter 'action'", c.Key)
	}

	switch c.Condition.Kind {
	case KindWhatever, KindExists, KindNotExists, KindRevision:
	default:
		return trace.BadParameter("conditional action for %q contains unexpected condition kind %v", c.Key, c.Condition.Kind)
	}

	switch c.Action.Kind {
	case KindNop, KindDelete:
	case KindPut:
		if len(c.Action.Item.Value) == 0 {
			return trace.BadParameter("conditional action for %q missing required put parameter Item.Value", c.Key)
		}
	default:
		return trace.BadParameter("conditional action for %q contains unexpected action kind %v", c.Key, c.Action.Kind)
	}

	return nil
}

const (
	// MaxAtomicWriteSize is the maximum number of conditional actions that may
	// be applied via a single atomic write. The exact number is subject to change
	// but must always be less than the minimum value supported across all backends.
	MaxAtomicWriteSize = 64
)

// AtomicWrite is a standalone interface for the AtomicWrite method. This interface will be deprecated
// once all backends implement AtomicWrite.
type AtomicWrite interface {

	// AtomicWrite executes a batch of conditional actions atomically s.t. all actions happen if all
	// conditions are met, but no actions happen if any condition fails to hold. If one or more conditions
	// failed to hold, [ErrConditionFailed] is returned. The number of conditional actions must not
	// exceed [MaxAtomicWriteSize] and no two conditional actions may point to the same key. If successful,
	// the returned revision is the new revision associated with all [Put] actions that were part of the
	// operation (the revision value has no meaning outside of the context of puts).
	AtomicWrite(ctx context.Context, condacts ...ConditionalAction) (revision string, err error)
}

// AtomicWriteBackend joins the AtomicWrite interface with the standard backend interface. This interface
// will be deprecated once all backends implement AtomicWrite.
type AtomicWriteBackend interface {
	Backend
	AtomicWrite
}

// ValidateAtomicWrite verifies that the supplied group of conditional actions are a valid input for atomic
// application. This means both verifying that each individual conditional action is well-formed, and also
// that no two conditional actions targets the same key.
func ValidateAtomicWrite(condacts []ConditionalAction) error {
	if len(condacts) > MaxAtomicWriteSize {
		return trace.BadParameter("too many conditional actions for atomic application (len=%d, max=%d)", len(condacts), MaxAtomicWriteSize)
	}

	keys := make([][]byte, 0, len(condacts))

	for i := range condacts {
		if err := condacts[i].Check(); err != nil {
			return trace.Wrap(err)
		}

		keys = append(keys, condacts[i].Key)
	}

	slices.SortFunc(keys, bytes.Compare)

	var prev []byte
	for _, key := range keys {
		if bytes.Equal(prev, key) {
			return trace.BadParameter("multiple conditional actions target key %q", key)
		}
		prev = key
	}

	return nil
}
