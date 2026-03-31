package identitycenter

import (
	"fmt"
	"strings"
)

// awsSyncError is an error returned by the AWS resource synchronizer. It
// contains a nested error tree that can be examined for the specific reasons
// the sync failed. Use awsSyncError to generate a human readable summary of the
// (possibly multiple) causes  of sync failure.
type awsSyncError struct {
	inner error
}

// newAWSSyncError wraps the supplied error in an awsSyncError.
func newAWSSyncError(err error) error {
	if err == nil {
		return nil
	}
	return &awsSyncError{inner: err}
}

// Unwrap implements error chaining for [awsSyncError].
func (err *awsSyncError) Unwrap() error {
	return err.inner
}

// Error implements the builtin [error] interface for [awsSyncError].
func (err *awsSyncError) Error() string {
	// We don't want to include the whole inner `Error()` string, as this can be
	// *huge* for Identity Center instances with many AWS Accounts with many
	// Permission Sets. Because we sync so often, ongoing errors including the full
	// error tree text in the Audit Log would just be spam.
	//
	// Instead, we interrogate the chained error tree to pull out the specific
	// failures in order to give specific, actionable advice for fixing them.

	var blockingRoles []string
	visitErrorTree(err, func(e error) {
		//nolint:errorlint // intentionally checking direct error type; nested errors will be discovered separately.
		if roleError, ok := e.(*roleNameCollisionError); ok {
			blockingRoles = append(blockingRoles, roleError.roleName)
		}
	})

	var errorText strings.Builder
	errorText.WriteString("Periodic AWS Resource synchronization failed.")

	if len(blockingRoles) > 0 {
		errorText.WriteString("\nThese roles are blocking the creation of Identity Center Account Assignment Roles. Please review and rename or delete them:\n")
		for _, roleName := range blockingRoles {
			fmt.Fprintf(&errorText, "\t%q\n", roleName)
		}
	}

	return errorText.String()
}

// roleNameCollisionError indicates that an Account Assignment role was blocked
// from being created because an existing role with the name is already present.
type roleNameCollisionError struct {
	roleName string
}

// Error implements the [error] interface for roleNameCollisionError
func (err *roleNameCollisionError) Error() string {
	return fmt.Sprintf("Role %q is blocking the creation of an Identity Center Account Assignment role", err.roleName)
}

// visitErrorTree walks the tree of nested errors, invoking a callback on each
// non-nil error in the tree. Order of error traversal is not guaranteed.
func visitErrorTree(root error, visitorFn func(error)) {
	// using a stack in order to try and re-use the same backing store of the
	// pending errors slice as we push and pop elements. This means we
	// traverse the tree in depth-first order, but that's an implementation
	// detail that shouldn't be relied on.
	stack := []error{root}
	for len(stack) > 0 {
		var err error
		err, stack = stack[len(stack)-1], stack[:len(stack)-1]

		if err == nil {
			continue
		}

		visitorFn(err)

		//nolint:errorlint // only interested in what interfaces this value itself implements, chained errors will be discovered later.
		switch unwrapper := err.(type) {
		case interface{ Unwrap() error }:
			if inner := unwrapper.Unwrap(); inner != nil {
				stack = append(stack, inner)
			}

		case interface{ Unwrap() []error }:
			stack = append(stack, unwrapper.Unwrap()...)
		}
	}
}
