package auth

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudv1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
)

type mockAuthorizer struct {
	authorize func(ctx context.Context) (*authz.Context, error)
}

func (m *mockAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	if m.authorize != nil {
		return m.authorize(ctx)
	}
	return nil, trace.NotImplemented("Authorize not implemented")
}

type mockEmitter struct {
	// in is the event param used in the last EmitAuditEvent call.
	in events.AuditEvent
}

// EmitAuditEvent stores `event` into the `in` property.
func (m *mockEmitter) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	m.in = event
	return nil
}

// cloudWithRolesTestSuite sets up a testing environment for methods in
// cloudWithRoles. Note that updating mock pointers after initialization
// has no effect. Instead, modify their internal values directly.
//
// For example, use:
//
//	suite.cloudClient.MockDeleteContact = func(...){ }
//
// Instead of:
//
//	suite.cloudClient = &cloud.MockedClient{MockDeleteContact: func(...){ }
type cloudWithRolesTestSuite struct {
	// cloudWithRoles is a pointer to the cloudWithRoles struct.
	cloudWithRoles *cloudWithRoles
	// cloudClient is a mocked cloud client that can be used
	// to overwrite the cloudWithRoles tenants API call responses.
	cloudClient *cloud.MockedClient
	// authorizer is a mockAuthorizer that can be used to overwrite the
	// `Authorize` call during authentication checks.
	authorizer *mockAuthorizer
	// emitter is a mockEmitter that implements the `events.Emitter` interface and
	// stores that latest emitted event in its `in` value.
	emitter *mockEmitter
	// authIdentity is the identity getter that will be used during Authorize() calls.
	authIdentity authz.IdentityGetter
}

// newCloudSuite configures and returns a new cloudWithRolesTestSuite.
// By default, the cloud client has no predefined mocks. Methods used must
// be implemented by the caller. The authorizer always approves requests
// for the provided identity, and a default mockEmitter is used.
func newCloudSuite(t *testing.T) cloudWithRolesTestSuite {
	authPlugin, err := NewPlugin(Config{
		License: ValidLicense{},
		Modules: modulestest.EnterpriseModules(),
	})
	require.NoError(t, err)

	cloudClient := &cloud.MockedClient{}
	authPlugin.cloudClient = cloudClient

	authPlugin.authServer = &auth.GRPCServer{}
	emitter := &mockEmitter{}
	authPlugin.authServer.Emitter = emitter

	roles := auth.GetPresetRoles(modules.BuildEnterprise)
	rolesNames := make([]string, len(roles))
	for i, r := range roles {
		rolesNames[i] = r.GetName()
	}

	authIdentity := authtest.TestAdmin().I
	authorizer := &mockAuthorizer{
		authorize: func(ctx context.Context) (*authz.Context, error) {
			return &authz.Context{
				User:     newUser(t, "testuser", types.UserTypeLocal, rolesNames...),
				Checker:  services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", auth.GetPresetRoles(modules.BuildEnterprise)),
				Identity: authIdentity,
			}, nil
		},
	}
	authPlugin.authServer.Authorizer = authorizer

	ac := cloudWithRoles{}
	ac.plugin = authPlugin

	return cloudWithRolesTestSuite{
		cloudWithRoles: &ac,
		cloudClient:    cloudClient,
		authorizer:     authorizer,
		emitter:        emitter,
		authIdentity:   authIdentity,
	}
}

// unauthorized is a function that can be used as an Authorize implementation
// and always return an AccessDenied error
func unauthorized(ctx context.Context) (*authz.Context, error) {
	return nil, trace.AccessDenied("unauthorized")
}

// newTestContact returns a *cloudv1.Contact with valid fields,
// to be used in tests.
func newTestContact() *cloudv1.Contact {
	return &cloudv1.Contact{
		Name:        "contactname",
		AccountId:   "accid",
		VerifyToken: "verifytoken",
		Email:       "email@goteleport.com",
		ContactType: 1,
		Verified:    true,
	}
}

func TestRemoveContact(t *testing.T) {
	ctx := context.Background()
	removeContactResp := &cloudv1.RemoveContactResponse{
		Contact: newTestContact(),
	}
	suite := newCloudSuite(t)
	suite.cloudClient.MockRemoveContact = func(ctx context.Context, req *cloudv1.RemoveContactRequest, opts ...grpc.CallOption) (*cloudv1.RemoveContactResponse, error) {
		return removeContactResp, nil
	}
	originalSuiteAuthorize := suite.authorizer.authorize

	tt := []struct {
		name      string
		authorize bool
		req       *cloudv1.RemoveContactRequest
		assert    func(t *testing.T, resp *cloudv1.RemoveContactResponse, err error, event events.AuditEvent)
	}{
		{
			name:      "unauthorized request should emit no events",
			authorize: false,
			req:       &cloudv1.RemoveContactRequest{},
			assert: func(t *testing.T, resp *cloudv1.RemoveContactResponse, err error, event events.AuditEvent) {
				require.True(t, trace.IsAccessDenied(err))
				require.Nil(t, event)
			},
		},
		{
			name:      "successful request should emit events",
			authorize: true,
			req: &cloudv1.RemoveContactRequest{
				ContactType: 2,
			},
			assert: func(t *testing.T, resp *cloudv1.RemoveContactResponse, err error, event events.AuditEvent) {
				require.NoError(t, err)
				require.Equal(t, removeContactResp, resp)
				// check event
				require.NotNil(t, event)
				removeEvent := event.(*events.ContactDelete)
				require.Equal(t, removeContactResp.Contact.Email, removeEvent.Email)
				require.Equal(t, events.ContactType(2), removeEvent.ContactType) // should match request
				require.Equal(t, libevents.ContactDeleteEvent, removeEvent.Type)
				require.Equal(t, libevents.ContactDeleteCode, removeEvent.Code)
				require.True(t, removeEvent.Status.Success)
				require.Empty(t, removeEvent.Status.Error)
				require.Equal(t, types.KindContact, removeEvent.ResourceMetadata.Name)
				require.Equal(t, suite.authIdentity.GetIdentity().Username, removeEvent.ResourceMetadata.UpdatedBy)
			},
		},
	}

	for _, tc := range tt {
		// overwrite test suite authorizer if test case tests non-authorized requests
		if tc.authorize {
			suite.authorizer.authorize = originalSuiteAuthorize
		} else {
			suite.authorizer.authorize = unauthorized
		}
		resp, err := suite.cloudWithRoles.RemoveContact(ctx, tc.req)
		tc.assert(t, resp, err, suite.emitter.in)
	}
}

func TestCreateContact(t *testing.T) {
	ctx := context.Background()
	createContactResp := &cloudv1.CreateContactResponse{
		Contact: newTestContact(),
	}
	suite := newCloudSuite(t)
	suite.cloudClient.MockCreateContact = func(ctx context.Context, req *cloudv1.CreateContactRequest, opts ...grpc.CallOption) (*cloudv1.CreateContactResponse, error) {
		return createContactResp, nil
	}
	originalSuiteAuthorize := suite.authorizer.authorize

	tt := []struct {
		name      string
		authorize bool
		req       *cloudv1.CreateContactRequest
		assert    func(t *testing.T, resp *cloudv1.CreateContactResponse, err error, event events.AuditEvent)
	}{
		{
			name:      "unauthorized request should emit no events",
			authorize: false,
			req:       &cloudv1.CreateContactRequest{},
			assert: func(t *testing.T, resp *cloudv1.CreateContactResponse, err error, event events.AuditEvent) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
				require.Nil(t, event)
			},
		},
		{
			name:      "successful request should emit events",
			authorize: true,
			req: &cloudv1.CreateContactRequest{
				Email:       "email@goteleport.com",
				ContactType: 2,
			},
			assert: func(t *testing.T, resp *cloudv1.CreateContactResponse, err error, event events.AuditEvent) {
				require.NoError(t, err)
				require.Equal(t, createContactResp, resp)
				// check event
				require.NotNil(t, event)
				createEvent := event.(*events.ContactCreate)
				require.Equal(t, "email@goteleport.com", createEvent.Email)      // should match request
				require.Equal(t, events.ContactType(2), createEvent.ContactType) // should match request
				require.Equal(t, libevents.ContactCreateEvent, createEvent.Type)
				require.Equal(t, libevents.ContactCreateCode, createEvent.Code)
				require.True(t, createEvent.Status.Success)
				require.Empty(t, createEvent.Status.Error)
				require.Equal(t, types.KindContact, createEvent.ResourceMetadata.Name)
				require.Equal(t, suite.authIdentity.GetIdentity().Username, createEvent.ResourceMetadata.UpdatedBy)
			},
		},
	}

	for _, tc := range tt {
		// overwrite test suite authorizer if test case tests non-authorized requests
		if tc.authorize {
			suite.authorizer.authorize = originalSuiteAuthorize
		} else {
			suite.authorizer.authorize = unauthorized
		}

		resp, err := suite.cloudWithRoles.CreateContact(ctx, tc.req)
		tc.assert(t, resp, err, suite.emitter.in)
	}
}

type mockChecker struct {
	services.AccessChecker
	allowedVerbs []string
}

func (m mockChecker) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string) error {
	if !slices.Contains(m.allowedVerbs, verb) {
		return trace.BadParameter("verb %s not allowed", verb)
	}
	return nil
}

func TestAction(t *testing.T) {
	suite := newCloudSuite(t)

	ctx := t.Context()
	tt := []struct {
		name              string
		actions           []string
		authorizedActions []string
		cloudEnabled      bool
		assert            require.ErrorAssertionFunc
	}{
		{
			name:              "ok: single verb",
			actions:           []string{types.VerbRead},
			authorizedActions: []string{types.VerbRead},
			cloudEnabled:      true,
			assert:            require.NoError,
		},
		{
			name:    "all verbs allowed",
			actions: []string{types.VerbRead, types.VerbCreate, types.VerbUpdate},

			authorizedActions: []string{types.VerbRead, types.VerbCreate, types.VerbUpdate},
			cloudEnabled:      true,
			assert:            require.NoError,
		},
		{
			name:              "first verb not allowed",
			actions:           []string{types.VerbRead},
			authorizedActions: nil,
			cloudEnabled:      true,
			assert:            require.Error,
		},
		{
			name:    "extras verb not allowed",
			actions: []string{types.VerbRead, types.VerbCreate, types.VerbUpdate},

			authorizedActions: []string{types.VerbRead, types.VerbCreate},
			cloudEnabled:      true,
			assert:            require.Error,
		},
		{
			name:         "cloud disabled short-circuits",
			actions:      []string{types.VerbList},
			cloudEnabled: false,
			assert:       require.Error,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ac := suite.cloudWithRoles
			mockChecker := mockChecker{
				allowedVerbs: tc.authorizedActions,
			}
			suite.authorizer.authorize = func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					User:     newUser(t, "testuser", types.UserTypeLocal),
					Checker:  mockChecker,
					Identity: authtest.TestAdmin().I,
				}, nil
			}
			ac.action(ctx, "some-resource", tc.actions...)
		})
	}
}

func TestEmitClientIPRestrictionstAuditEvent(t *testing.T) {
	suite := newCloudSuite(t)
	ctx := t.Context()

	tt := []struct {
		name    string
		resp    *cloudv1.PutClientIPRestrictionsResponse
		respErr error
		assert  func(t *testing.T, event events.AuditEvent, err error)
	}{
		{
			name: "success with multiple cidrs",
			resp: &cloudv1.PutClientIPRestrictionsResponse{
				ClientIpRestrictions: []*cloudv1.CIDR{
					{Cidr: "10.0.0.0/24"},
					{Cidr: "192.168.1.0/24"},
					{Cidr: "2001:db8::/32"},
				},
			},
			assert: func(t *testing.T, event events.AuditEvent, err error) {
				require.NoError(t, err)
				require.NotNil(t, event)

				ev, ok := event.(*events.ClientIPRestrictionsUpdate)
				require.True(t, ok, "wrong event type")
				require.Equal(t, libevents.ClientIPRestrictionsUpdateCode, ev.Metadata.Code)
				require.True(t, ev.Status.Success)
				require.Empty(t, ev.Status.Error)
				require.Empty(t, ev.Status.UserMessage)

				require.ElementsMatch(t, []string{
					"10.0.0.0/24", "192.168.1.0/24", "2001:db8::/32",
				}, ev.ClientIPRestrictions)

				require.Equal(t, types.KindClientIPRestriction, ev.ResourceMetadata.Name)
			},
		},
		{
			name: "success with empty response",
			resp: nil,
			assert: func(t *testing.T, event events.AuditEvent, err error) {
				require.NoError(t, err)
				require.NotNil(t, event)

				ev, ok := event.(*events.ClientIPRestrictionsUpdate)
				require.True(t, ok)
				require.True(t, ev.Status.Success)
				require.Empty(t, ev.ClientIPRestrictions)
			},
		},
		{
			name: "error does not log input even if resp present",
			resp: &cloudv1.PutClientIPRestrictionsResponse{
				ClientIpRestrictions: []*cloudv1.CIDR{
					{Cidr: "1.1.1.0/24"},
					{Cidr: "2.2.2.0/24"},
					{Cidr: "3.3.3.0/24"},
				},
			},
			respErr: trace.Wrap(trace.BadParameter("invalid CIDR payload")),
			assert: func(t *testing.T, event events.AuditEvent, err error) {
				// Should not error when emitting an error event
				require.NoError(t, err)
				require.NotNil(t, event)

				ev, ok := event.(*events.ClientIPRestrictionsUpdate)
				require.True(t, ok)
				require.False(t, ev.Status.Success)

				require.Contains(t, ev.Status.Error, "invalid CIDR payload")
				require.Contains(t, ev.Status.UserMessage, "invalid CIDR payload")

				require.Empty(t, ev.ClientIPRestrictions, "must not log client IP restrictions on error")
			},
		},
		{
			name:    "error nil resp emits event without cidrs",
			resp:    nil,
			respErr: trace.AccessDenied("no permission"),
			assert: func(t *testing.T, event events.AuditEvent, err error) {
				require.NoError(t, err)
				require.NotNil(t, event)

				ev, ok := event.(*events.ClientIPRestrictionsUpdate)
				require.True(t, ok)
				require.False(t, ev.Status.Success)
				require.Empty(t, ev.ClientIPRestrictions)

				require.Contains(t, ev.Status.UserMessage, "no permission")
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ac := suite.cloudWithRoles
			err := ac.emitClientIPRestrictionstAuditEvent(ctx, tc.resp, tc.respErr)
			tc.assert(t, suite.emitter.in, err)
		})
	}
}

func TestUpdateEnvironmentProfile(t *testing.T) {
	ctx := t.Context()
	suite := newCloudSuite(t)
	updateResp := &cloudv1.GetEnvironmentProfileResponse{EnvironmentProfile: "staging"}
	suite.cloudClient.MockUpdateEnvironmentProfile = func(in *cloudv1.UpdateEnvironmentProfileRequest) (*cloudv1.GetEnvironmentProfileResponse, error) {
		return updateResp, nil
	}
	originalSuiteAuthorize := suite.authorizer.authorize

	tt := []struct {
		name      string
		authorize bool
		req       *cloudv1.UpdateEnvironmentProfileRequest
		assert    func(t *testing.T, resp *cloudv1.GetEnvironmentProfileResponse, err error, event events.AuditEvent)
	}{
		{
			name:      "unauthorized request should emit no events",
			authorize: false,
			req:       &cloudv1.UpdateEnvironmentProfileRequest{EnvironmentProfile: "staging"},
			assert: func(t *testing.T, resp *cloudv1.GetEnvironmentProfileResponse, err error, event events.AuditEvent) {
				require.True(t, trace.IsAccessDenied(err))
				require.Nil(t, event)
			},
		},
		{
			name:      "successful request should emit events",
			authorize: true,
			req:       &cloudv1.UpdateEnvironmentProfileRequest{EnvironmentProfile: "staging"},
			assert: func(t *testing.T, resp *cloudv1.GetEnvironmentProfileResponse, err error, event events.AuditEvent) {
				require.NoError(t, err)
				require.Equal(t, updateResp, resp)
				// check event
				require.NotNil(t, event)
				ev, ok := event.(*events.EnvironmentProfileUpdate)
				require.True(t, ok, "wrong event type (%T)", event)
				require.Equal(t, libevents.EnvironmentProfileUpdateEvent, ev.Type)
				require.Equal(t, libevents.EnvironmentProfileUpdatedCode, ev.Code)
				require.Equal(t, "staging", ev.EnvironmentProfileMetadata.EnvironmentProfile)
				require.Equal(t, suite.authIdentity.GetIdentity().Username, ev.UserMetadata.User)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			// overwrite test suite authorizer if test case tests non-authorized requests
			if tc.authorize {
				suite.authorizer.authorize = originalSuiteAuthorize
			} else {
				suite.authorizer.authorize = unauthorized
			}
			suite.emitter.in = nil
			resp, err := suite.cloudWithRoles.UpdateEnvironmentProfile(ctx, tc.req)
			tc.assert(t, resp, err, suite.emitter.in)
		})
	}
}

func newUser(t *testing.T, name string, userType types.UserType, roles ...string) types.User {
	t.Helper()

	user, err := types.NewUser(name)
	require.NoError(t, err)

	if userType == types.UserTypeSSO {
		user.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				Type: "dummy",
			},
		})
	}

	for _, role := range roles {
		user.AddRole(role)
	}

	return user
}

func newAccessList(t *testing.T, name string, roleGrants []string) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Title: "title",
			Owners: []accesslist.Owner{
				{
					Name: "test-user1",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: time.Now().Add(365 * 24 * time.Hour),
			},
			MembershipRequires: accesslist.Requires{},
			OwnershipRequires:  accesslist.Requires{},
			Grants: accesslist.Grants{
				Roles: roleGrants,
			},
		},
	)
	require.NoError(t, err)
	return accessList
}

func TestGetMAUDailyBreakdown(t *testing.T) {
	ctx := t.Context()
	suite := newCloudSuite(t)
	suite.cloudClient.MockGetMAUDailyBreakdown = func(ctx context.Context, req *cloudv1.GetMAUDailyBreakdownRequest, opts ...grpc.CallOption) (*cloudv1.GetMAUDailyBreakdownResponse, error) {
		return &cloudv1.GetMAUDailyBreakdownResponse{}, nil
	}

	tt := []struct {
		name       string
		allowRules []types.Rule
		assert     require.ErrorAssertionFunc
	}{
		{
			name:       "no billing read permission",
			allowRules: []types.Rule{},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsAccessDenied(err))
			},
		},
		{
			name:       "with billing read permission",
			allowRules: []types.Rule{{Resources: []string{types.KindBilling}, Verbs: []string{types.VerbRead}}},
			assert:     require.NoError,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			role, err := types.NewRole("test-role", types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: tc.allowRules},
			})
			require.NoError(t, err)
			suite.authorizer.authorize = func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					User:     newUser(t, "testuser", types.UserTypeLocal, role.GetName()),
					Checker:  services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", services.RoleSet{role}),
					Identity: suite.authIdentity,
				}, nil
			}
			_, err = suite.cloudWithRoles.GetMAUDailyBreakdown(ctx, &cloudv1.GetMAUDailyBreakdownRequest{})
			tc.assert(t, err)
		})
	}
}

func TestGetTPRDailyBreakdown(t *testing.T) {
	ctx := t.Context()
	suite := newCloudSuite(t)
	suite.cloudClient.MockGetTPRDailyBreakdown = func(ctx context.Context, req *cloudv1.GetTPRDailyBreakdownRequest, opts ...grpc.CallOption) (*cloudv1.GetTPRDailyBreakdownResponse, error) {
		return &cloudv1.GetTPRDailyBreakdownResponse{}, nil
	}

	tt := []struct {
		name       string
		allowRules []types.Rule
		assert     require.ErrorAssertionFunc
	}{
		{
			name:       "no billing read permission",
			allowRules: []types.Rule{},
			assert: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsAccessDenied(err))
			},
		},
		{
			name:       "with billing read permission",
			allowRules: []types.Rule{{Resources: []string{types.KindBilling}, Verbs: []string{types.VerbRead}}},
			assert:     require.NoError,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			role, err := types.NewRole("test-role", types.RoleSpecV6{
				Allow: types.RoleConditions{Rules: tc.allowRules},
			})
			require.NoError(t, err)
			suite.authorizer.authorize = func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					User:     newUser(t, "testuser", types.UserTypeLocal, role.GetName()),
					Checker:  services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", services.RoleSet{role}),
					Identity: suite.authIdentity,
				}, nil
			}
			_, err = suite.cloudWithRoles.GetTPRDailyBreakdown(ctx, &cloudv1.GetTPRDailyBreakdownRequest{})
			tc.assert(t, err)
		})
	}
}

// TestCloudWithRoles_StripeRPCs verifies that every Stripe RPC wrapper on
// cloudWithRoles gates on KindBilling with the expected verb (Read for GETs,
// Update for mutations) and forwards to the cloud client on success.
func TestCloudWithRoles_StripeRPCs(t *testing.T) {
	ctx := t.Context()
	suite := newCloudSuite(t)

	tt := []struct {
		name    string
		verb    string
		setMock func(c *cloud.MockedClient)
		call    func(ac *cloudWithRoles) error
	}{
		{
			name: "GetStripeConfig",
			verb: types.VerbRead,
			setMock: func(c *cloud.MockedClient) {
				c.MockGetStripeConfig = func(ctx context.Context, in *cloudv1.GetStripeConfigRequest, opts ...grpc.CallOption) (*cloudv1.GetStripeConfigResponse, error) {
					return &cloudv1.GetStripeConfigResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.GetStripeConfig(ctx, &cloudv1.GetStripeConfigRequest{})
				return err
			},
		},
		{
			name: "StripeListCards",
			verb: types.VerbRead,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeListCards = func(ctx context.Context, in *cloudv1.StripeListCardsRequest, opts ...grpc.CallOption) (*cloudv1.StripeListCardsResponse, error) {
					return &cloudv1.StripeListCardsResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeListCards(ctx, &cloudv1.StripeListCardsRequest{})
				return err
			},
		},
		{
			name: "StripeListInvoices",
			verb: types.VerbRead,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeListInvoices = func(ctx context.Context, in *cloudv1.StripeListInvoicesRequest, opts ...grpc.CallOption) (*cloudv1.StripeListInvoicesResponse, error) {
					return &cloudv1.StripeListInvoicesResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeListInvoices(ctx, &cloudv1.StripeListInvoicesRequest{})
				return err
			},
		},
		{
			name: "StripeGetSettings",
			verb: types.VerbRead,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeGetSettings = func(ctx context.Context, in *cloudv1.StripeGetSettingsRequest, opts ...grpc.CallOption) (*cloudv1.StripeGetSettingsResponse, error) {
					return &cloudv1.StripeGetSettingsResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeGetSettings(ctx, &cloudv1.StripeGetSettingsRequest{})
				return err
			},
		},
		{
			name: "StripeCreateSetupIntent",
			verb: types.VerbUpdate,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeCreateSetupIntent = func(ctx context.Context, in *cloudv1.StripeCreateSetupIntentRequest, opts ...grpc.CallOption) (*cloudv1.StripeCreateSetupIntentResponse, error) {
					return &cloudv1.StripeCreateSetupIntentResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeCreateSetupIntent(ctx, &cloudv1.StripeCreateSetupIntentRequest{})
				return err
			},
		},
		{
			name: "StripeCreateCard",
			verb: types.VerbUpdate,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeCreateCard = func(ctx context.Context, in *cloudv1.StripeCreateCardRequest, opts ...grpc.CallOption) (*cloudv1.StripeCreateCardResponse, error) {
					return &cloudv1.StripeCreateCardResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeCreateCard(ctx, &cloudv1.StripeCreateCardRequest{})
				return err
			},
		},
		{
			name: "StripeUpdateCard",
			verb: types.VerbUpdate,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeUpdateCard = func(ctx context.Context, in *cloudv1.StripeUpdateCardRequest, opts ...grpc.CallOption) (*cloudv1.StripeUpdateCardResponse, error) {
					return &cloudv1.StripeUpdateCardResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeUpdateCard(ctx, &cloudv1.StripeUpdateCardRequest{})
				return err
			},
		},
		{
			name: "StripeDeleteCard",
			verb: types.VerbUpdate,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeDeleteCard = func(ctx context.Context, in *cloudv1.StripeDeleteCardRequest, opts ...grpc.CallOption) (*cloudv1.StripeDeleteCardResponse, error) {
					return &cloudv1.StripeDeleteCardResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeDeleteCard(ctx, &cloudv1.StripeDeleteCardRequest{})
				return err
			},
		},
		{
			name: "StripeUpdateEmail",
			verb: types.VerbUpdate,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeUpdateEmail = func(ctx context.Context, in *cloudv1.StripeUpdateEmailRequest, opts ...grpc.CallOption) (*cloudv1.StripeUpdateEmailResponse, error) {
					return &cloudv1.StripeUpdateEmailResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeUpdateEmail(ctx, &cloudv1.StripeUpdateEmailRequest{})
				return err
			},
		},
		{
			name: "StripeUpdatePOPrefix",
			verb: types.VerbUpdate,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeUpdatePOPrefix = func(ctx context.Context, in *cloudv1.StripeUpdatePOPrefixRequest, opts ...grpc.CallOption) (*cloudv1.StripeUpdatePOPrefixResponse, error) {
					return &cloudv1.StripeUpdatePOPrefixResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeUpdatePOPrefix(ctx, &cloudv1.StripeUpdatePOPrefixRequest{})
				return err
			},
		},
		{
			name: "StripeUpdateStripeAddress",
			verb: types.VerbUpdate,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeUpdateStripeAddress = func(ctx context.Context, in *cloudv1.StripeUpdateStripeAddressRequest, opts ...grpc.CallOption) (*cloudv1.StripeUpdateStripeAddressResponse, error) {
					return &cloudv1.StripeUpdateStripeAddressResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeUpdateStripeAddress(ctx, &cloudv1.StripeUpdateStripeAddressRequest{})
				return err
			},
		},
		{
			name: "StripeCancel",
			verb: types.VerbUpdate,
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeCancel = func(ctx context.Context, in *cloudv1.StripeCancelRequest, opts ...grpc.CallOption) (*cloudv1.StripeCancelResponse, error) {
					return &cloudv1.StripeCancelResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeCancel(ctx, &cloudv1.StripeCancelRequest{})
				return err
			},
		},
	}

	// Verbs other than the one the RPC gates on should be rejected. Using
	// KindBilling but with the wrong verb ensures the check inspects the verb,
	// not just the kind.
	wrongVerb := func(rpcVerb string) string {
		if rpcVerb == types.VerbRead {
			return types.VerbUpdate
		}
		return types.VerbRead
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cases := []struct {
				name       string
				allowRules []types.Rule
				assert     require.ErrorAssertionFunc
			}{
				{
					name:       "no billing permission",
					allowRules: nil,
					assert: func(tt require.TestingT, err error, i ...interface{}) {
						require.True(tt, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
					},
				},
				{
					name:       "wrong verb rejected",
					allowRules: []types.Rule{{Resources: []string{types.KindBilling}, Verbs: []string{wrongVerb(tc.verb)}}},
					assert: func(tt require.TestingT, err error, i ...interface{}) {
						require.Error(tt, err)
					},
				},
				{
					name:       "wrong kind rejected",
					allowRules: []types.Rule{{Resources: []string{types.KindContact}, Verbs: []string{tc.verb}}},
					assert: func(tt require.TestingT, err error, i ...interface{}) {
						require.Error(tt, err)
					},
				},
				{
					name:       "billing " + tc.verb + " allowed",
					allowRules: []types.Rule{{Resources: []string{types.KindBilling}, Verbs: []string{tc.verb}}},
					assert:     require.NoError,
				},
			}

			tc.setMock(suite.cloudClient)

			for _, ac := range cases {
				t.Run(ac.name, func(t *testing.T) {
					role, err := types.NewRole("test-role", types.RoleSpecV6{
						Allow: types.RoleConditions{Rules: ac.allowRules},
					})
					require.NoError(t, err)
					suite.authorizer.authorize = func(ctx context.Context) (*authz.Context, error) {
						return &authz.Context{
							User:     newUser(t, "testuser", types.UserTypeLocal, role.GetName()),
							Checker:  services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", services.RoleSet{role}),
							Identity: suite.authIdentity,
						}, nil
					}

					ac.assert(t, tc.call(suite.cloudWithRoles))
				})
			}
		})
	}
}

// TestCloudWithRoles_StripeAuditEvents verifies that every mutating Stripe
// RPC wrapper on cloudWithRoles emits the correct billing audit event on
// success and emits nothing when the caller is not authorized.
func TestCloudWithRoles_StripeAuditEvents(t *testing.T) {
	ctx := t.Context()
	suite := newCloudSuite(t)
	originalAuthorize := suite.authorizer.authorize

	tt := []struct {
		name      string
		setMock   func(c *cloud.MockedClient)
		call      func(ac *cloudWithRoles) error
		assertEvt func(t *testing.T, ev events.AuditEvent)
	}{
		{
			name: "StripeCreateCard emits BillingCardCreate",
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeCreateCard = func(ctx context.Context, in *cloudv1.StripeCreateCardRequest, opts ...grpc.CallOption) (*cloudv1.StripeCreateCardResponse, error) {
					return &cloudv1.StripeCreateCardResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeCreateCard(ctx, &cloudv1.StripeCreateCardRequest{})
				return err
			},
			assertEvt: func(t *testing.T, ev events.AuditEvent) {
				e, ok := ev.(*events.BillingCardCreate)
				require.True(t, ok, "wrong event type (%T)", ev)
				require.Equal(t, libevents.BillingCardCreateEvent, e.Type)
				require.Equal(t, libevents.BillingCardCreateCode, e.Code)
				require.Equal(t, suite.authIdentity.GetIdentity().Username, e.UserMetadata.User)
			},
		},
		{
			name: "StripeUpdateCard emits BillingCardUpdate",
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeUpdateCard = func(ctx context.Context, in *cloudv1.StripeUpdateCardRequest, opts ...grpc.CallOption) (*cloudv1.StripeUpdateCardResponse, error) {
					return &cloudv1.StripeUpdateCardResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeUpdateCard(ctx, &cloudv1.StripeUpdateCardRequest{})
				return err
			},
			assertEvt: func(t *testing.T, ev events.AuditEvent) {
				// BillingCardUpdate reuses the BillingCardCreate proto shape.
				e, ok := ev.(*events.BillingCardCreate)
				require.True(t, ok, "wrong event type (%T)", ev)
				require.Equal(t, libevents.BillingCardUpdateEvent, e.Type)
				require.Equal(t, libevents.BillingCardUpdateCode, e.Code)
				require.Equal(t, suite.authIdentity.GetIdentity().Username, e.UserMetadata.User)
			},
		},
		{
			name: "StripeDeleteCard emits BillingCardDelete",
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeDeleteCard = func(ctx context.Context, in *cloudv1.StripeDeleteCardRequest, opts ...grpc.CallOption) (*cloudv1.StripeDeleteCardResponse, error) {
					return &cloudv1.StripeDeleteCardResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeDeleteCard(ctx, &cloudv1.StripeDeleteCardRequest{})
				return err
			},
			assertEvt: func(t *testing.T, ev events.AuditEvent) {
				e, ok := ev.(*events.BillingCardDelete)
				require.True(t, ok, "wrong event type (%T)", ev)
				require.Equal(t, libevents.BillingCardDeleteEvent, e.Type)
				require.Equal(t, libevents.BillingCardDeleteCode, e.Code)
				require.Equal(t, suite.authIdentity.GetIdentity().Username, e.UserMetadata.User)
			},
		},
		{
			name: "StripeUpdateEmail emits BillingInformationUpdate",
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeUpdateEmail = func(ctx context.Context, in *cloudv1.StripeUpdateEmailRequest, opts ...grpc.CallOption) (*cloudv1.StripeUpdateEmailResponse, error) {
					return &cloudv1.StripeUpdateEmailResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeUpdateEmail(ctx, &cloudv1.StripeUpdateEmailRequest{})
				return err
			},
			assertEvt: func(t *testing.T, ev events.AuditEvent) {
				e, ok := ev.(*events.BillingInformationUpdate)
				require.True(t, ok, "wrong event type (%T)", ev)
				require.Equal(t, libevents.BillingInformationUpdateEvent, e.Type)
				require.Equal(t, libevents.BillingInformationUpdateCode, e.Code)
				require.Equal(t, suite.authIdentity.GetIdentity().Username, e.UserMetadata.User)
			},
		},
		{
			name: "StripeUpdatePOPrefix emits BillingInformationUpdate",
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeUpdatePOPrefix = func(ctx context.Context, in *cloudv1.StripeUpdatePOPrefixRequest, opts ...grpc.CallOption) (*cloudv1.StripeUpdatePOPrefixResponse, error) {
					return &cloudv1.StripeUpdatePOPrefixResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeUpdatePOPrefix(ctx, &cloudv1.StripeUpdatePOPrefixRequest{})
				return err
			},
			assertEvt: func(t *testing.T, ev events.AuditEvent) {
				e, ok := ev.(*events.BillingInformationUpdate)
				require.True(t, ok, "wrong event type (%T)", ev)
				require.Equal(t, libevents.BillingInformationUpdateEvent, e.Type)
				require.Equal(t, libevents.BillingInformationUpdateCode, e.Code)
				require.Equal(t, suite.authIdentity.GetIdentity().Username, e.UserMetadata.User)
			},
		},
		{
			name: "StripeUpdateStripeAddress emits BillingInformationUpdate",
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeUpdateStripeAddress = func(ctx context.Context, in *cloudv1.StripeUpdateStripeAddressRequest, opts ...grpc.CallOption) (*cloudv1.StripeUpdateStripeAddressResponse, error) {
					return &cloudv1.StripeUpdateStripeAddressResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeUpdateStripeAddress(ctx, &cloudv1.StripeUpdateStripeAddressRequest{})
				return err
			},
			assertEvt: func(t *testing.T, ev events.AuditEvent) {
				e, ok := ev.(*events.BillingInformationUpdate)
				require.True(t, ok, "wrong event type (%T)", ev)
				require.Equal(t, libevents.BillingInformationUpdateEvent, e.Type)
				require.Equal(t, libevents.BillingInformationUpdateCode, e.Code)
				require.Equal(t, suite.authIdentity.GetIdentity().Username, e.UserMetadata.User)
			},
		},
		{
			name: "StripeCancel emits BillingSubscriptionCancel",
			setMock: func(c *cloud.MockedClient) {
				c.MockStripeCancel = func(ctx context.Context, in *cloudv1.StripeCancelRequest, opts ...grpc.CallOption) (*cloudv1.StripeCancelResponse, error) {
					return &cloudv1.StripeCancelResponse{}, nil
				}
			},
			call: func(ac *cloudWithRoles) error {
				_, err := ac.StripeCancel(ctx, &cloudv1.StripeCancelRequest{})
				return err
			},
			assertEvt: func(t *testing.T, ev events.AuditEvent) {
				e, ok := ev.(*events.BillingSubscriptionCancel)
				require.True(t, ok, "wrong event type (%T)", ev)
				require.Equal(t, libevents.BillingSubscriptionCancelEvent, e.Type)
				require.Equal(t, libevents.BillingSubscriptionCancelCode, e.Code)
				require.Equal(t, suite.authIdentity.GetIdentity().Username, e.UserMetadata.User)
			},
		},
	}

	for _, tc := range tt {
		tc.setMock(suite.cloudClient)

		t.Run(tc.name+"/unauthorized emits no event", func(t *testing.T) {
			suite.emitter.in = nil
			suite.authorizer.authorize = unauthorized
			require.True(t, trace.IsAccessDenied(tc.call(suite.cloudWithRoles)))
			require.Nil(t, suite.emitter.in)
		})

		t.Run(tc.name+"/successful emits event", func(t *testing.T) {
			suite.emitter.in = nil
			suite.authorizer.authorize = originalAuthorize
			require.NoError(t, tc.call(suite.cloudWithRoles))
			require.NotNil(t, suite.emitter.in)
			tc.assertEvt(t, suite.emitter.in)
		})
	}
}

func newAccessListMember(t *testing.T, accessList, name string) *accesslist.AccessListMember {
	t.Helper()

	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: name,
		},
		accesslist.AccessListMemberSpec{
			AccessList: accessList,
			Name:       name,
			Joined:     time.Now(),
			Expires:    time.Now().Add(time.Hour * 24),
			Reason:     "a reason",
			AddedBy:    "dummy",
		},
	)
	require.NoError(t, err)
	return member
}
