package principal

import (
	"context"
	"errors"

	"github.com/gravitational/trace"

	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	"github.com/gravitational/teleport/lib/services"
)

// ErrNoUpdateRequired is a sentinel value that can be returned by the update
// callback in `updatePrincipalAssignment()` to signal that the principal
// assignment requires no update.
var ErrNoUpdateRequired = errors.New("no update required")

// Update updates a principal assignment, honoring optimistic locking, and
// allowing transaction-like updates to the principal state.
//
// Update will attempt to modify the supplied PrincipalAssignment with the given `mutate`
// function, and write the modified record to the data service. If the write fails with a
// `CompareFailed` error, we assume that the record has changed underneath us while we
// were processing on it, so we
//   - reload the record from the backend,
//   - re-apply the record changes by calling `mutate` on the reloaded record
//   - try to save the record again
//
// The mutate function can return ErrNoUpdateRequired to signal that no chages to the
// record are necessary (for example, the changes you're trying to make have been
// negated a concurrent change), and Update() will return without error, returning the
// latest loaded PrincipalAssignment Record.
func Update(
	ctx context.Context,
	icSvc services.IdentityCenterPrincipalAssignments,
	principalAssignment *identitycenterv1.PrincipalAssignment,
	mutate func(*identitycenterv1.PrincipalAssignment) error,
) (*identitycenterv1.PrincipalAssignment, error) {
	// TODO: look into some sort of backoff algorithm
	const retrytLimit = 10
	for range retrytLimit {
		if err := mutate(principalAssignment); err != nil {
			if errors.Is(err, ErrNoUpdateRequired) {
				return principalAssignment, nil
			}
			return nil, trace.Wrap(err)
		}

		updated, err := icSvc.UpdatePrincipalAssignment(ctx, principalAssignment)
		if err == nil {
			return updated, nil
		}

		// if the error is caused by anything other than the record changing
		// underneath us...
		if !trace.IsCompareFailed(err) {
			return nil, trace.Wrap(err, "updating principal assignment")
		}

		// otherwise, if someone *HAS* changed the record while we were working
		// on it, so re-load the record and try again.
		principalAssignment, err = icSvc.GetPrincipalAssignment(ctx, GetID(principalAssignment))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return nil, trace.LimitExceeded("failed to update Principal ID after %d tries", retrytLimit)
}
