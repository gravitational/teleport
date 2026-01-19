package okta

import (
	"context"
	"log/slog"
	"math"
	"strings"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	oktasdk "github.com/okta/okta-sdk-golang/v2/okta"
	oktaquery "github.com/okta/okta-sdk-golang/v2/okta/query"

	"github.com/gravitational/teleport"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/okta"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktacommon "github.com/gravitational/teleport/e/lib/okta/common"
	oktaconvert "github.com/gravitational/teleport/e/lib/okta/convert"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	"github.com/gravitational/teleport/e/lib/scim/conv"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	"github.com/gravitational/teleport/e/lib/scim/service/provider/okta/oktahandler"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

const (
	oktaGroupLimit = 100
)

// oktaShim provides Okta-specific behavior to the provider-agnostic SCIM server
// and resource handlers. A new shim will be created for every request requiring
// Okta-specific behavior.
type oktaShim struct {
	common.Config
	plugin *types.PluginV1
}

// New creates a new Okta handled for the given resource type.
func New(config common.Config, pluginV1 *types.PluginV1, resourceType string) (common.ResourceHandler, error) {
	oktaSettings := pluginV1.Spec.GetOkta()
	if oktaSettings == nil {
		return nil, trace.BadParameter("missing okta settings")
	}
	config.Logger = slog.With(teleport.ComponentKey, teleport.Component("scim", eteleport.ComponentOkta))
	switch resourceType {
	case "Users":
		return &oktahandler.UserHandler{
			Config: config,
			ProviderUser: &oktaShim{
				Config: config,
				plugin: pluginV1,
			},
		}, nil
	case "Groups":
		return &oktahandler.GroupHandler{
			Config: config,
			ProviderGroup: &oktaShim{
				Config: config,
				plugin: pluginV1,
			},
		}, nil
	default:
		return nil, trace.BadParameter("unsupported resource type %q", resourceType)
	}
}

func (s *oktaShim) oktaOrgURL() string {
	return s.plugin.Spec.GetOkta().OrgUrl
}

func (s *oktaShim) syncSettings() *types.PluginOktaSyncSettings {
	syncSettings := s.plugin.Spec.GetOkta().GetSyncSettings()
	if syncSettings == nil {
		return &types.PluginOktaSyncSettings{}
	}
	return syncSettings
}

// AccessListPredicate checks if the access list is "owned" by this okta.
func (s *oktaShim) AccessListPredicate(_ context.Context, accessList *accesslist.AccessList) bool {
	return okta.MatchByLabels[*accesslist.AccessList](s.oktaOrgURL())(accessList)
}

// UserPredicate checks if the user is "owned" by this okta.
func (s *oktaShim) UserPredicate(ctx context.Context, user types.User) bool {
	if ok := s.userCreatedByOktaConnector(user); ok {
		// User was created by the same connector.
		// This can happen when SCIM user provisioning is enabled but a SAML transient user still exists in
		// backend. The Okta SCIM user provisioning will update SAML user to SCIM user without
		// waiting for SAML user expiration.
		return true
	}
	return okta.MatchByLabels[types.User](s.oktaOrgURL())(user)
}

func (s *oktaShim) userCreatedByOktaConnector(user types.User) bool {
	userConnector := user.GetCreatedBy().Connector
	if userConnector == nil || userConnector.ID == "" {
		return false
	}
	pluginConnectorID := s.syncSettings().SsoConnectorId
	if pluginConnectorID == "" {
		return false
	}
	return userConnector.ID == pluginConnectorID
}

// UserToResource converts a Teleport user to an Okta SCIM resource. The
func (s *oktaShim) UserToResource(_ context.Context, user types.User) (*scimpb.Resource, error) {
	u, err := conv.UserToResource(
		user,
		conv.WithExternalIDFunc(getOktaUserExternalID),
		conv.WithAttributes(getOktaTrailsUserAttributes(user)),
		conv.WithUserOptionClock(s.Clock),
	)
	return u, trace.Wrap(err)
}

func getOktaTrailsUserAttributes(user types.User) map[string]any {
	var attribs = make(map[string]any)
	for k, v := range user.GetTraits() {
		if !strings.HasPrefix(k, eteleport.OktaTraitPrefix) {
			continue
		}
		k = strings.TrimPrefix(k, eteleport.OktaTraitPrefix)

		if len(v) == 1 {
			attribs[k] = v[0]
			continue
		}
		attribs[k] = v
	}
	return attribs
}

// ResourceToUser converts an Okta SCIM resource to a Teleport user
func (s *oktaShim) ResourceToUser(ctx context.Context, res *scimpb.Resource) (types.User, error) {
	var err error
	var teleportUser types.User
	if s.syncSettings().GetEnableUserSync() {
		if teleportUser, err = s.getOktaUser(ctx, res.ExternalId); err != nil {
			return nil, trace.Wrap(err, "fetching Okta user from API")
		}
	} else {
		if teleportUser, err = s.createUserFromSCIMResource(res); err != nil {
			return nil, trace.Wrap(err, "converting SCIM user to Teleport user")
		}
	}

	if err := s.evaluateSAMLConnector(ctx, teleportUser); err != nil {
		return nil, trace.Wrap(err, "setting user %q roles and traits from connector", teleportUser.GetName())
	}

	return teleportUser, nil
}

// OnCreatingAccessList is called by the access list handler immediately
// after the access list is created
func (s *oktaShim) OnCreatingAccessList(ctx context.Context, acl *accesslist.AccessList) error {
	// The group ID should be unset on the initial ACL creation. In order to
	// maintain compatibility with the Okta Sync service, we reach out via the
	// Okta API to find out what the Okta ID for the group is.
	groupID := acl.GetName()
	if groupID == "" {
		var err error
		groupID, err = s.lookupGroup(ctx, acl.Spec.Title)
		if err != nil {
			return trace.Wrap(err, "looking up group ID")
		}
	}

	acl.Metadata.Name = groupID
	acl.Spec.Owners = s.defaultOwners()

	return nil
}

// GetResourceLabels returns the labels that should be set on all resources
func (s *oktaShim) GetResourceLabels() map[string]string {
	return map[string]string{
		types.OriginLabel:                  types.OriginOkta,
		types.TeleportInternalResourceType: types.SystemResource,
		eteleport.OktaOrgURLLabel:          s.oktaOrgURL(),
	}
}

// OnCreatingAccessListMember is called by the access list handler
func (s *oktaShim) OnCreatingAccessListMember(_ context.Context, m *accesslist.AccessListMember) error {
	m.Spec.AddedBy = okta.ImporterName
	return nil
}

// OnCreatingUser is called by the user handler immediately before the Teleport
// user is created in the cluster.
func (s *oktaShim) OnCreatingUser(ctx context.Context, createdUser types.User, res *scimpb.Resource) error {
	return nil
}

func (s *oktaShim) evaluateSAMLConnector(ctx context.Context, user types.User) error {
	client, err := s.oktaClient(ctx)
	if err != nil {
		if trace.IsNotFound(err) {
			// If okta client creation failed due to not found error the Okta API credentials are not set.
			// In this case user traits to role mapping is not possible because we can't fetch groups from Okta.
			// (Okta SCIM user push does not include groups)
			return nil
		}
		return trace.Wrap(err)
	}

	oktaUserID, ok := user.GetLabel(eteleport.OktaUserIDLabel)
	if !ok {
		return trace.BadParameter("missing Okta user ID")
	}
	// Okta groups are not directly available during user SCIM push so we need to fetch them from API.
	groups, _, err := client.User.ListUserGroups(ctx, oktaUserID)
	if err != nil {
		return trace.Wrap(err)
	}
	var groupsList []string
	for _, v := range groups {
		groupsList = append(groupsList, v.Profile.Name)
	}
	connectorID := s.syncSettings().SsoConnectorId
	connector, err := s.IdentityService.GetSAMLConnector(ctx, connectorID, false)
	if err != nil {
		return trace.Wrap(err)
	}
	oktacommon.SetUserRolesAndTraits(user, groupsList, connector)
	return nil
}

// OnCreatedUser is called by the user handler immediately after the Teleport
// user is created in the cluster. Our implementation ensures that there are no
// outstanding SCIM locks on that user
func (s *oktaShim) OnCreatedUser(ctx context.Context, createdUser types.User, res *scimpb.Resource) error {
	log := s.Logger.With("user", createdUser.GetName())
	log.InfoContext(ctx, "Ensuring newly-created user has no SCIM locks")

	err := okta.UnlockUser(ctx, createdUser, []string{okta.LockReasonDeactivated},
		s.oktaOrgURL(), s.LocksService)
	if err != nil {
		// This is probably not enough of a reason to fail the provisioning, but
		// it should be logged
		log.ErrorContext(ctx, "Failed unlocking user", "error", err)
	}
	return nil
}

// oktaUserResource describes the Okta-specific attributes for a user
type oktaUserResource struct {
	UserName string `mapstructure:"userName"`
	Active   *bool  `mapstructure:"active"`
}

// OnUpdatingUser handles a user update request. Okta piggybacks user activation
// and deactivation into "update" messages
func (s *oktaShim) OnUpdatingUser(ctx context.Context, teleportUser types.User, res *scimpb.Resource) (types.User, bool, error) {
	log := s.Logger.With("user", teleportUser.GetName())

	var oktaUser oktaUserResource
	if err := mapstructure.Decode(res.Attributes.AsMap(), &oktaUser); err != nil {
		return nil, false, trace.Wrap(err)
	}

	// Okta uses the "active" attribute as a signal rather than as a simple
	// attribute; there are three possible states:
	//  true:  Okta is signaling that it wants to activate the account, either
	//         as a new account, or re-activating a suspended account
	//  false: Okta is signaling that the user has been disabled, and we should
	//         take action to lock the user out.
	// absent: Business as usual; a normal status update
	//
	//  See: https://developer.okta.com/docs/reference/scim/scim-20/#create-users
	if oktaUser.Active != nil {
		// if this is an activation request...
		if (*oktaUser.Active) == true {
			log.DebugContext(ctx, "Okta activating user - unlocking")
			err := okta.UnlockUser(ctx, teleportUser, []string{okta.LockReasonDeactivated},
				s.oktaOrgURL(), s.LocksService)
			if err != nil {
				return nil, false, trace.Wrap(err)
			}
			return teleportUser, false, nil
		}

		// if we get to here, this is a deactivation request as per
		// https://developer.okta.com/docs/reference/scim/scim-20/#delete-users
		log.DebugContext(ctx, "Okta deactivating user - locking")
		_, err := okta.LockUser(ctx, okta.LockParams{
			User:     teleportUser,
			Reason:   okta.LockReasonDeactivated,
			Message:  "User deactivated by Okta",
			OrgURL:   s.oktaOrgURL(),
			Clock:    s.Clock,
			LocksSvc: s.LocksService,
			Logger:   s.Logger,
		})
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		if err := s.UsersService.DeleteUser(ctx, teleportUser.GetName()); err != nil {
			if !trace.IsNotFound(err) {
				return nil, false, trace.Wrap(err)
			}
		}
		return teleportUser, false, nil
	}

	newUser, err := s.ResourceToUser(ctx, res)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}

	// Things like the creation time, original creator, etc need to be preserved
	// across the update.
	okta.PreserveUserMetadata(newUser, teleportUser)

	return newUser, true, nil
}

// resourceToUser constructs an in-memory Teleport user from the supplied
// SCIM resource
func (s *oktaShim) createUserFromSCIMResource(res *scimpb.Resource) (types.User, error) {
	if res == nil {
		return nil, trace.BadParameter("Resource may not be empty")
	}
	if res.Attributes == nil {
		return nil, trace.BadParameter("Missing resource attributes")
	}
	username := res.Id
	if username == "" {
		attrs := res.GetAttributes().AsMap()
		var err error
		username, err = getAttr(attrs, "userName")
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	user, err := oktaconvert.NewTeleportUser(oktaconvert.NewTeleportUserArgs{
		Clock:              s.Clock,
		SAMLConnectorName:  s.syncSettings().SsoConnectorId,
		OktaOrgURL:         s.oktaOrgURL(),
		OktaLogin:          username,
		OktaID:             res.GetExternalId(),
		IgnoreOktaStatus:   true,
		OktaProfile:        make(map[string]any, 0),
		AssignDefaultRoles: s.syncSettings().GetAssignDefaultRoles(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetRevision(res.GetMeta().GetVersion())

	return user, nil
}

func getAttr(attrs map[string]any, key string) (string, error) {
	untypedValue, ok := attrs[key]
	if !ok {
		return "", trace.BadParameter("Missing required attribute %s", key)
	}

	value, ok := untypedValue.(string)
	if !ok {
		return "", trace.BadParameter("Invalid attribute type %T", untypedValue)
	}

	return value, nil
}

// getOktaUser fetches an appuser profile from the Okta Org API
func (s *oktaShim) getOktaUser(ctx context.Context, userID string) (types.User, error) {
	c, err := s.oktaClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appUser, _, err := c.Application.GetApplicationUser(ctx, s.syncSettings().AppId, userID, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u, err := oktaconvert.ConvertOktaAppUser(oktaconvert.ConvertOktaUserArgs[*oktasdk.AppUser]{
		Clock:              s.Clock,
		SAMLConnectorName:  s.syncSettings().SsoConnectorId,
		OktaOrgURL:         s.oktaOrgURL(),
		OktaSDKUser:        appUser,
		AssignDefaultRoles: s.syncSettings().GetAssignDefaultRoles(),
	})
	return u, trace.Wrap(err)
}

// lookupGroup looks up the Okta group ID for a given display name. For a given
// displayName, `lookupGroup()` will list all Okta groups with that name, and if
// exactly one group is found with that name, it will return that group's
// Okta ID. If either no group or multiple groups are found, `lookupGroup()`
// returns an error
func (s *oktaShim) lookupGroup(ctx context.Context, displayName string) (string, error) {
	c, err := s.oktaClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// For authoritative details on how Okta group name search works, visit
	//  https://developer.okta.com/docs/reference/api/groups/#list-groups-with-search
	//
	// But to summarize:
	//  * Searching and paging are mutually exclusive in the API.
	//  * Okta performs a "starts-with" search, so we may get multiple results.
	//  * Exact matches are sorted first in the returned list

	s.Logger.DebugContext(ctx, "Looking up group", "group_name", displayName)
	groups, _, err := c.Group.ListGroups(ctx, &oktaquery.Params{Q: displayName, Limit: oktaGroupLimit})
	if err != nil {
		return "", trace.Wrap(err, "listing groups")
	}

	candidateID := ""

	// Okta performs a "starts-with" search, so we may get multiple results.
	for _, candidate := range groups {
		log := s.Logger.With("candidate", candidate.Id)

		if candidate.Profile == nil {
			log.InfoContext(ctx, "Candidate has no profile")
			continue
		}

		log.InfoContext(ctx, "testing group", "group_name", candidate.Profile.Name)

		if candidate.Profile.Name != displayName {
			log.InfoContext(ctx, "Display name mismatch")
			continue
		}

		if candidate.Type != "OKTA_GROUP" {
			log.InfoContext(ctx, "Invalid group type")
			continue
		}

		if candidateID != "" {
			return "", trace.BadParameter("multiple candidates found for okta group %q", displayName)
		}

		candidateID = candidate.Id
	}

	if candidateID == "" {
		return "", trace.NotFound("no such okta group: %q", displayName)
	}

	return candidateID, nil
}

// oktaClient creates an Okta client or returns NotFound if the credentials for the Okta client are
// not present.
func (s *oktaShim) oktaClient(ctx context.Context) (*oktasdk.Client, error) {
	staticCreds, err := oktaplugin.GetStaticCredentials(ctx, s.CredentialsService, s.plugin.Credentials.GetStaticCredentialsRef())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var oktaAuthProvider oktaapi.AuthProvider
	selectedOktaCreds, err := oktaplugin.SelectOktaCredentials(staticCreds)
	if err != nil {
		return nil, trace.Wrap(err, "looking up for Okta credentials")
	}
	switch {
	case selectedOktaCreds.OauthClientId != "":
		oktaAuthProvider = oktaapi.NewOauthProviderWithOktaCASigner(ctx, oktaapi.OauthOktaCACredentialsConfig{
			OAuthClientID: selectedOktaCreds.OauthClientId,
			AuthService:   s.CertAuthorityGetter,
			CAKeyStore:    s.JWTSignerGetter,
			Clock:         s.Clock,
		})
	case selectedOktaCreds.ApiToken != "":
		oktaAuthProvider = oktaapi.NewSSWSAuthProvider(selectedOktaCreds.ApiToken)
	default:
		return nil, trace.NotFound("Okta API credentials not found in plugin static credentials")
	}

	oktaOAuthScopes := oktacommon.GetReadOnlyOAuthScopes()
	oktaOpts := append(
		oktaAuthProvider.GetAuthOptions(),
		oktasdk.WithCache(false),
		oktasdk.WithOrgUrl(s.oktaOrgURL()),
		oktasdk.WithRequestTimeout(okta.RequestTimeoutSeconds),
		oktasdk.WithRateLimitMaxRetries(math.MaxInt32),
		oktasdk.WithScopes(oktaOAuthScopes),
		oktasdk.WithHttpClientPtr(s.HTTPClient),
	)
	_, oktaSDKClient, err := oktasdk.NewClient(ctx, oktaOpts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(kopiczko) switch oktaShim to use oktaapi.Client and test this call
	if err := oktacommon.CheckClientOAuthScopes(ctx, oktaapi.NewAPIClient(oktaSDKClient), oktaOAuthScopes...); err != nil {
		return nil, trace.Wrap(err, "checking Okta client required OAuth scopes")
	}

	return oktaSDKClient, nil
}

func (s *oktaShim) defaultOwners() []accesslist.Owner {
	result := make([]accesslist.Owner, len(s.syncSettings().DefaultOwners))
	for i, owner := range s.syncSettings().DefaultOwners {
		result[i] = accesslist.Owner{
			Name: owner,
		}
	}
	return result
}

func getOktaUserExternalID(u types.User) string {
	if id, ok := u.GetLabel(eteleport.OktaUserIDLabel); ok {
		return id
	}
	return ""
}
