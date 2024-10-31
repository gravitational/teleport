package provisioning

import (
	"context"

	"github.com/gravitational/trace"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
)

// This file defines functions for provisioning users to a downstream system.
// The provisioning process is roughly this (in Mermaid):
//
// stateDiagram
//     state Create {
//         state "Downstream User exists?" as check_user
//         state "Provision User" as provision
//         state "Record External ID" as record_external_id
//         [*] --> check_user
//         check_user --> record_external_id: yes
//         check_user --> provision: no
//         provision --> record_external_id
//         record_external_id --> [*]
//     }
//     state "Check External ID" as check_external_id
//     [*] --> check_external_id
//     check_external_id --> Update: ExternalID known
//     check_external_id --> Create: ExternalID unknown
//     Update --> [*]: Success
//     Update --> Create: Fail (404)
//     Update --> [*]: Fail (Other)
//     Create --> Update: User Requires Update
//     Create --> [*]: User Done

func (p *provisioner) provisionUser(
	ctx context.Context,
	state *provisioningv1.PrincipalState,
) (*provisioningv1.PrincipalState, error) {
	log := p.log.With("principal_state", principalStateValuer{state})
	log.DebugContext(ctx, "Provisioning user")

	user, err := p.usersSvc.GetUser(ctx, state.Spec.PrincipalId, false)
	if err != nil {
		return nil, trace.Wrap(err, "fetching user for provisioning")
	}

	// If we don't have an external ID recorded for this resource, we treat this
	// as new user creation. If a matching user exists on the downstream system
	// we will adopt it, rather than create a new user.
	if state.Status.ExternalId == "" {
		updatedState, err := p.adoptOrCreateDownstreamUser(ctx, state, user)
		if err != nil {
			return nil, trace.Wrap(err, "handling user with no external ID")
		}
		return updatedState, nil
	}

	// If we get to here, we have a downstream user with a known external ID
	// that needs to be updated.
	updatedState, err := p.updateDownstreamUser(ctx, state, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return updatedState, nil
}

func (p *provisioner) adoptOrCreateDownstreamUser(
	ctx context.Context,
	state *provisioningv1.PrincipalState,
	user types.User,
) (*provisioningv1.PrincipalState, error) {
	log := p.log.With(principalStateAttr(state))

	downstreamUser, err := p.scimClient.GetUserByUserName(ctx, user.GetName())
	switch {
	case err == nil:
		// There is a corresponding user in the downstream system (matched by
		// name) and we will adopt them as the downstream avatar of our Teleport
		// user
		externalID := ExternalID(downstreamUser.ID)
		updatedState, err := recordExternalID(ctx, p.stateSvc, state, externalID, nil)
		if err != nil {
			return nil, trace.Wrap(err, "recording external ID for user")
		}
		p.onExternalIDUpdated(ctx, updatedState)

		provisionedState, err := p.updateDownstreamUser(ctx, updatedState, user)
		if err != nil {
			return nil, trace.Wrap(err, "updating adopted downstream user")
		}
		return provisionedState, nil

	case trace.IsNotFound(err):
		// Not having a corresponding downstream user is perfectly legitimate -
		// we just need to create them.
		updatedState, err := p.createDownstreamUser(ctx, state, user)
		if err != nil {
			return nil, trace.Wrap(err, "creating downstream user")
		}
		return updatedState, nil

	default:
		// any other error is fatal
		log.ErrorContext(ctx, "existence check failed to execute", "error", err)
		return nil, trace.Wrap(err, "checking for downstream user existence")
	}
}

func (p *provisioner) createDownstreamUser(
	ctx context.Context,
	state *provisioningv1.PrincipalState,
	user types.User,
) (*provisioningv1.PrincipalState, error) {
	activeState, locks, err := p.validateUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createdUser, err := p.scimClient.CreateUser(ctx,
		scimsdk.ToUser(user, scimsdk.WithActiveState(activeState)))
	if err != nil {
		return nil, trace.Wrap(err, "sending provisioning request")
	}

	// The Teleport external ID for this user is the downstream system's primary
	// ID.
	externalID := ExternalID(createdUser.ID)

	updatedState, err := recordExternalID(ctx, p.stateSvc, state, externalID, locks)
	if err != nil {
		return nil, trace.Wrap(err, "recording external ID")
	}
	p.onExternalIDUpdated(ctx, updatedState)

	return updatedState, nil
}

// validateUser checks that the user is in good standing for provisioning to the
// downstream system. Returns an error if the user must not be provisioned
// downstream. A user who is locked but otherwise fine will return no error, but
// the returned `userActiveState` will be `userInactive`.
func (p *provisioner) validateUser(ctx context.Context, user types.User) (scimsdk.UserActiveState, []string, error) {
	locks, err := p.locksSvc.GetLocks(
		ctx,
		true, /* i.e. only get in-force locks */
		types.LockTarget{User: user.GetName()})
	if err != nil {
		return scimsdk.UserInactive, nil, trace.Wrap(err, "validating user")
	}

	if len(locks) > 0 {
		lockIDs := make([]string, len(locks))
		for i, lock := range locks {
			lockIDs[i] = lock.GetName()
		}
		return scimsdk.UserInactive, lockIDs, nil
	}

	return scimsdk.UserActive, nil, nil
}

func (p *provisioner) updateDownstreamUser(
	ctx context.Context,
	state *provisioningv1.PrincipalState,
	user types.User,
) (*provisioningv1.PrincipalState, error) {
	log := p.log.With(principalStateAttr(state))

	if state.GetStatus().GetExternalId() == "" {
		return nil, trace.BadParameter("principal state must have an ExternalId")
	}

	activeState, locks, err := p.validateUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err, "validating user")
	}

	updatedUser, err := p.scimClient.UpdateUser(ctx, scimsdk.ToUser(user,
		scimsdk.WithUserID(state.GetStatus().GetExternalId()),
		scimsdk.WithActiveState(activeState)))
	if err != nil {
		return nil, trace.Wrap(err, "updating downstream user")
	}

	if updatedUser.ID != state.GetStatus().GetExternalId() {
		state, err = recordExternalID(ctx, p.stateSvc, state, ExternalID(updatedUser.ID), nil)
		if err != nil {
			return nil, trace.Wrap(err, "updating downstream user")
		}
		p.onExternalIDUpdated(ctx, state)
	}

	updatedState, err := markStateAsProvisioned(ctx, p.stateSvc, state, p.clock.Now(), locks, user.GetRevision(), log)
	if err != nil {
		return nil, trace.Wrap(err, "marking principal as provisioned")
	}

	return updatedState, nil
}
