package okta

import (
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
)

type userName = oktaapi.UserName

type oktaAppID = oktaapi.OktaAppID

type oktaUserID = oktaapi.OktaUserID

type oktaGroupID = oktaapi.OktaGroupID

type testOktaClient = oktaapi.TestOktaClient

var newTestClient = oktaapi.NewTestClient
