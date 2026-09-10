package common

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

type sutOptions struct {
	clusterName   string
	resources     []types.Resource
	samlConnector string
	license       string
	HTTPTransport http.RoundTripper
	clock         clockwork.Clock
	logger        *slog.Logger
	appConfig     servicecfg.AppsConfig
	insecureMode  bool
	modules       *modulestest.Modules
	disableCache  bool
}

type Option func(*sutOptions)

func WithModules(m *modulestest.Modules) func(*sutOptions) {
	return func(o *sutOptions) {
		o.modules = m
	}
}
func WithInsecure() func(*sutOptions) {
	return func(o *sutOptions) {
		o.insecureMode = true
	}
}

func WithClusterName(name string) func(*sutOptions) {
	return func(o *sutOptions) {
		o.clusterName = name
	}
}

// WithoutCache is an [Option] that disables the Teleport cache
func WithoutCache(o *sutOptions) {
	o.disableCache = true
}

// WithApp adds an apps service to the configuration allowing to run SUT with
// application access layer.
func WithApp(name, uri string, labels map[string]string) func(*sutOptions) {
	return func(o *sutOptions) {
		o.appConfig.Enabled = true
		o.appConfig.Apps = append(o.appConfig.Apps, servicecfg.App{
			Name:         name,
			URI:          uri,
			StaticLabels: labels,
		})
	}
}

// WithUser adds a user with the given roles to the SUT.
func WithUser(t *testing.T, user string, roles ...string) func(*sutOptions) {
	userResource, err := types.NewUser(user)
	require.NoError(t, err)
	userResource.SetRoles(roles)

	return WithResources(userResource)
}

type RoleOption func(*types.RoleV6)

func WithSearchAs(rct types.RoleConditionType, r string) RoleOption {
	return func(rv *types.RoleV6) {
		dst := &rv.Spec.Deny
		if rct == types.Allow {
			dst = &rv.Spec.Allow
		}
		if dst.Request == nil {
			dst.Request = &types.AccessRequestConditions{}
		}
		dst.Request.SearchAsRoles = append(dst.Request.SearchAsRoles, r)
	}
}

func WithClusterLabel(rct types.RoleConditionType, key string, values ...string) RoleOption {
	return func(rv *types.RoleV6) {
		dst := &rv.Spec.Deny
		if rct == types.Allow {
			dst = &rv.Spec.Allow
		}
		if dst.ClusterLabels == nil {
			dst.ClusterLabels = types.Labels{}
		}
		dst.ClusterLabels[key] = values
	}
}

func WithRoleNodeLabel(rct types.RoleConditionType, key string, values ...string) RoleOption {
	return func(rv *types.RoleV6) {
		dst := &rv.Spec.Deny
		if rct == types.Allow {
			dst = &rv.Spec.Allow
		}
		if dst.NodeLabels == nil {
			dst.NodeLabels = types.Labels{}
		}
		dst.NodeLabels[key] = values
	}
}

func WithAccountAssignment(rct types.RoleConditionType, accountID, permissionSetARN string) RoleOption {
	return func(rv *types.RoleV6) {
		dst := &rv.Spec.Deny
		if rct == types.Allow {
			dst = &rv.Spec.Allow
		}
		if dst.NodeLabels == nil {
			dst.NodeLabels = types.Labels{}
		}
		assignment := types.IdentityCenterAccountAssignment{
			Account:       accountID,
			PermissionSet: permissionSetARN,
		}
		dst.AccountAssignments = append(dst.AccountAssignments, assignment)
	}
}

func WithRole(t *testing.T, name string, options ...RoleOption) func(*sutOptions) {
	role, err := types.NewRole(name, types.RoleSpecV6{})
	require.NoError(t, err)

	rv, ok := role.(*types.RoleV6)
	require.True(t, ok, "expected RoleV6, got %T", role)
	for _, applyOption := range options {
		applyOption(rv)
	}

	return func(o *sutOptions) {
		o.resources = append(o.resources, rv)
	}
}

// WithResources adds resources to the SUT during initialization.
func WithResources(resources ...types.Resource) func(*sutOptions) {
	return func(o *sutOptions) {
		o.resources = append(o.resources, resources...)
	}
}

// WithHTTPClient  adds an HTTP client to the SUT.
func WithHTTPClient(tr http.RoundTripper) func(*sutOptions) {
	return func(o *sutOptions) {
		o.HTTPTransport = tr
	}
}

// WithSAMLConnector add a SAML connector to the SUT.
func WithSAMLConnector(connector string) func(*sutOptions) {
	return func(o *sutOptions) {
		o.samlConnector = connector
	}
}

// WithLicense adds a license to the SUT.
func WithLicense(license string) func(*sutOptions) {
	return func(o *sutOptions) {
		o.license = license
	}
}

// WithClock sets the clock for the SUT.
func WithClock(clock clockwork.Clock) func(*sutOptions) {
	return func(o *sutOptions) {
		o.clock = clock
	}
}

func WithLogger(logger *slog.Logger) func(*sutOptions) {
	return func(o *sutOptions) {
		o.logger = logger
	}
}
