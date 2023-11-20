package okta

import (
	"crypto"
	"fmt"
	"math/big"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mitchellh/mapstructure"
	"github.com/okta/okta-sdk-golang/v2/okta"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/trait"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/srv/app"
)

// oktaGroupToUserGroup converts an Okta group object to a types.UserGroup object.
func (s *Service) oktaGroupToUserGroup(oktaGroup *okta.Group, appIDs []string) (types.UserGroup, error) {
	if err := isGroupValid(oktaGroup); err != nil {
		return nil, trace.Wrap(err)
	}

	labels, err := s.getGroupLabels(oktaGroup.Id, oktaGroup.Profile.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels[types.OriginLabel] = types.OriginOkta
	labels[eteleport.OktaOrgURLLabel] = s.orgURL
	labels[eteleport.OktaGroupIDLabel] = oktaGroup.Id

	description := oktaGroup.Profile.Name

	if oktaGroup.Profile.Description != "" {
		description += fmt.Sprintf(" (%s)", oktaGroup.Profile.Description)
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

type embeddedLinks struct {
	AppLinks []appLink `mapstructure:"appLinks"`
	Metadata *appLink  `mapstructure:"metadata"`
}

type appLink struct {
	Name string `mapstructure:"name"`
	Href string `mapstructure:"href"`
	Type string `mapstructure:"type"`
}

// oktaAppToApps converts an Okta app object to types.Application objects. This will convert
// multiple appLinks in an Okta object into multiple applications.
func (s *Service) oktaAppToApp(oktaApplication *okta.Application, groupIDs []string) ([]*types.AppV3, error) {
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
	embeddedLinks := &embeddedLinks{}
	if err := mapstructure.Decode(oktaApplication.Links, embeddedLinks); err != nil {
		return nil, trace.Wrap(err)
	}

	if len(embeddedLinks.AppLinks) == 0 {
		return nil, trace.BadParameter("app links is empty in okta application object %s", appIdentifier)
	}

	var apps []*types.AppV3

	labels, err := s.getApplicationLabels(oktaApplication.Id, oktaApplication.Label)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels[types.OriginLabel] = types.OriginOkta
	labels[eteleport.OktaOrgURLLabel] = s.orgURL
	labels[eteleport.OktaAppIDLabel] = oktaApplication.Id

	// Create an app for each app link. This is required because there can be multiple
	// app links per Okta application.
	for _, appLink := range embeddedLinks.AppLinks {
		appID, err := appName(s.hash, oktaApplication.Id, appLink.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		publicAddr, err := app.FindPublicAddr(s.accessPoint, "", appID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		app, err := types.NewAppV3(
			types.Metadata{
				Name:        appID,
				Description: oktaApplication.Label,
				Labels:      labels,
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

		apps = append(apps, app)
	}

	return apps, nil
}

// appName returns an app name based on the ID and app link name.
func appName(hash crypto.Hash, id, appLinkName string) (string, error) {
	// Let's create a short unique string for the app ID.
	hasher := hash.New()
	_, err := hasher.Write([]byte(fmt.Sprintf("%s-%s", id, appLinkName)))
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
	prefixLen := len(encodedID)
	if prefixLen > length {
		prefixLen = length
	}
	return encodedID[:prefixLen]
}

// isGroupValid will return an error if the group shouldn't be synced with the list of user groups.
func isGroupValid(oktaGroup *okta.Group) error {
	if oktaGroup.Profile == nil {
		return trace.BadParameter("the okta group %s has no profile", oktaGroup.Id)
	}

	if oktaGroup.Profile.Name == oktaGroupEveryone {
		return trace.BadParameter("group %s is %s", oktaGroup.Id, oktaGroupEveryone)
	}

	return nil
}

// isAppValid will return an error if the application shouldn't be synced with the app catalog.
func isAppValid(app *okta.Application) error {
	appIdentifier := fmt.Sprintf("%s (%s)", app.Id, app.Label)

	// If the application isn't active, then we'll filter it out.
	if app.Status != oktaActive {
		return trace.BadParameter("application %s is not active", appIdentifier)
	}

	// We'll filter out the admin console as well.
	if app.Label == oktaAdminConsole {
		return trace.BadParameter("application %s is the Okta admin console", app.Id)
	}

	// Make sure the app isn't hidden.
	if app.Visibility != nil && app.Visibility.Hide != nil && app.Visibility.Hide.Web != nil && *app.Visibility.Hide.Web {
		return trace.BadParameter("application %s is hidden from the web", appIdentifier)
	}

	return nil
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

type oktaUserProfile struct {
	Login  string                 `mapstructure:"login"`
	Fields map[string]interface{} `mapstructure:",remain"`
}

func (p *oktaUserProfile) AsTraits() trait.Traits {
	traits := trait.Traits{}
	for k, v := range p.Fields {
		switch value := v.(type) {
		case string:
			traits[eteleport.OktaTraitPrefix+k] = []string{value}
		case []string:
			traits[eteleport.OktaTraitPrefix+k] = value
		}
	}
	return traits
}

func parseOktaUserProfile(oktaUser *okta.User) (*oktaUserProfile, error) {
	var profile oktaUserProfile
	err := mapstructure.Decode(oktaUser.Profile, &profile)
	if err != nil {
		return nil, trace.Wrap(err, "parsing okta user profile")
	}
	return &profile, nil
}

func makeUserConverter(clock clockwork.Clock, ssoConnectorID string, srcURL string) userConverter {
	return func(oktaUser *okta.User) (types.User, error) {
		if oktaUser == nil {
			return nil, trace.BadParameter("oktaUser must not be nil")
		}

		profile, err := parseOktaUserProfile(oktaUser)
		if err != nil {
			return nil, trace.Wrap(err, "decoding Okta user profile")
		}

		newUser, err := types.NewUser(profile.Login)
		if err != nil {
			return nil, trace.Wrap(err, "processing okta user %s", profile.Login)
		}

		newUser.SetStaticLabels(map[string]string{
			types.OriginLabel:         types.OriginOkta,
			eteleport.OktaOrgURLLabel: srcURL,
			eteleport.OktaUserIDLabel: oktaUser.Id,
		})
		newUser.AddRole(teleport.PresetRequesterRoleName)

		traits := profile.AsTraits()
		newUser.SetTraits(traits)

		newUser.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{
				Name: teleport.UserSystem,
			},
			Time: clock.Now(),
			Connector: &types.ConnectorRef{
				ID:       ssoConnectorID,
				Type:     constants.SAML,
				Identity: oktaUser.Id,
			},
		})

		return newUser, nil
	}
}
