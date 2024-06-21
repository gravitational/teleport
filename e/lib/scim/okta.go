package scim

import (
	"context"
	"math"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mitchellh/mapstructure"
	oktapi "github.com/okta/okta-sdk-golang/v2/okta"
	oktaquery "github.com/okta/okta-sdk-golang/v2/okta/query"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/okta/common"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

const (
	oktaGroupLimit = 100
)

// oktaShim provides Okta-specific behavior to the provider-agnostic SCIM server
// and resource handlers. A new shim will be created for every request requiring
// Okta-specific behavior.
type oktaShim struct {
	creds    CredentialsService
	locks    LocksService
	users    UsersService
	roles    RolesService
	plugin   *types.PluginV1
	clock    clockwork.Clock
	log      logrus.FieldLogger
	identity IdentityService
}

// Static assertion that the oktaShim implements the `shim` interface
var _ providerShim = (*oktaShim)(nil)

// newOktaShim is a factory function for creating Okta shim values from an Okta
// plugin resource
func newOktaShim(ctx context.Context, plugin types.Plugin, service *Service) (providerShim, error) {
	p, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("unsupported plugin resource")
	}

	oktaSettings := p.Spec.GetOkta()
	if oktaSettings == nil {
		return nil, trace.BadParameter("missing okta settings")
	}

	log := service.log.WithField(
		teleport.ComponentKey,
		teleport.Component(ComponentName, eteleport.ComponentOkta))

	return &oktaShim{
		creds:    service.creds,
		locks:    service.locks,
		clock:    service.clock,
		users:    service.users,
		roles:    service.roles,
		identity: service.identity,
		plugin:   p,
		log:      log,
	}, nil
}

func (s *oktaShim) syncSettings() *types.PluginOktaSyncSettings {
	oktaSettings := s.plugin.Spec.GetOkta()
	if oktaSettings == nil {
		return nil
	}

	return oktaSettings.SyncSettings
}

// authorizeRequest grants or denies access based on a bearer
func (s *oktaShim) authorizeRequest(ctx context.Context, authHeader string) error {
	creds, err := getStaticCreds(ctx, s.creds, s.plugin)
	if err != nil {
		return trace.Wrap(err)
	}

	selectedCredential, err := okta.SelectSCIMToken(creds)
	if err != nil {
		return trace.Wrap(err)
	}

	if selectedCredential == nil || selectedCredential.GetAPIToken() == "" {
		return trace.AccessDenied("no token set")
	}

	if err := checkBearerToken(selectedCredential, authHeader); err != nil {
		return trace.AccessDenied("invalid token")
	}

	return nil
}

func (s *oktaShim) accessListPredicate(_ context.Context, accessList *accesslist.AccessList) bool {
	return okta.MatchByLabels[*accesslist.AccessList](s.plugin.Spec.GetOkta().OrgUrl)(accessList)
}

func (s *oktaShim) userPredicate(ctx context.Context, user types.User) bool {
	if ok := s.userCreatedByOKTAConnector(user); ok {
		// User was created by the same connector.
		// This can happen when SCIM user provisioning is enabled but a SAML transient user still exists in
		// backend. The OKTA SCIM user provisioning will update SAML user to SCIM user without
		// waiting for SAML user expiration.
		return true
	}
	return okta.MatchByLabels[types.User](s.plugin.Spec.GetOkta().OrgUrl)(user)
}

func (s *oktaShim) userCreatedByOKTAConnector(user types.User) bool {
	userConnector := user.GetCreatedBy().Connector
	if userConnector == nil || userConnector.ID == "" {
		return false
	}
	pluginConnectorID := s.plugin.Spec.GetOkta().SyncSettings.SsoConnectorId
	if pluginConnectorID == "" {
		return false
	}
	return userConnector.ID == pluginConnectorID
}

func (s *oktaShim) userToResource(_ context.Context, user types.User) (*scimpb.Resource, error) {
	resource := scimpb.Resource{
		Id:         user.GetName(),
		ExternalId: getOktaUserExternalID(user),
		Meta: &scimpb.Meta{
			Created: timestamppb.New(user.GetCreatedBy().Time),
			Version: user.GetRevision(),
		},
	}

	attribs := attributeSet{usernameAttribute: user.GetName()}
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

	attribStruct, err := structpb.NewStruct(attribs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resource.Attributes = attribStruct

	return &resource, nil
}

func (s *oktaShim) resourceToUser(ctx context.Context, res *scimpb.Resource) (types.User, error) {
	if !s.plugin.Spec.GetOkta().SyncSettings.SyncUsers {
		// Note: User traits can differ between SCIM user and user created
		// by Okta sync service due to different okta user/app user attributes
		// mapping.
		// If periodic user sync is disabled, we don't care about keeping
		// User data in sync between users originated from SCIM and created via OKTA
		// sync service. If only SCIM integration was enabled we will treat user model
		// from SCIM push as a single source of truth.
		user, err := s.createUserFromResource(ctx, res)
		return user, trace.Wrap(err)
	}

	var oktaUser oktaUserResource
	if err := mapstructure.Decode(res.Attributes.AsMap(), &oktaUser); err != nil {
		return nil, trace.Wrap(err)
	}

	s.log.Infof("Attempting to fetch user %s/%s", res.ExternalId, oktaUser.UserName)

	teleportUser, err := s.getOktaUser(ctx, res.ExternalId, s.plugin.Spec.GetOkta())
	if err != nil {
		return nil, trace.Wrap(err, "fetching Okta user from API")
	}

	return teleportUser, nil
}

func (s *oktaShim) onCreatingAccessList(ctx context.Context, acl *accesslist.AccessList) error {
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

func (s *oktaShim) getResourceLabels() map[string]string {
	return map[string]string{
		types.OriginLabel:                  types.OriginOkta,
		types.TeleportInternalResourceType: types.SystemResource,
		eteleport.OktaOrgURLLabel:          s.plugin.Spec.GetOkta().OrgUrl,
	}
}

func (s *oktaShim) onCreatingAccessListMember(_ context.Context, m *accesslist.AccessListMember) error {
	m.Spec.AddedBy = okta.ImporterName
	return nil
}

func (s *oktaShim) onCreatingUser(ctx context.Context, createdUser types.User, res *scimpb.Resource) error {
	return nil
}

func (s *oktaShim) evaluateSAMLConnector(ctx context.Context, user types.User) error {
	client, err := s.oktaClient(ctx)
	if err != nil {
		if trace.IsNotFound(err) {
			// If okta client creation failed due to not found error the OKTA API credentials are not set.
			// In this case user traits to role mapping is not possible because we can't fetch groups from Okta.
			// (OKTA SCIM user push does not include groups)
			return nil
		}
		return trace.Wrap(err)
	}

	oktaUserID, ok := user.GetLabel(eteleport.OktaUserIDLabel)
	if !ok {
		return trace.BadParameter("missing Okta user ID")
	}
	// OKTA groups are not directly available during user SCIM push so we need to fetch them from API.
	groups, _, err := client.User.ListUserGroups(ctx, oktaUserID)
	if err != nil {
		return trace.Wrap(err)
	}
	var groupsList []string
	for _, v := range groups {
		groupsList = append(groupsList, v.Profile.Name)
	}
	connectorID := s.plugin.Spec.GetOkta().SyncSettings.SsoConnectorId
	connector, err := s.identity.GetSAMLConnector(ctx, connectorID, false)
	if err != nil {
		return trace.Wrap(err)
	}
	common.SetUserRolesAndTraits(user, groupsList, connector)
	return nil
}

// onCreatedUser is called by the user handler immediately after the Teleport
// user is created in the cluster. Our implementation ensures that there are no
// outstanding SCIM locks on that user
func (s *oktaShim) onCreatedUser(ctx context.Context, createdUser types.User, res *scimpb.Resource) error {
	oktaSettings := s.plugin.Spec.GetOkta()
	log := s.log.WithField("user", createdUser.GetName())
	log.Info("Ensuring newly-created user has no SCIM locks")

	err := okta.UnlockUser(ctx, createdUser, []string{okta.LockReasonDeactivated},
		oktaSettings.OrgUrl, s.locks)
	if err != nil {
		// This is probably not enough of a reason to fail the provisioning, but
		// it should be logged
		log.Errorf("Failed unlocking user: %s", err.Error())
	}
	return nil
}

// oktaUserResource describes the Okta-specific attributes for a user
type oktaUserResource struct {
	UserName string `mapstructure:"userName"`
	Active   *bool  `mapstructure:"active"`
}

// onUpdatingUser handles a user update request. Okta piggybacks user activation
// and deactivation into "update" messages
func (s *oktaShim) onUpdatingUser(ctx context.Context, teleportUser types.User, res *scimpb.Resource) (types.User, bool, error) {
	oktaSettings := s.plugin.Spec.GetOkta()
	log := s.log.WithField("user", teleportUser.GetName())

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
			log.Debug("Okta activating user. Unlocking.")
			err := okta.UnlockUser(ctx, teleportUser, []string{okta.LockReasonDeactivated},
				oktaSettings.OrgUrl, s.locks)
			if err != nil {
				return nil, false, trace.Wrap(err)
			}
			return teleportUser, false, nil
		}

		// if we get to here, this is a deactivation request as per
		// https://developer.okta.com/docs/reference/scim/scim-20/#delete-users
		log.Debug("Okta deactivating user. Locking.")
		_, err := okta.LockUser(ctx, okta.LockParams{
			User:     teleportUser,
			Reason:   okta.LockReasonDeactivated,
			Message:  "User deactivated by Okta",
			OrgURL:   oktaSettings.OrgUrl,
			Clock:    s.clock,
			LocksSvc: s.locks,
			Log:      s.log,
		})
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		if err := s.users.DeleteUser(ctx, teleportUser.GetName()); err != nil {
			if !trace.IsNotFound(err) {
				return nil, false, trace.Wrap(err)
			}
		}
		return teleportUser, false, nil
	}

	if !s.plugin.Spec.GetOkta().SyncSettings.SyncUsers {
		// If periodic user sync is disabled, we don't care about keeping
		// user in data in sync between SCIM user and user create by Okta
		// sync service and threat user model from SCIM push as a single
		// source of truth.
		// Note that user traits can differ between SCIM user and user created
		// by Okta sync service due to different okta user/app user attributes
		// mapping.
		user, err := s.createUserFromResource(ctx, res)
		return user, true, trace.Wrap(err)
	}

	// Otherwise, Okta is actually trying to update our user. Because Okta's
	// user and appuser profile schemes are so wildly customizable, and the
	// mapping from an appuser profile to the structured SCIM data is not well
	// known, reconciling the SCIm data with the teleport user traits is almost
	// impossible.
	//
	// To sidestep the whole mess, we use this skim request as a trigger to poll
	// the Okta API for the target user's data as a flat list of attributes and
	// update as per the Okta sync service

	newUser, err := s.resourceToUser(ctx, res)
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
func (s *oktaShim) createUserFromResource(ctx context.Context, res *scimpb.Resource) (types.User, error) {
	if res == nil {
		return nil, trace.BadParameter("Resource may not be empty")
	}
	pluginSettings := s.plugin.Spec.GetOkta().SyncSettings
	if res.Attributes == nil {
		return nil, trace.BadParameter("Missing resource attributes")
	}
	scimAttribs := res.Attributes.AsMap()
	username := res.Id
	if username == "" {
		var err error
		username, err = getAttr(scimAttribs, "userName")
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	user, err := types.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetStaticLabels(map[string]string{
		types.OriginLabel:         types.OriginOkta,
		eteleport.OktaOrgURLLabel: s.plugin.Spec.GetOkta().OrgUrl,
		eteleport.OktaUserIDLabel: res.ExternalId,
	})
	user.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{
			Name: teleport.UserSystem,
		},
		Time: s.clock.Now(),
		Connector: &types.ConnectorRef{
			ID:       pluginSettings.SsoConnectorId,
			Type:     constants.SAML,
			Identity: res.ExternalId,
		},
	})
	if err := s.evaluateSAMLConnector(ctx, user); err != nil {
		return nil, trace.Wrap(err)
	}
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
func (s *oktaShim) getOktaUser(ctx context.Context, userID string, oktaSettings *types.PluginOktaSettings) (types.User, error) {
	c, err := s.oktaClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appUser, _, err := c.Application.GetApplicationUser(ctx, oktaSettings.SyncSettings.AppId, userID, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newUser, err := okta.ConvertAppUser(appUser, s.clock,
		oktaSettings.SyncSettings.SsoConnectorId,
		oktaSettings.OrgUrl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newUser, nil
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

	s.log.Debugf("Looking up group %q", displayName)
	groups, _, err := c.Group.ListGroups(ctx, &oktaquery.Params{Q: displayName, Limit: oktaGroupLimit})
	if err != nil {
		return "", trace.Wrap(err, "listing groups")
	}

	candidateID := ""

	// Okta performs a "starts-with" search, so we may get multiple results.
	for _, candidate := range groups {
		log := s.log.WithField("candidate", candidate.Id)

		if candidate.Profile == nil {
			log.Info("Candidate has no profile")
			continue
		}

		log.Infof("testing group %q", candidate.Profile.Name)

		if candidate.Profile.Name != displayName {
			log.Infof("Display name mismatch")
			continue
		}

		if candidate.Type != "OKTA_GROUP" {
			log.Infof("Invalid group type")
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

func (s *oktaShim) oktaClient(ctx context.Context) (*oktapi.Client, error) {
	staticCredsRef := s.plugin.GetCredentials().GetStaticCredentialsRef()
	if staticCredsRef == nil {
		return nil, trace.NotFound("no static credentials found")
	}

	staticCreds, err := s.creds.GetPluginStaticCredentialsByLabels(ctx, staticCredsRef.Labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oktAPIToken, err := okta.SelectAPIToken(staticCreds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	syncSettings := s.plugin.Spec.GetOkta()
	_, apiClient, err := oktapi.NewClient(ctx,
		oktapi.WithCache(false),
		oktapi.WithOrgUrl(syncSettings.OrgUrl),
		oktapi.WithToken(oktAPIToken.GetAPIToken()),
		oktapi.WithRequestTimeout(okta.RequestTimeoutSeconds),
		oktapi.WithRateLimitMaxRetries(math.MaxInt32),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return apiClient, nil
}

func (s *oktaShim) defaultOwners() []accesslist.Owner {
	settings := s.syncSettings()
	if settings == nil {
		return nil
	}
	result := make([]accesslist.Owner, len(settings.DefaultOwners))
	for i, owner := range settings.DefaultOwners {
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
