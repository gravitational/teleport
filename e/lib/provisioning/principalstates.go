package provisioning

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/services"
)

func getID(s *provisioningv1.PrincipalState) services.ProvisioningStateID {
	return services.ProvisioningStateID(s.Metadata.Name)
}

func getDownstreamID(s *provisioningv1.PrincipalState) services.DownstreamID {
	return services.DownstreamID(s.GetSpec().DownstreamId)
}

func getIDForPrincipal(principalName string, principalType provisioningv1.PrincipalType) (services.ProvisioningStateID, error) {
	switch principalType {
	case provisioningv1.PrincipalType_PRINCIPAL_TYPE_USER:
		return getIDForUserName(principalName), nil

	case provisioningv1.PrincipalType_PRINCIPAL_TYPE_ACCESS_LIST:
		return getIDForAccessListName(principalName), nil

	default:
		return "", trace.BadParameter("invalid principal type: %v", principalType)
	}
}

func getIDForUser(u types.User) services.ProvisioningStateID {
	return getIDForUserName(u.GetName())
}

func getIDForUserName(name string) services.ProvisioningStateID {
	return services.ProvisioningStateID("u-" + name)
}

func getIDForAccessList(acl *accesslist.AccessList) services.ProvisioningStateID {
	return getIDForAccessListName(acl.GetName())
}

func getIDForAccessListName(name string) services.ProvisioningStateID {
	return services.ProvisioningStateID("acl-" + name)
}

func newPrincipalState(
	downstreamID services.DownstreamID,
	principalType provisioningv1.PrincipalType,
	id services.ProvisioningStateID,
	principalName string,
	provisioningState provisioningv1.ProvisioningState,
) *provisioningv1.PrincipalState {
	return &provisioningv1.PrincipalState{
		Kind:    types.KindProvisioningPrincipalState,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: string(id),
		},
		Spec: &provisioningv1.PrincipalStateSpec{
			DownstreamId:  string(downstreamID),
			PrincipalType: principalType,
			PrincipalId:   principalName,
		},
		Status: &provisioningv1.PrincipalStateStatus{
			ProvisioningState: provisioningState,
		},
	}
}

// recordExternalID sets the PrincipalState state's External ID and updates the
// backend record. Returns the updated state.
func recordExternalID(
	ctx context.Context,
	statesSvc services.DownstreamProvisioningStates,
	state *provisioningv1.PrincipalState,
	extID ExternalID,
	locks []string,
) (*provisioningv1.PrincipalState, error) {
	mutate := func(s *provisioningv1.PrincipalState) error {
		s.Status.ExternalId = string(extID)
		s.Status.ActiveLocks = locks
		return nil
	}
	updatedState, err := updateProvisioningState(ctx, statesSvc, state, mutate)
	if err != nil {
		return nil, trace.Wrap(err, "marking principal as provisioned")
	}
	return updatedState, nil
}

// markStateInError records a provisioning failure in the Principal's state
// record.
func markStateInError(
	ctx context.Context,
	statesSvc services.DownstreamProvisioningStates,
	state *provisioningv1.PrincipalState,
	provisioningError error,
	log *slog.Logger,
) (*provisioningv1.PrincipalState, error) {
	recordError := func(s *provisioningv1.PrincipalState) error {
		s.Status.Error = provisioningError.Error()
		return nil
	}
	updatedState, err := updateProvisioningState(ctx, statesSvc, state, recordError)
	if err != nil {
		return nil, trace.Wrap(err, "marking principal as provisioned")
	}
	return updatedState, nil
}

// markStateAsProvisioned records a PrincipalState as having been provisioned.
//
// If the state record was modified during provisioning, this function will *not*
// update the record's ProvisioningState. This is in case the STALE state
// represents a change to the principal that happened while we were busy with
// the state record, and thus may still need to be provisioned in a later update.
//
// The "last provisioned" time and error state will be updated regardless, as
// this should always be new information.
func markStateAsProvisioned(
	ctx context.Context,
	statesSvc services.DownstreamProvisioningStates,
	state *provisioningv1.PrincipalState,
	timestamp time.Time,
	locks []string,
	principalRevision string,
	log *slog.Logger,
) (*provisioningv1.PrincipalState, error) {
	// Note that we only try and set this for the first update pass. We don't
	// want clobber a newer "STALE" state if the record has been touched while
	// we've been busy with it. Because this isn't in the mutator, it won't get
	// updated in the backend on a re-tried update.
	status := state.Status
	if status == nil {
		return nil, trace.BadParameter("provisioning state missing status block")
	}

	// Do NOT clobber a DELETED state; the record needs to go through the the
	// full downstream de-provisioning cycle before it can be re-created.
	if status.ProvisioningState != provisioningv1.ProvisioningState_PROVISIONING_STATE_DELETED {
		status.ProvisioningState = provisioningv1.ProvisioningState_PROVISIONING_STATE_PROVISIONED
	}

	timestampPB := timestamppb.New(timestamp)
	markProvisioned := func(s *provisioningv1.PrincipalState) error {
		status := s.Status
		if status == nil {
			return trace.BadParameter("provisioning state missing status block")
		}

		// Do NOT clobber the provisioning state. An updated record with STALE
		// implies tht the record has changed underneath us while we were
		// processing it, so the principal may already need to be re-provisioned
		// downstream. An updated record with DELETED needs to go through the
		// whole downstream de-provisioning cycle before it can be re-created.

		status.ActiveLocks = locks
		status.LastProvisioned = timestampPB
		status.ProvisionedPrincipalRevision = principalRevision
		status.Error = ""
		return nil
	}

	updatedState, err := updateProvisioningState(ctx, statesSvc, state, markProvisioned)
	if err != nil {
		return nil, trace.Wrap(err, "marking principal as provisioned")
	}
	return updatedState, nil
}

// errNoChangeRequired is a sentinel error used by the mutator functions in
// updateProvisioningState to signal that no update is required and the update
// should be abandoned
var errNoChangeRequired = errors.New("no change required. Update aborted.")

// updateProvisioningState updates a Provisioning Principal State Record while
// honoring record locking. updateProvisioningState will apply the provided
// mutator to the supplied state and attempt to write the update to the
// back-end.
//
// The mutation function can return `errNoChangeRequired` to signal that the
// update should be abandoned without error. The mutation function MUST NOT
// actually change the in-memory state record when returning `errNoChangeRequired`,
// or callers will receive a corrupted state value.
//
// Any other errors will be treated as fatal and propagated back up the call stack.
//
// If the update fails with a comparison error (i.e. the record was updated
// behind the caller's back, and the supplied state is out of date), this function
// will load a fresh copy of the state object and re-try the update.
//
// Be careful not to clobber any data that should be preserved from an updated
// record in the mutate function.
func updateProvisioningState(
	ctx context.Context,
	statesSvc services.DownstreamProvisioningStates,
	state *provisioningv1.PrincipalState,
	mutateState func(*provisioningv1.PrincipalState) error,
) (*provisioningv1.PrincipalState, error) {
	// TODO(tcsc): Investigate some sort of backoff algorithm for this
	for range defaultUpdateAttempts {

		if err := mutateState(state); err != nil {
			if errors.Is(err, errNoChangeRequired) {
				return state, nil
			}
			return nil, trace.Wrap(err, "mutating provisioning state")
		}

		updated, err := statesSvc.UpdateProvisioningState(ctx, state)
		switch {
		case err == nil:
			return updated, nil

		case trace.IsCompareFailed(err):
			state, err = statesSvc.GetProvisioningState(ctx, getDownstreamID(state), getID(state))
			if err != nil {
				return nil, trace.Wrap(err, "loading state record for update")
			}
			continue

		default:
			return nil, trace.Wrap(err, "marking state as provisioned")
		}
	}
	return nil, trace.LimitExceeded("retry attempts exceeded")
}
