package auth

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudv1 "github.com/gravitational/teleport/e/api/cloud/v1"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	libevents "github.com/gravitational/teleport/lib/events"
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
	in apievents.AuditEvent
}

// EmitAuditEvent stores `event` into the `in` property.
func (m *mockEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
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
	// emitter is a mockEmitter that implements the `apievents.Emitter` interface and
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
	})
	require.NoError(t, err)

	cloudClient := &cloud.MockedClient{}
	authPlugin.cloudClient = cloudClient

	authPlugin.authServer = &auth.GRPCServer{}
	emitter := &mockEmitter{}
	authPlugin.authServer.Emitter = emitter

	roles := auth.GetPresetRoles()
	rolesNames := make([]string, len(roles))
	for i, r := range roles {
		rolesNames[i] = r.GetName()
	}

	authIdentity := auth.TestAdmin().I
	authorizer := &mockAuthorizer{
		authorize: func(ctx context.Context) (*authz.Context, error) {
			return &authz.Context{
				User:     newUser(t, "testuser", types.UserTypeLocal, rolesNames...),
				Checker:  services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", auth.GetPresetRoles()),
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
	return &v1.Contact{
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
	removeContactResp := &v1.RemoveContactResponse{
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
		assert    func(t *testing.T, resp *cloudv1.RemoveContactResponse, err error, event apievents.AuditEvent)
	}{
		{
			name:      "unauthorized request should emit no events",
			authorize: false,
			req:       &v1.RemoveContactRequest{},
			assert: func(t *testing.T, resp *cloudv1.RemoveContactResponse, err error, event apievents.AuditEvent) {
				require.True(t, trace.IsAccessDenied(err))
				require.Nil(t, event)
			},
		},
		{
			name:      "successful request should emit events",
			authorize: true,
			req: &v1.RemoveContactRequest{
				ContactType: 2,
			},
			assert: func(t *testing.T, resp *cloudv1.RemoveContactResponse, err error, event apievents.AuditEvent) {
				require.NoError(t, err)
				require.Equal(t, removeContactResp, resp)
				// check event
				require.NotNil(t, event)
				removeEvent := event.(*apievents.ContactDelete)
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
	createContactResp := &v1.CreateContactResponse{
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
		assert    func(t *testing.T, resp *cloudv1.CreateContactResponse, err error, event apievents.AuditEvent)
	}{
		{
			name:      "unauthorized request should emit no events",
			authorize: false,
			req:       &v1.CreateContactRequest{},
			assert: func(t *testing.T, resp *cloudv1.CreateContactResponse, err error, event apievents.AuditEvent) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
				require.Nil(t, event)
			},
		},
		{
			name:      "successful request should emit events",
			authorize: true,
			req: &v1.CreateContactRequest{
				Email:       "email@goteleport.com",
				ContactType: 2,
			},
			assert: func(t *testing.T, resp *cloudv1.CreateContactResponse, err error, event apievents.AuditEvent) {
				require.NoError(t, err)
				require.Equal(t, createContactResp, resp)
				// check event
				require.NotNil(t, event)
				createEvent := event.(*apievents.ContactCreate)
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
