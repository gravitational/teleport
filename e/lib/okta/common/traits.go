package common

import (
	"slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type traitsMappingGetter interface {
	GetTraitMappings() types.TraitMappingSet
}

const (
	oktaUsernameTrait = "username"
	oktaGroupsTrait   = "groups"
)

// SetUserRolesAndTraits sets user roles and traits based on the groups and connector.
func SetUserRolesAndTraits(user types.User, groups []string, connector traitsMappingGetter) {
	mapping := connector.GetTraitMappings()

	foundUserMapping := false
	fondGroupMapping := false

	// Okta connector should have a mapping for username and groups.
	// Not standard traits are not supported.
	for _, v := range mapping {
		if v.Trait == oktaUsernameTrait {
			foundUserMapping = true
		}
		if v.Trait == oktaGroupsTrait {
			fondGroupMapping = true
		}
	}

	traits := user.GetTraits()

	if traits == nil {
		traits = make(map[string][]string)
	}
	if foundUserMapping {
		traits[oktaUsernameTrait] = []string{user.GetName()}
	}
	if fondGroupMapping {
		traits[oktaGroupsTrait] = groups
	}
	user.SetTraits(traits)

	roles := user.GetRoles()
	_, rolesFromTraits := services.TraitsToRoles(mapping, traits)
	roles = append(roles, rolesFromTraits...)
	slices.Sort(roles)
	roles = slices.Compact(roles)
	user.SetRoles(roles)
}
