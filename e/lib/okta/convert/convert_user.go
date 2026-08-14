package oktaconvert

import (
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mitchellh/mapstructure"
	oktasdk "github.com/okta/okta-sdk-golang/v2/okta"

	ossteleport "github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/trait"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

type ConvertOktaUserArgs[T comparable] struct {
	Clock              clockwork.Clock
	SAMLConnectorName  string
	OktaOrgURL         string
	OktaSDKUser        T
	AssignDefaultRoles bool
}

func (args *ConvertOktaUserArgs[T]) CheckAndSetDefaults() error {
	if args.Clock == nil {
		args.Clock = clockwork.NewRealClock()
	}
	if args.SAMLConnectorName == "" {
		return trace.BadParameter("missing SAMLConnectorName")
	}
	if args.OktaOrgURL == "" {
		return trace.BadParameter("missing Okta org URL")
	}
	var zeroUser T
	if args.OktaSDKUser == zeroUser {
		return trace.BadParameter("missing Okta SDK user")
	}
	return nil
}

// ConvertOktaAppUser converts an Okta SDK AppUser profile into a Teleport user.
func ConvertOktaAppUser(args ConvertOktaUserArgs[*oktasdk.AppUser]) (types.User, error) {
	if err := args.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	appUser := args.OktaSDKUser
	if appUser == nil {
		return nil, trace.BadParameter("Okta app user must not be nil")
	}
	profile, ok := appUser.Profile.(map[string]any)
	if !ok {
		return nil, trace.BadParameter("invalid type for Okta app user profile: %T", args.OktaSDKUser.Profile)
	}

	u, err := NewTeleportUser(NewTeleportUserArgs{
		Clock:              args.Clock,
		SAMLConnectorName:  args.SAMLConnectorName,
		OktaOrgURL:         args.OktaOrgURL,
		OktaLogin:          appUser.Credentials.UserName,
		OktaID:             appUser.Id,
		OktaStatus:         appUser.Status,
		OktaProfile:        profile,
		AssignDefaultRoles: args.AssignDefaultRoles,
	})
	return u, trace.Wrap(err)
}

// ConvertOktaAppUser converts an Okta SDK User profile into a Teleport user.
func ConvertOktaOrgUser(args ConvertOktaUserArgs[*oktasdk.User]) (types.User, error) {
	if err := args.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	orgUser := args.OktaSDKUser
	if orgUser == nil {
		return nil, trace.BadParameter("Okta org user must not be nil")
	}
	if orgUser.Profile == nil {
		return nil, trace.BadParameter("missing Okta org user profile")
	}
	profile := *orgUser.Profile

	u, err := NewTeleportUser(NewTeleportUserArgs{
		Clock:              args.Clock,
		SAMLConnectorName:  args.SAMLConnectorName,
		OktaOrgURL:         args.OktaOrgURL,
		OktaLogin:          "", // will be taken from the profile
		OktaID:             orgUser.Id,
		OktaStatus:         orgUser.Status,
		OktaProfile:        profile,
		AssignDefaultRoles: args.AssignDefaultRoles,
	})
	return u, trace.Wrap(err)
}

type NewTeleportUserArgs struct {
	Clock             clockwork.Clock
	SAMLConnectorName string
	OktaOrgURL        string
	OktaLogin         string
	// OktaID is the value of the created user's teleport.internal/okta-user-id label.
	OktaID string
	// IgnoreOktaID when true, causes to ignore the OktaID value and does not set the
	// teleport.internal/okta-user-id label on the created user.
	IgnoreOktaID bool
	// OktaStatus is the value of the created user's teleport.internal/okta-user-status label.
	OktaStatus string
	// IgnoreOktaStatus when true, causes to ignore the OktaStatus value and does not set the
	// teleport.internal/okta-user-status label.
	IgnoreOktaStatus   bool
	OktaProfile        map[string]any
	AssignDefaultRoles bool
}

func (args *NewTeleportUserArgs) CheckAndSetDefaults() error {
	if args.Clock == nil {
		args.Clock = clockwork.NewRealClock()
	}
	if args.SAMLConnectorName == "" {
		return trace.BadParameter("missing SAMLConnectorName")
	}
	if args.OktaOrgURL == "" {
		return trace.BadParameter("missing Okta org URL")
	}
	if args.OktaLogin == "" {
		return trace.BadParameter("missing Okta user login")
	}
	if !args.IgnoreOktaID && args.OktaID == "" {
		return trace.BadParameter("missing Okta user ID")
	}
	if !args.IgnoreOktaStatus && args.OktaStatus == "" {
		return trace.BadParameter("missing Okta user status")
	}
	if args.OktaProfile == nil {
		return trace.BadParameter("missing Okta user profile")
	}
	return nil
}

// NewTeleportUser creates a in-memory Teleport user resource with the appropriate labels, traits
// and properties to mark the user as belonging to an Okta organization.  If the supplied login is
// empty,  the `login` attribute from the user's profile will be used.
func NewTeleportUser(args NewTeleportUserArgs) (types.User, error) {
	profile, err := newOktaUserProfile(args.OktaProfile)
	if err != nil {
		return nil, trace.Wrap(err, "parsing okta user profile")
	}

	if args.OktaLogin == "" {
		args.OktaLogin = profile.Login
	}
	if err := args.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "creating okta user")
	}

	newUser, err := types.NewUser(args.OktaLogin)
	if err != nil {
		return nil, trace.Wrap(err, "processing okta user %s", args.OktaLogin)
	}

	staticLabels := map[string]string{
		types.OriginLabel:         types.OriginOkta,
		eteleport.OktaOrgURLLabel: args.OktaOrgURL,
	}
	if !args.IgnoreOktaID {
		staticLabels[eteleport.OktaUserIDLabel] = args.OktaID
	}
	if !args.IgnoreOktaStatus {
		staticLabels[eteleport.OktaUserStatusLabel] = args.OktaStatus
	}
	newUser.SetStaticLabels(staticLabels)

	if args.AssignDefaultRoles {
		newUser.AddRole(ossteleport.SystemOktaRequesterRoleName)
	}

	newUser.SetTraits(profile.AsTraits())

	newUser.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{
			Name: ossteleport.UserSystem,
		},
		Time: args.Clock.Now(),
		Connector: &types.ConnectorRef{
			ID:       args.SAMLConnectorName,
			Type:     constants.SAML,
			Identity: args.OktaID,
		},
	})

	return newUser, nil
}

type oktaUserProfile struct {
	Login  string         `mapstructure:"login"`
	Fields map[string]any `mapstructure:",remain"`
}

func newOktaUserProfile(rawProfile map[string]any) (oktaUserProfile, error) {
	var profile oktaUserProfile
	// TODO(kopiczko) don't use mapstructure here, it can be done with simple map operations
	err := mapstructure.Decode(&rawProfile, &profile)
	if err != nil {
		return oktaUserProfile{}, trace.Wrap(err)
	}
	return profile, nil
}

func (p oktaUserProfile) AsTraits() trait.Traits {
	traits := trait.Traits{}
	for k, v := range p.Fields {
		switch value := v.(type) {
		case string:
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				traits[eteleport.OktaTraitPrefix+k] = []string{trimmed}
			}
		case []string:
			if trimmed := removeEmpty(value); len(trimmed) > 0 {
				traits[eteleport.OktaTraitPrefix+k] = trimmed
			}
		}
	}
	return traits
}

func removeEmpty(src []string) []string {
	return slices.DeleteFunc(src, func(s string) bool { return len(strings.TrimSpace(s)) == 0 })
}
