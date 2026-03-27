package entraid

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/services"
)

type entraUniqueID string

var errUnsupportedUsername = &trace.BadParameterError{Message: "username not supported"}

func (r *DirectoryReconciler) reconcileUsers(ctx context.Context,
	groupsMap map[string]*msgraph.Group,
	groupMembersMap map[string][]msgraph.GroupMember,
) (map[entraUniqueID]types.User, error) {
	app, err := r.getApplication(ctx, r.entraAppID)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get Entra ID application")
	}
	emitAsRoles, groupNameBuilder := getGroupNameBuilderFunc(app)

	userMemberships := buildUserMemberships(groupsMap, groupMembersMap, groupNameBuilder)
	entraUsers, err := r.listEntraUsers(ctx, userMemberships, emitAsRoles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r.metrics.discoveredEntraUsers.Set(float64(len(entraUsers)))

	connector, err := r.samlService.GetSAMLConnector(ctx, r.ssoConnectorID, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get SAML connector")
	}

	for _, entraUser := range entraUsers {
		_, roles := services.TraitsToRoles(connector.GetTraitMappings(), entraUser.GetTraits())
		entraUser.SetRoles(roles)
	}

	teleportUsers, err := listTeleportUsers(ctx, r.userSvc, r.ssoConnectorID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, src := range teleportUsers {
		if dst, ok := entraUsers[src.GetName()]; ok {
			preserveUserMetadata(dst, src)
		}
	}

	usersByEntraID := map[entraUniqueID]types.User{}
	for _, u := range entraUsers {
		id, ok := u.GetLabel(types.EntraUniqueIDLabel)
		if !ok {
			return nil, trace.BadParameter("user %v missing Entra ID unique ID label", u.GetName())
		}
		usersByEntraID[entraUniqueID(id)] = u
	}
	var conflictingUsers []string
	backend, err := services.NewReconciler(services.ReconcilerConfig[types.User]{
		Matcher:             matchByLabel[types.User],
		CompareResources:    func(u1, u2 types.User) int { return services.EqualFromBool(u1.IsEqual(u2)) },
		GetCurrentResources: func() map[string]types.User { return teleportUsers },
		GetNewResources:     func() map[string]types.User { return entraUsers },
		OnCreate: func(ctx context.Context, u types.User) error {
			_, err := r.userSvc.CreateUser(ctx, u)
			// if Entra user clashes with a local user, do not overwrite
			if trace.IsAlreadyExists(err) {
				conflictingUsers = append(conflictingUsers, u.GetName())

				// Delete from the lookup map, since the Teleport user by this name is not an Entra user.
				delete(usersByEntraID, entraUniqueID(u.GetMetadata().Labels[types.EntraUniqueIDLabel]))

				return nil
			}
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming types.User, existing types.User) error {
			_, err := r.userSvc.UpdateUser(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, u types.User) error {
			return trace.Wrap(r.userSvc.DeleteUser(ctx, u.GetName()))
		},
		Metrics:            r.metrics.userReconcilerMetrics,
		AllowOriginChanges: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := backend.Reconcile(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	r.importedUsers = len(usersByEntraID)

	var errConflict error
	if len(conflictingUsers) != 0 {
		errConflict = trace.AlreadyExists(`existing user account found which was not created by this Microsoft Entra ID `+
			`integration, account and group sync will be skipped for these user(s): %s`, strings.Join(conflictingUsers, ", "))
		r.errSkippedResources.users = append(r.errSkippedResources.users, errConflict)
	}

	return usersByEntraID, nil
}

func listTeleportUsers(ctx context.Context, svc userAccessPoint, connectorID string) (map[string]types.User, error) {
	result := map[string]types.User{}

	var pageToken string
	for {
		resp, err := svc.ListUsers(ctx, &userspb.ListUsersRequest{PageToken: pageToken})
		if err != nil {
			return nil, trace.Wrap(err, "listing teleport entra users")
		}

		for _, user := range resp.Users {
			if matchByLabel(user) {
				result[user.GetName()] = user
				continue
			}

			// Fallback to match by connector since it's possible for a
			// user to log in to Teleport before their account is created by the
			// plugin. In such a case, user account gets created by the SAML
			// connector without the origin label assigned to the user resource.
			// Note: All users created by the integration also have the "CreatedBy"
			// field and will be matched with the [matchByConnector]. This arguably
			// makes the origin checker redundant. But the origin checker gets
			// precedence for now until its decided if we should consolidate to use
			// connector matcher as default and the only supported matcher.
			if matchByConnector(user.GetCreatedBy().Connector, connectorID) {
				slog.InfoContext(ctx, "User account found to be created by the referenced connector, overwriting", "user", user.GetName())
				result[user.GetName()] = user
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return result, nil
}

func (r *DirectoryReconciler) listEntraUsers(ctx context.Context, usersMemberships groupMembershipMap, emitAsRoles bool) (map[string]types.User, error) {
	result := map[string]types.User{}
	var unsupportedUsers []string
	err := r.graphClient.IterateUsers(ctx, func(u *msgraph.User) bool {
		user, err := convertUser(u, r.tenantID, r.ssoConnectorID, usersMemberships, emitAsRoles)
		if err != nil {
			if errors.Is(err, errUnsupportedUsername) {
				unsupportedUsers = append(unsupportedUsers, unameForLog(u))
			} else {
				r.errSkippedResources.users = append(r.errSkippedResources.users, trace.Wrap(err))
			}
			return true
		}
		result[user.GetName()] = user
		return true
	})

	if len(unsupportedUsers) > 0 {
		names := utils.Deduplicate(unsupportedUsers)
		r.errSkippedResources.users = append(r.errSkippedResources.users,
			trace.BadParameter(`username contains unsupported character(s), it should only include alphanumerics, `+
				`hyphens, dots, and plus sign. Unsupported usernames: %s`, strings.Join(names, ", ")))
	}

	return result, trace.Wrap(err)
}

func processUsername(in *msgraph.User) (string, bool, error) {
	if in == nil {
		return "", false, trace.BadParameter("expected Entra ID user to be non-nil")
	}
	upn := in.UserPrincipalName
	if upn == nil {
		return "", false, trace.BadParameter("expected Entra ID user to have a UPN")
	}

	username := in.Mail
	if username == nil {
		username = upn
	}

	isExternal := false
	// Entra ID  users may have a suffix that indicates they are external users in B2B Guest scenarios.
	// This suffix is removed when the user logins via the SAML assertion so we need to remove it here.
	// more info: https://docs.microsoft.com/en-us/azure/active-directory/external-identities/what-is-b2b
	// and https://learn.microsoft.com/en-us/entra/identity/app-provisioning/how-provisioning-works
	// Example: "user_theirdomain#EXT#@domain -> "user@theirdomain"
	const externalUserSuffix = "#EXT#"
	if idx := strings.Index(*username, externalUserSuffix); idx != -1 {
		// Reformat Entra ID external username [username_domain.com#EXT#@yourtenant.onmicrosoft.com]
		// to a Teleport supported username format [username@domain.com].
		user := (*username)[:idx] // remove #EXT#@domain
		if idx := strings.LastIndex(user, "_"); idx != -1 {
			*username = user[:idx] + "@" + user[idx+1:] // replace the last _ with @
		}
		isExternal = true
	}

	if err := isValidUsername(*username); err != nil {
		return "", false, trace.Wrap(err)
	}

	return *username, isExternal, nil
}

func convertUser(in *msgraph.User, tenantID string, ssoConnectorID string, usersMemberships groupMembershipMap, emitAsRoles bool) (types.User, error) {
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

	const (
		entraIDSAMLClaimName    = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"
		entraIDSAMLClaimEmail   = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
		entraIDSAMLClaimGroups  = "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups"
		entraIDSAMLClaimRoles   = "http://schemas.microsoft.com/ws/2008/06/identity/claims/roles"
		entraIDSAMLGivenName    = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"
		entraIDSAMLSurname      = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname"
		defaultSchemasNamespace = "http://schemas.microsoft.com/identity/claims/"
		tenantIDClaim           = defaultSchemasNamespace + "tenantid"
		objectIdentifierClaim   = defaultSchemasNamespace + "objectidentifier"
		displayNameClaim        = defaultSchemasNamespace + "displayname"
	)
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
	if groups := usersMemberships[*in.ID].groupNames; len(groups) > 0 {
		sort.Strings(groups)
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

func buildUserMemberships(groupsMap map[string]*msgraph.Group, groupMembersMap map[string][]msgraph.GroupMember, groupNameBuilder func(*msgraph.Group) string) groupMembershipMap {
	unwindedGroupMemberships := unwindGroupMembership(groupsMap, groupMembersMap)
	result := map[string]groupMembershipInfo{}
	for groupID, members := range groupMembersMap {
		for _, member := range members {
			if user, ok := member.(*msgraph.User); ok {
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

func isValidUsername(in string) error {
	// Username value is used as a backend key for the user resource.
	// Entra ID user's username may contain an unsupported characters such
	// as single quote ('), forward slash (/) etc and will cause the users
	// reconciler to fail.
	key := backend.NewKey(in)
	if !backend.IsKeySafe(key) {
		return errUnsupportedUsername
	}

	return nil
}

func unameForLog(in *msgraph.User) string {
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
