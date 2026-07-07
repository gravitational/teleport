package okta

import (
	"context"
	"crypto"
	"fmt"
	"maps"
	"math/big"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	oktasdk "github.com/okta/okta-sdk-golang/v2/okta"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktaconvert "github.com/gravitational/teleport/e/lib/okta/convert"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/srv/app"
)

// oktaGroupToUserGroup converts an Okta group object to a types.UserGroup object.
func (s *Service) oktaGroupToUserGroup(oktaGroup *oktasdk.Group, appIDs []string) (types.UserGroup, error) {
	if err := isGroupValid(oktaGroup); err != nil {
		return nil, trace.Wrap(err)
	}

	labels, err := s.getGroupLabels(oktaGroup.Id, oktaGroup.Profile.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels[types.OriginLabel] = types.OriginOkta
	labels[types.OktaGroupNameLabel] = oktaGroup.Profile.Name
	labels[eteleport.OktaOrgURLLabel] = s.orgURL
	labels[eteleport.OktaGroupIDLabel] = oktaGroup.Id

	description := oktaGroup.Profile.Name

	if oktaGroup.Profile.Description != "" {
		description += fmt.Sprintf(" (%s)", oktaGroup.Profile.Description)
		labels[types.OktaGroupDescriptionLabel] = oktaGroup.Profile.Description
	}

	userGroup, err := types.NewUserGroup(
		types.Metadata{
			Name:        oktaGroup.Id,
			Description: description,
			Labels:      labels,
		},
		types.UserGroupSpecV1{
			Applications: appIDs,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userGroup, nil
}

type oktaApplicationEmbedLinks struct {
	AppLinks []oktaApplicationEmbedLink `mapstructure:"appLinks"`
}

type oktaApplicationEmbedLink struct {
	Name string `mapstructure:"name"`
	Href string `mapstructure:"href"`
}

// oktaAppToAppServers converts an Okta app object to types.Application objects. This will convert
// multiple appLinks in an Okta object into multiple app_server resources.
func (s *Service) oktaAppToAppServers(ctx context.Context, oktaApplication *oktasdk.Application, groupIDs []string) ([]types.AppServer, error) {
	appIdentifier := fmt.Sprintf("%s (%s)", oktaApplication.Id, oktaApplication.Label)

	// Filter out Okta apps if they're not the kind we want to display to users.
	if err := isAppValid(oktaApplication); err != nil {
		return nil, trace.Wrap(err)
	}

	if oktaApplication.Links == nil {
		return nil, trace.BadParameter("links is missing in okta application object %s", appIdentifier)
	}

	// Unfortunately the app links are stuffed into an interface{}, so we've got to extract the
	// fields for app links using mapstructure.
	embeddedLinks := &oktaApplicationEmbedLinks{}
	if err := mapstructure.Decode(oktaApplication.Links, embeddedLinks); err != nil {
		return nil, trace.Wrap(err)
	}

	if len(embeddedLinks.AppLinks) == 0 {
		return nil, trace.BadParameter("app links is empty in okta application object %s", appIdentifier)
	}

	var appServers []types.AppServer

	labels, err := s.getApplicationLabels(oktaApplication.Id, oktaApplication.Label)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels[types.OriginLabel] = types.OriginOkta
	labels[types.OktaAppNameLabel] = oktaApplication.Label
	labels[eteleport.OktaOrgURLLabel] = s.orgURL
	labels[eteleport.OktaAppIDLabel] = oktaApplication.Id
	if isHiddenApp(oktaApplication) {
		labels[eteleport.OktaAppHiddenLabel] = "true"
	}

	// Create an app for each app link. This is required because there can be multiple
	// app links per Okta application.
	for _, appLink := range embeddedLinks.AppLinks {
		appID, err := AppName(oktaApplication.Id, appLink.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		publicAddr, err := app.FindPublicAddr(ctx, s.accessPoint, "", appID, "")
		if err != nil {
			return nil, trace.Wrap(err)
		}

		copyLabels := maps.Clone(labels)
		copyLabels[types.OktaAppDescriptionLabel] = appLink.Name

		app, err := types.NewAppV3(
			types.Metadata{
				Name:        appID,
				Description: oktaApplication.Label,
				Labels:      copyLabels,
			},
			types.AppSpecV3{
				URI:        appLink.Href,
				PublicAddr: publicAddr,
				UserGroups: groupIDs,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		appServer, err := types.NewAppServerV3(
			types.Metadata{
				Name:        app.GetName(),
				Description: app.GetDescription(),
				Labels:      app.GetStaticLabels(),
			},
			types.AppServerSpecV3{
				Version:  teleport.Version,
				Hostname: s.hostname,
				HostID:   oktaAppServerHostID,
				App:      app,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		appServers = append(appServers, appServer)
	}

	return appServers, nil
}

// AppName returns an app name based on the Okta app ID and embed link name.
func AppName(id, appLinkName string) (string, error) {
	// Let's create a short unique string for the app ID.
	hasher := crypto.SHA256.New()
	_, err := fmt.Fprintf(hasher, "%s-%s", id, appLinkName)
	if err != nil {
		return "", trace.Wrap(err)
	}
	hashedID := hasher.Sum(nil)

	// Take up to the first 14 characters of the resulting base36 encoded hash.
	// Otherwise, the links for the apps in the UI are far too long. We choose
	// 14 characters as 36^14 is greater than the original choice of base64's
	// 64^12.
	return shortenedEncodedID(hashedID, 14), nil
}

// shortenedEncodedID takes a raw hashed ID and returns an encoded, shortened version up to
// the given length.
func shortenedEncodedID(hashedID []byte, length int) string {
	encodedID := base36Encode(hashedID)
	prefixLen := min(len(encodedID), length)
	return encodedID[:prefixLen]
}

// isGroupValid will return an error if the group shouldn't be synced with the list of user groups.
func isGroupValid(oktaGroup *oktasdk.Group) error {
	if oktaGroup.Profile == nil {
		return trace.BadParameter("the okta group %s has no profile", oktaGroup.Id)
	}

	if oktaGroup.Profile.Name == oktaapi.OktaGroupEveryone {
		return trace.BadParameter("group %s is %s", oktaGroup.Id, oktaapi.OktaGroupEveryone)
	}

	return nil
}

// isAppValid will return an error if the application shouldn't be synced with the app catalog.
func isAppValid(app *oktasdk.Application) error {
	appIdentifier := fmt.Sprintf("%s (%s)", app.Id, app.Label)

	// If the application isn't active, then we'll filter it out.
	if app.Status != oktaapi.OktaActive {
		return trace.BadParameter("application %s is not active", appIdentifier)
	}

	// We'll filter out the admin console as well.
	if app.Label == oktaapi.OktaAdminConsole {
		return trace.BadParameter("application %s is the Okta admin console", app.Id)
	}
	return nil
}

func isHiddenApp(app *oktasdk.Application) bool {
	return app.Visibility != nil && app.Visibility.Hide != nil && app.Visibility.Hide.Web != nil && *app.Visibility.Hide.Web
}

// base36Encode will take input and encode in in base36. In this case,
// the base36 will include all lower case alphabetic characters, numbers,
// and no others. This is necessary because base64 URL will include the _
// character, which is not accepted by many cert providers, and additionally
// will include upper case characters.
func base36Encode(data []byte) string {
	hashInt := big.NewInt(0)
	hashInt = hashInt.SetBytes(data)
	return hashInt.Text(36)
}

func (s *Service) convertAppUser(appUser *oktasdk.AppUser) (types.User, error) {
	u, err := oktaconvert.ConvertOktaAppUser(oktaconvert.ConvertOktaUserArgs[*oktasdk.AppUser]{
		Clock:              s.clock,
		SAMLConnectorName:  s.ssoConnectorID,
		OktaOrgURL:         s.orgURL,
		OktaSDKUser:        appUser,
		AssignDefaultRoles: s.assignDefaultRoles,
	})
	return u, trace.Wrap(err)
}

func (s *Service) convertOrgUser(orgUser *oktasdk.User) (types.User, error) {
	u, err := oktaconvert.ConvertOktaOrgUser(oktaconvert.ConvertOktaUserArgs[*oktasdk.User]{
		Clock:              s.clock,
		SAMLConnectorName:  s.ssoConnectorID,
		OktaOrgURL:         s.orgURL,
		OktaSDKUser:        orgUser,
		AssignDefaultRoles: s.assignDefaultRoles,
	})
	return u, trace.Wrap(err)
}
