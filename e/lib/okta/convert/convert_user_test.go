package oktaconvert

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

func Test_ConvertOktaOrgUser(t *testing.T) {
	const (
		loginName  = "scooby@the-mystery-machine.com"
		oktaUserID = "SCOOBY"
		oktaOrgURL = "https://example.okta.com"
	)

	oktaUser := &okta.User{
		Id:     oktaUserID,
		Status: "ACTIVE",
		Profile: &okta.UserProfile{
			"firstName": "Scoobert",
			"lastName":  "Doo",
			"nickName":  "Scooby",
			"login":     loginName,

			// not sure Okta profile elements can be anything other than strings,
			// but let's throw in a list of strings just to ensure we can handle
			// it
			"email": []string{loginName, "SnackFan0154@hotmail.com"},
		},
	}

	ctime := time.Date(1986, time.August, 6, 23, 58, 0, 0, time.UTC)

	t.Run("default", func(t *testing.T) {
		teleportUser, err := ConvertOktaOrgUser(ConvertOktaUserArgs[*okta.User]{
			Clock:              clockwork.NewFakeClockAt(ctime),
			SAMLConnectorName:  "sso-connector",
			OktaOrgURL:         oktaOrgURL,
			OktaSDKUser:        oktaUser,
			AssignDefaultRoles: true,
		})
		require.NoError(t, err)
		require.Equal(t, loginName, teleportUser.GetName())

		// Expect that the user is marked as an Okta-sourced user
		labels := teleportUser.GetMetadata().Labels
		require.Equal(t, types.OriginOkta, labels[types.OriginLabel])
		require.Equal(t, oktaOrgURL, labels[eteleport.OktaOrgURLLabel])
		require.Equal(t, oktaUserID, labels[eteleport.OktaUserIDLabel])

		// Expect that the user has been given the requester role by default
		require.Equal(t, []string{teleport.SystemOktaRequesterRoleName}, teleportUser.GetRoles())

		// Expect that the okta user profile has been converted to traits
		traits := teleportUser.GetTraits()
		require.Equal(t, []string{"Scoobert"}, traits["okta/firstName"])
		require.Equal(t, []string{"Doo"}, traits["okta/lastName"])
		require.Equal(t, []string{"Scooby"}, traits["okta/nickName"])
		require.ElementsMatch(t, []string{loginName, "SnackFan0154@hotmail.com"},
			traits["okta/email"])

		// Expect that the well-known profile elements have been filtered out...
		require.NotContains(t, traits, "okta/login")

		// Expect that the creation date has been set
		creator := teleportUser.GetCreatedBy()
		require.Equal(t, ctime, creator.Time.UTC())
	})

	t.Run("DisableOktaRequesterRoleAssignment", func(t *testing.T) {
		teleportUser, err := ConvertOktaOrgUser(ConvertOktaUserArgs[*okta.User]{
			Clock:              clockwork.NewFakeClockAt(ctime),
			SAMLConnectorName:  "sso-connector",
			OktaOrgURL:         oktaOrgURL,
			OktaSDKUser:        oktaUser,
			AssignDefaultRoles: false,
		})
		require.NoError(t, err)
		require.Empty(t, teleportUser.GetRoles())
	})
}

func TestAppUserConversion(t *testing.T) {
	const (
		loginName        = "scooby@the-mystery-machine.com"
		oktaUserID       = "SCOOBY"
		oktaOrgURL       = "https://example.okta.com"
		okaSAMLConnector = "saml-connector"
	)

	oktaAppUser := &okta.AppUser{
		Id:         oktaUserID,
		ExternalId: "eid+" + loginName,
		Scope:      "USER",
		Status:     "PROVISIONED",
		SyncState:  "SYNCHRONIZED",
		Credentials: &okta.AppUserCredentials{
			UserName: "uid+" + loginName,
		},
		Profile: map[string]any{
			"firstName": "Scoobert",
			"lastName":  "Doo",
			"nickName":  "Scooby",
			"login":     loginName,

			// not sure Okta profile elements can be anything other than strings,
			// but let's throw in a list of strings just to ensure we can handle
			// it
			"email": []string{loginName, "SnackFan0154@hotmail.com"},
		},
	}

	ctime := time.Date(1986, time.August, 6, 23, 58, 0, 0, time.UTC)

	t.Run("default", func(t *testing.T) {
		teleportUser, err := ConvertOktaAppUser(ConvertOktaUserArgs[*okta.AppUser]{
			Clock:              clockwork.NewFakeClockAt(ctime),
			SAMLConnectorName:  okaSAMLConnector,
			OktaOrgURL:         oktaOrgURL,
			OktaSDKUser:        oktaAppUser,
			AssignDefaultRoles: true,
		})
		require.NoError(t, err)
		require.Equal(t, "uid+"+loginName, teleportUser.GetName())

		// Expect that the user is marked as an Okta-sourced user
		labels := teleportUser.GetMetadata().Labels
		require.Equal(t, types.OriginOkta, labels[types.OriginLabel])
		require.Equal(t, oktaOrgURL, labels[eteleport.OktaOrgURLLabel])
		require.Equal(t, oktaUserID, labels[eteleport.OktaUserIDLabel])

		// Expect that the user has been given the requester role by default
		require.Equal(t, []string{teleport.SystemOktaRequesterRoleName}, teleportUser.GetRoles())

		// Expect that the okta user profile has been converted to traits
		traits := teleportUser.GetTraits()
		require.Equal(t, []string{"Scoobert"}, traits["okta/firstName"])
		require.Equal(t, []string{"Doo"}, traits["okta/lastName"])
		require.Equal(t, []string{"Scooby"}, traits["okta/nickName"])
		require.ElementsMatch(t, []string{loginName, "SnackFan0154@hotmail.com"},
			traits["okta/email"])

		// Expect that the well-known profile elements have been filtered out...
		require.NotContains(t, traits, "okta/login")

		// Expect that the creation date has been set
		creator := teleportUser.GetCreatedBy()
		require.Equal(t, ctime, creator.Time.UTC())
	})

	t.Run("DisableOktaRequesterRoleAssignment", func(t *testing.T) {
		teleportUser, err := ConvertOktaAppUser(ConvertOktaUserArgs[*okta.AppUser]{
			Clock:              clockwork.NewFakeClockAt(ctime),
			SAMLConnectorName:  okaSAMLConnector,
			OktaOrgURL:         oktaOrgURL,
			OktaSDKUser:        oktaAppUser,
			AssignDefaultRoles: false,
		})
		require.NoError(t, err)
		require.Empty(t, teleportUser.GetRoles())
	})
}
