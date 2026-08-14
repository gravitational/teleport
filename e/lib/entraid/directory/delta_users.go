package directory

import (
	"errors"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/msgraph/models"
	"github.com/gravitational/teleport/lib/services"
)

type userDeltaProcessor struct {
	// Teleport users map.
	entraUsersMap map[entraUniqueID]types.User
	// Entra ID group membership map.
	entraUserGroupMemberships groupMembershipMap
	userConfig                userConfig
}

func newUserDeltaProcessor(
	userMemberships groupMembershipMap,
	teleportUsersMap map[string]types.User,
	userConfig userConfig,
) *userDeltaProcessor {

	processor := &userDeltaProcessor{
		entraUserGroupMemberships: userMemberships,
		entraUsersMap:             make(map[entraUniqueID]types.User, len(teleportUsersMap)),
		userConfig:                userConfig,
	}

	for _, user := range teleportUsersMap {
		id, ok := user.GetLabel(types.EntraUniqueIDLabel)
		if !ok || id == "" {
			continue
		}
		processor.entraUsersMap[entraUniqueID(id)] = processor.user(
			entraUniqueID(id),
			user,
		)
	}

	return processor
}

// user returns a copy of teleport user so it
// becomes the base of users collection for delta sync.
// It may update user's traits and roles based on newly
// discovered [entraUserGroupMemberships].
func (u *userDeltaProcessor) user(id entraUniqueID, user types.User) types.User {
	out := user.Clone()
	newTraits := out.GetTraits()
	if newTraits == nil {
		// Entra imported users are supposed to have
		// entra specific traits added.
		newTraits = make(map[string][]string)
	}
	delete(newTraits, entraIDSAMLClaimRoles)
	delete(newTraits, entraIDSAMLClaimGroups)
	if groups := userGroupNames(string(id), u.entraUserGroupMemberships); len(groups) > 0 {
		if u.userConfig.emitAsRoles {
			newTraits[entraIDSAMLClaimRoles] = groups
		} else {
			newTraits[entraIDSAMLClaimGroups] = groups
		}
	}

	out.SetTraits(newTraits)
	_ /* warnings */, roles := services.TraitsToRoles(u.userConfig.tms, newTraits)
	out.SetRoles(roles)
	return out
}

// apply processes new, updated or deleted user deltas
// and applies the changes to the user base created
// from Entra ID users.
func (u *userDeltaProcessor) apply(in *models.ListUsersDeltaResponse) error {
	if in == nil || in.GetID() == nil {
		return nil
	}

	userID := entraUniqueID(*in.GetID())

	if isRemoved(in.Removed) {
		delete(u.entraUsersMap, userID)
		return nil
	}
	// New or updated user.
	user, err := convertUser(
		in.User,
		u.entraUserGroupMemberships,
		u.userConfig,
	)
	if err != nil {
		if errors.Is(err, errUnsupportedUsername) {
			// Delete if the updated account properties makes the account unsupported.
			// E.g. username updated to contain single quote which is not supported.
			delete(u.entraUsersMap, userID)
		}
		return trace.Wrap(err)
	}

	u.entraUsersMap[userID] = user
	return nil
}

// result returns the final state of the Entra ID users
// after applying delta changes.
func (u *userDeltaProcessor) result() map[string]types.User {
	newEntraUsers := make(map[string]types.User, len(u.entraUsersMap))
	for _, user := range u.entraUsersMap {
		newEntraUsers[user.GetName()] = user
	}

	return newEntraUsers
}
