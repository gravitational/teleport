package okta

import (
	"github.com/gravitational/teleport/e/lib/okta/api"
)

type userName = api.UserName

type oktaAppID = api.OktaAppID

type oktaUserID = api.OktaUserID

type oktaGroupID = api.OktaGroupID

type testOktaClient = api.TestOktaClient

var newTestClient = api.NewTestClient
