package directory

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

var errUnsupportedUsername = &trace.BadParameterError{Message: "username not supported"}

func errUnsupportedUsers(users []string) error {
	names := utils.Deduplicate(users)
	return trace.BadParameter(`username contains unsupported character(s), it should only include alphanumerics, `+
		`hyphens, dots, and plus sign. Unsupported usernames: %s`, strings.Join(names, ", "))
}

const (
	entraIDSAMLClaimGroups  = "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups"
	entraIDSAMLClaimRoles   = "http://schemas.microsoft.com/ws/2008/06/identity/claims/roles"
	entraIDSAMLClaimName    = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"
	entraIDSAMLClaimEmail   = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
	entraIDSAMLGivenName    = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"
	entraIDSAMLSurname      = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname"
	defaultSchemasNamespace = "http://schemas.microsoft.com/identity/claims/"
	tenantIDClaim           = defaultSchemasNamespace + "tenantid"
	objectIdentifierClaim   = defaultSchemasNamespace + "objectidentifier"
	displayNameClaim        = defaultSchemasNamespace + "displayname"
)

func convertUser(in *models.User, tenantID string, ssoConnectorID string, usersMemberships groupMembershipMap, emitAsRoles bool) (types.User, error) {
	username, isExternal, err := processUsername(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	samAccountName := in.OnPremisesSAMAccountName
	upn := *in.UserPrincipalName
	out, err := types.NewUser(username)
	labels := map[string]string{
		types.EntraUniqueIDLabel:                                *in.ID,
		types.EntraTenantIDLabel:                                tenantID,
		types.EntraUPNLabel:                                     upn,
		types.TeleportInternalLabelPrefix + "entra-is-external": strconv.FormatBool(isExternal),
	}
	if samAccountName != nil {
		labels[types.EntraSAMAccountNameLabel] = *samAccountName
	}
	out.SetStaticLabels(labels)
	out.SetOrigin(types.OriginEntraID)
	// Explicitly set to UNSET for idempotency.
	out.SetPasswordState(types.PasswordState_PASSWORD_STATE_UNSET)

	out.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{
			Name: teleport.UserSystem,
		},
		Time: time.Now().UTC(),
		Connector: &types.ConnectorRef{
			ID:       ssoConnectorID,
			Type:     constants.SAML,
			Identity: upn,
		},
	})

	traits := map[string][]string{
		entraIDSAMLClaimName:  {username},
		tenantIDClaim:         {tenantID},
		objectIdentifierClaim: {*in.ID},
	}

	if in.DisplayName != nil {
		traits[displayNameClaim] = []string{*in.DisplayName}
	}

	if in.GivenName != nil {
		traits[entraIDSAMLGivenName] = []string{*in.GivenName}
	}

	if in.Surname != nil {
		traits[entraIDSAMLSurname] = []string{*in.Surname}
	}

	if in.Mail != nil {
		traits[entraIDSAMLClaimEmail] = []string{*in.Mail}
	}
	if groups := userGroupNames(*in.GetID(), usersMemberships); len(groups) > 0 {
		if emitAsRoles {
			traits[entraIDSAMLClaimRoles] = groups
		} else {
			traits[entraIDSAMLClaimGroups] = groups
		}
	}
	out.SetTraits(traits)

	return out, trace.Wrap(err)
}

// preserveUserMetadata copies any metadata that needs to be preserved across an
// update from src to dst.
func preserveUserMetadata(dst, src types.User) {
	dst.SetRevision(src.GetRevision())
	dst.SetCreatedBy(src.GetCreatedBy())
	dst.SetWeakestDevice(src.GetWeakestDevice())
}

type groupMembershipInfo struct {
	groupIds   []string
	groupNames []string
}

type groupMembershipMap map[string]groupMembershipInfo

func buildUserMemberships(groupsMap map[string]*models.Group, groupMembersMap map[string][]models.GroupMember, groupNameBuilder func(*models.Group) string) groupMembershipMap {
	unwindedGroupMemberships := unwindGroupMembership(groupsMap, groupMembersMap)
	result := map[string]groupMembershipInfo{}
	for groupID, members := range groupMembersMap {
		for _, member := range members {
			if user, ok := member.(*models.User); ok {
				var displayNames []string
				for _, membershipGroup := range unwindedGroupMemberships[groupID] {
					group, ok := groupsMap[membershipGroup]
					if !ok || group.DisplayName == nil {
						continue
					}
					displayNames = append(displayNames, groupNameBuilder(group))
				}
				result[*user.ID] = groupMembershipInfo{
					groupIds:   append(result[*user.ID].groupIds, unwindedGroupMemberships[groupID]...),
					groupNames: append(result[*user.ID].groupNames, displayNames...),
				}
			}
		}
	}

	for k, v := range result {
		result[k] = groupMembershipInfo{
			groupIds:   utils.Deduplicate(v.groupIds),
			groupNames: utils.Deduplicate(v.groupNames),
		}
	}
	return result
}

// matchByConnector matches with the Auth Connector of the SAML type.
// This should only be used as a fallback when user is not matched with
// the origin label.
func matchByConnector(ref *types.ConnectorRef, connectorID string) bool {
	if ref == nil {
		return false
	}
	return ref.IsSameProvider(&types.ConnectorRef{
		Type: constants.SAML,
		ID:   connectorID,
	})
}

func unameForLog(in *models.User) string {
	if in == nil {
		return ""
	}
	if in.Mail != nil {
		return fmt.Sprintf("(mail=%s)", *in.Mail)
	}
	if in.UserPrincipalName != nil {
		return fmt.Sprintf("(upn=%s)", *in.UserPrincipalName)
	}
	if in.GetID() != nil {
		return fmt.Sprintf("(id=%s)", *in.GetID())
	}
	return ""
}

type userConfig struct {
	tenantID       string
	ssoConnectorID string
	emitAsRoles    bool
	tms            types.TraitMappingSet
}

func userGroupNames(id string, membershipMap groupMembershipMap) []string {
	groups := slices.Clone(membershipMap[id].groupNames)
	sort.Strings(groups)
	return groups
}
