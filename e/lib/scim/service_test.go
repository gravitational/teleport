package scim

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
)

// enableIGS configures the system modules to allow IGS features for tge life of
// the supplied test.
func enableIGS(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			IdentityGovernanceSecurity: true,
		},
	})
}

func bindTestFn[A, B any](fn func(context.Context, A) (B, error), val A) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := fn(ctx, val)
		return err
	}
}

// TestSCIMServiceDeniesAccessToNonProxyUser tests that a non-proxy user always
// causes an AccessDenied error without touching any other resources.
func TestSCIMServiceDeniesAccessToNonProxyUser(t *testing.T) {
	enableIGS(t)
	uut, _ := newTestService(t)

	nonProxyCtx := authz.ContextWithUser(
		context.Background(),
		authz.BuiltinRole{
			Role:     types.RoleOkta,
			Username: string(types.RoleOkta),
		})

	testFuncs := []struct {
		name string
		fn   func(context.Context) error
	}{
		{
			name: "list",
			fn:   bindTestFn(uut.ListSCIMResources, &scimpb.ListSCIMResourcesRequest{}),
		}, {
			name: "get",
			fn:   bindTestFn(uut.GetSCIMResource, &scimpb.GetSCIMResourceRequest{}),
		}, {
			name: "create",
			fn:   bindTestFn(uut.CreateSCIMResource, &scimpb.CreateSCIMResourceRequest{}),
		}, {
			name: "update",
			fn:   bindTestFn(uut.UpdateSCIMResource, &scimpb.UpdateSCIMResourceRequest{}),
		}, {
			name: "delete",
			fn:   bindTestFn(uut.DeleteSCIMResource, &scimpb.DeleteSCIMResourceRequest{}),
		},
	}

	for _, f := range testFuncs {
		t.Run(f.name, func(t *testing.T) {
			// Given a context tagged with a non-proxy user, when I attempt to
			// invoke a public method on the SCIM server...
			err := f.fn(nonProxyCtx)

			// Expect that the operation fails with AccessDenied
			require.Error(t, err)
			require.True(t, trace.IsAccessDenied(err),
				"Expected AccessDenied, got %q", err.Error())

			// Because the behavior of the mock services is to panic if any
			// unexpected calls are made to them, We also (implicitly) test the
			// expectation that the Service did not interact with any *other*
			// services before issuing the AccessDenied error.
		})
	}
}

// TestSCIMServiceFailsWithoutIGS asserts that the SCIM service will refuse to
// serve requests when IGS is disabled
func TestSCIMServiceFailsWithoutIGS(t *testing.T) {
	// Explicitly disable IGS
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			IdentityGovernanceSecurity: false,
		},
	})

	uut, fix := newTestService(t)

	testFuncs := []struct {
		name string
		fn   func(context.Context) error
	}{
		{
			name: "list",
			fn:   bindTestFn(uut.ListSCIMResources, &scimpb.ListSCIMResourcesRequest{}),
		}, {
			name: "get",
			fn:   bindTestFn(uut.GetSCIMResource, &scimpb.GetSCIMResourceRequest{}),
		}, {
			name: "create",
			fn:   bindTestFn(uut.CreateSCIMResource, &scimpb.CreateSCIMResourceRequest{}),
		}, {
			name: "update",
			fn:   bindTestFn(uut.UpdateSCIMResource, &scimpb.UpdateSCIMResourceRequest{}),
		}, {
			name: "delete",
			fn:   bindTestFn(uut.DeleteSCIMResource, &scimpb.DeleteSCIMResourceRequest{}),
		},
	}

	for _, f := range testFuncs {
		t.Run(f.name, func(t *testing.T) {
			// Given a context tagged with a non-proxy user, when I attempt to
			// invoke a public method on the SCIM server...
			err := f.fn(fix.userCtx)

			// Expect that the operation fails with NotImplemented
			require.Error(t, err)
			require.True(t, trace.IsNotImplemented(err),
				"Expected NotImplemented, got %q", err.Error())

			// Because the behavior of the mock services is to panic if any
			// unexpected calls are made to them, We also (implicitly) test the
			// expectation that the Service did not interact with any *other*
			// services before issuing the NotImplemented error.
		})
	}
}

// anyContext is an argument matcher for testify mocks that matches any context.
var anyContext interface{} = mock.MatchedBy(func(context.Context) bool { return true })

var anyResource interface{} = mock.MatchedBy(func(*scimpb.Resource) bool { return true })

const (
	testPluginName          = "test"
	testPluginOrgUrl        = "https://mystery-machine.mockta.com"
	testSSOConnectorID      = "test-okta-integration"
	testAppID               = "okta-app-id"
	testPluginID            = "some-string-unique-to-the-plugin"
	testAuthHeader          = "some sort of bearer token"
	withSecrets             = true
	withoutSecrets          = false
	testUserExternalIDlabel = "mock-extrenal-id"
	testUserLabel           = "favouriteFruit"
	testUserLabelValue      = "nectarine"

	// doesn't really matter what this value is, as long as it matches the
	// plugin value created by mkTestPlugin() and is unlikely to be something
	// that will have a SCIM integration
	testPluginType = types.PluginTypeJira
)

func mkTestPlugin() types.Plugin {
	return &types.PluginV1{
		Kind:    types.KindPlugin,
		SubKind: types.PluginSubkindAccess,
		Version: "abc123",
		Metadata: types.Metadata{
			Name:      testPluginName,
			Namespace: defaults.Namespace,
			Labels:    map[string]string{"test": testPluginName},
		},
		// Spec is deliberately empty, apart from giving the plugin just enough
		// info to determine its type. The SCIM server should not touch *any*
		// plugin-type specific properties of the plugin. Only a type-specific
		// shim should is allowed to do that, and we'll handle that with a mock
		// shim for these tests.
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Jira{},
		},
		Credentials: &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{
						"plugin": testPluginID,
					},
				},
			},
		},
	}
}

func requireNotFound(t require.TestingT, err error, _ ...interface{}) {
	require.True(t, trace.IsNotFound(err), "Expected NotFound, got %s", err)
}

func requireAlreadyExists(t require.TestingT, err error, _ ...interface{}) {
	require.True(t, trace.IsAlreadyExists(err), "Expected AlreadyExists, got %s", err)
}
