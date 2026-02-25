package provisioning

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/common"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/accesslists"
)

func (p *provisioner) provisionAccessList(
	ctx context.Context,
	state *provisioningv1.PrincipalState,
) (*provisioningv1.PrincipalState, error) {
	log := p.log.With(principalStateAttr(state))
	log.DebugContext(ctx, "Provisioning access list")

	if err := p.onPrincipalProvisioning(ctx, state); errors.Is(err, ErrDoNotProvision) {
		log.InfoContext(ctx, "Group provisioning suppressed by event callback")
		return state, nil
	}

	acl, aclMembers, err := getAccessListWithMembers(ctx, state.Spec.PrincipalId, p.accessListSvc)
	if err != nil {
		return nil, trace.Wrap(err, "loading access list and members for provisioning")
	}
	log = log.With("title", acl.Spec.Title)

	groupMembers, err := p.filterValidMembers(ctx, acl, aclMembers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.DebugContext(ctx, "Access list loaded, provisioning",
		"member_count", len(groupMembers))

	// If we don't have an external ID for this Group yet, we need to check and
	// see if a group with the target display name exists downstream. If it does,
	// we will adopt it by recording its ID against the current access list. If
	// not we will create it from scratch
	if state.Status.ExternalId == "" {
		updatedState, err := p.adoptOrCreateDownstreamGroup(ctx, state, acl)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// from now on, use the updated state that has an External ID, rather
		// than the one supplied by the caller
		state = updatedState
	} else {
		// make sure the downstream group has the correct name, metadata, etc
		if err := p.updateDownstreamGroup(ctx, state, acl); err != nil {
			if trace.IsNotFound(err) {
				log.WarnContext(ctx, "Downstream group has been deleted or moved")
				return nil, &missingPrincipalError{state: state}
			}
			return nil, trace.Wrap(err, "updating downstream group metadata")
		}
	}

	if err := p.scimClient.ReplaceGroupMembers(ctx, state.GetStatus().ExternalId, groupMembers); err != nil {
		if trace.IsNotFound(err) {
			log.WarnContext(ctx, "Downstream group has been deleted or moved")
			return nil, &missingPrincipalError{state: state}
		}
		return nil, trace.Wrap(err, "updating downstream group members")
	}

	provisionedState, err := markStateAsProvisioned(ctx, p.stateSvc, state, p.clock.Now(), nil, acl.GetRevision())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return provisionedState, nil
}

func (p *provisioner) adoptOrCreateDownstreamGroup(
	ctx context.Context,
	state *provisioningv1.PrincipalState,
	acl *accesslist.AccessList,
) (*provisioningv1.PrincipalState, error) {
	log := p.log.With(principalStateAttr(state))

	grp, err := p.scimClient.GetGroupByDisplayName(ctx, acl.Spec.Title)
	switch {
	case err == nil:
		// There is a corresponding group in the downstream system (matched
		// by display name) and we will adopt them as the downstream equivalent
		// of our Teleport access list
		externalID := ExternalID(grp.ID)
		log.DebugContext(ctx, "Adopting downstream group", externalIDAttr(externalID))
		adoptedState, err := recordExternalID(ctx, p.stateSvc, state, externalID, nil)
		if err != nil {
			return nil, trace.Wrap(err, "recording external ID for user")
		}

		// Ensure the adopted downstream group has the right metadata, display
		// name an so on
		if err := p.updateDownstreamGroup(ctx, adoptedState, acl); err != nil {
			return nil, trace.Wrap(err, "updating adopted group")
		}
		return adoptedState, nil

	case trace.IsNotFound(err):
		// Not having a corresponding downstream group is perfectly legitimate,
		// let's create one that we can populate as a later part of the
		// update.
		log.DebugContext(ctx, "Creating downstream group")
		createdState, err := p.createDownstreamGroup(ctx, state, acl)
		if err != nil {
			return nil, trace.Wrap(err, "creating downstream group")
		}

		return createdState, nil

	default:
		// but any other error is fatal and we need to bail out
		log.ErrorContext(ctx, "existence check failed to execute", "error", err)
		return nil, trace.Wrap(err, "checking for downstream group existence")
	}
}

// filterValidMembers skips member from the provisioning list if it
//   - fails to get user account for the member and the member did not originated from AWS.
//   - fails to get member's external ID.
//   - the member fails to meet access list membership requirement.
func (p *provisioner) filterValidMembers(
	ctx context.Context,
	acl *accesslist.AccessList,
	aclMembers []*accesslist.AccessListMember,
) ([]*scimsdk.GroupMember, error) {
	log := p.log.With(
		slog.Group("access_list",
			slog.String("name", acl.GetName()),
			slog.String("title", acl.Spec.Title)))

	groupMembers := make([]*scimsdk.GroupMember, 0, len(aclMembers))
	for _, aclMember := range aclMembers {
		memberUserName := aclMember.Spec.Name
		memberStateId := GetIDForUserName(memberUserName)

		log := log.With(
			"member_username", memberUserName,
			"member_state_id", memberStateId)

		user, err := p.usersSvc.GetUser(ctx, memberUserName, false)
		if err != nil {
			// Any error should just skip the user and let the access list provisioning proceed.

			// Preserve non-existent user membership that we imported from AWS.
			// Such members are labeled with OriginAWSIdentityCenter and ExternalIDLabel.
			if aclMember.Origin() == common.OriginAWSIdentityCenter {
				extID, ok := aclMember.GetAllLabels()[ExternalIDLabel.String()]
				if !ok {
					log.WarnContext(ctx, "External ID not found for a user from AWS Identity Center. This is a bug.")
					continue
				}
				groupMembers = append(groupMembers, &scimsdk.GroupMember{
					ExternalID: extID,
					Type:       scimsdk.ResourceTypeUser,
				})
				continue
			}
			log.ErrorContext(ctx, "Error loading user. User group membership won't be provisioned.", "error", err)
			continue
		}

		extID, err := p.externalIDGetter.GetExternalID(ctx, memberStateId)
		if err != nil {
			log.ErrorContext(ctx, "Failed to get external ID. User group membership won't be provisioned.", "error", err)
			continue
		}

		if extID == "" {
			log.WarnContext(ctx, "External ID not found. User group membership won't be provisioned.")
			continue
		}

		// Assert that the user is not only a recorded member, but also
		// currently meets all the Access List membership requirements
		if _, err := accesslists.IsAccessListMember(
			ctx,
			user,
			acl,
			p.accessListSvc,
			p.locksSvc,
			p.clock,
		); err != nil {
			if trace.IsAccessDenied(err) {
				log.WarnContext(ctx, "User does not meet Access List requirements")
				continue
			}
			return nil, trace.Wrap(err)
		}

		groupMembers = append(groupMembers, &scimsdk.GroupMember{
			ExternalID: string(extID),
			Type:       scimsdk.ResourceTypeUser,
		})
	}

	return groupMembers, nil
}

// createDownstreamGroup creates an empty downstream group for the supplied
// AccessList
func (p *provisioner) createDownstreamGroup(
	ctx context.Context,
	state *provisioningv1.PrincipalState,
	acl *accesslist.AccessList,
) (*provisioningv1.PrincipalState, error) {
	log := p.log.With(principalStateAttr(state))
	log.DebugContext(ctx, "Preparing to create new group")

	newGroup, err := p.scimClient.CreateGroup(ctx, scimsdk.ToGroup(acl))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Teleport's External ID is the resource's Primary ID in the downstream
	// system.
	externalID := ExternalID(newGroup.ID)

	updatedState, err := recordExternalID(ctx, p.stateSvc, state, externalID, nil)
	if err != nil {
		return nil, trace.Wrap(err, "recording external ID")
	}
	return updatedState, nil
}

// updateDownstreamGroup makes sure the downstream group name (and whatever
// other group-level metadata we want to sync) matches the Teleport AccessList
func (p *provisioner) updateDownstreamGroup(ctx context.Context,
	state *provisioningv1.PrincipalState,
	acl *accesslist.AccessList,
) error {
	err := p.scimClient.ReplaceGroupName(ctx,
		scimsdk.ToGroup(acl, scimsdk.WithGroupID(state.GetStatus().GetExternalId())))
	if err != nil {
		return trace.Wrap(err, "updating downstream group")
	}
	return nil
}
