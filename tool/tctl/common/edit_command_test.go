/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestEditResources(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{TestBuildType: modules.BuildEnterprise})

	log := utils.NewSlogLoggerForTests()
	process, err := testenv.NewTeleportProcess(t.TempDir(), testenv.WithLogger(log))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})
	rootClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rootClient.Close() })

	tests := []struct {
		name string
		edit func(t *testing.T, clt *authclient.Client)
	}{
		{
			name: types.KindGithubConnector,
			edit: testEditGithubConnector,
		},
		{
			name: types.KindRole,
			edit: testEditRole,
		},
		{
			name: types.KindUser,
			edit: testEditUser,
		},
		{
			name: types.KindClusterNetworkingConfig,
			edit: testEditClusterNetworkingConfig,
		},
		{
			name: types.KindClusterAuthPreference,
			edit: testEditAuthPreference,
		},
		{
			name: types.KindSessionRecordingConfig,
			edit: testEditSessionRecordingConfig,
		},
		{
			name: types.KindStaticHostUser,
			edit: testEditStaticHostUser,
		},
		{
			name: types.KindAutoUpdateConfig,
			edit: testEditAutoUpdateConfig,
		},
		{
			name: types.KindAutoUpdateVersion,
			edit: testEditAutoUpdateVersion,
		},
		{
			name: types.KindDynamicWindowsDesktop,
			edit: testEditDynamicWindowsDesktop,
		},
		{
			name: "edit multiple resources with SubKind (" + types.KindCertAuthority + ")",
			edit: testEditMultipleWithSubKind,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.edit(t, rootClient)
		})
	}
}

func testEditGithubConnector(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected, err := types.NewGithubConnector("github", types.GithubConnectorSpecV3{
		ClientID:     "12345",
		ClientSecret: "678910",
		RedirectURL:  "https://proxy.example.com/v1/webapi/github/callback",
		Display:      "Github",
		TeamsToRoles: []types.TeamRolesMapping{
			{
				Organization: "acme",
				Team:         "users",
				Roles:        []string{"access", "editor", "auditor"},
			},
		},
	})
	require.NoError(t, err, "creating initial connector resource")
	created, err := clt.CreateGithubConnector(ctx, expected.(*types.GithubConnectorV3))
	require.NoError(t, err, "persisting initial connector resource")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.SetRevision(created.GetRevision())
		expected.SetClientID("abcdef")

		collection := &connectorsCollection{github: []types.GithubConnector{expected}}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the connector and validate that the expected field is updated.
	_, err = runEditCommand(t, clt, []string{"edit", "connector/github"}, withEditor(editor))
	require.NoError(t, err, "expected editing github connector to succeed")

	actual, err := clt.GetGithubConnector(ctx, expected.GetName(), true)
	require.NoError(t, err, "retrieving github connector after edit")
	assert.NotEqual(t, created.GetClientID(), actual.GetClientID(), "client id should have been modified by edit")
	require.Empty(t, cmp.Diff(expected, actual, cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace")))

	// Try editing the connector a second time. This time the revisions will not match
	// since the created revision is stale.
	_, err = runEditCommand(t, clt, []string{"edit", "connector/github"}, withEditor(editor))
	assert.Error(t, err, "stale connector was allowed to be updated")
	require.ErrorIs(t, err, backend.ErrIncorrectRevision, "expected an incorrect revision error, got %T", err)
}

func testEditRole(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected, err := types.NewRole("test-role", types.RoleSpecV6{})
	require.NoError(t, err, "creating initial role resource")
	created, err := clt.CreateRole(ctx, expected.(*types.RoleV6))
	require.NoError(t, err, "persisting initial role resource")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.SetRevision(created.GetRevision())
		expected.SetLogins(types.Allow, []string{"abcdef"})

		collection := &roleCollection{roles: []types.Role{expected}}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the role and validate that the expected field is updated.
	_, err = runEditCommand(t, clt, []string{"edit", "role/test-role"}, withEditor(editor))
	require.NoError(t, err, "expected editing role to succeed")

	actual, err := clt.GetRole(ctx, expected.GetName())
	require.NoError(t, err, "retrieving role after edit")
	assert.NotEqual(t, created.GetLogins(types.Allow), actual.GetLogins(types.Allow), "logins should have been modified by edit")
	require.Empty(t, cmp.Diff(expected, actual, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Try editing the role a second time. This time the revisions will not match
	// since the created revision is stale.
	_, err = runEditCommand(t, clt, []string{"edit", "role/test-role"}, withEditor(editor))
	assert.Error(t, err, "stale role was allowed to be updated")
	require.ErrorIs(t, err, backend.ErrIncorrectRevision, "expected an incorrect revision error, got %T", err)
}

func testEditUser(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected, err := types.NewUser("llama")
	require.NoError(t, err, "creating initial user resource")
	created, err := clt.CreateUser(ctx, expected.(*types.UserV2))
	require.NoError(t, err, "persisting initial user resource")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.SetRevision(created.GetRevision())
		expected.SetLogins([]string{"abcdef"})
		expected.SetCreatedBy(created.GetCreatedBy())
		expected.SetWeakestDevice(created.GetWeakestDevice())

		collection := &userCollection{users: []types.User{expected}}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the user and validate that the expected field is updated.
	_, err = runEditCommand(t, clt, []string{"edit", "user/llama"}, withEditor(editor))
	require.NoError(t, err, "expected editing role to succeed")

	actual, err := clt.GetUser(ctx, expected.GetName(), true)
	require.NoError(t, err, "retrieving user after edit")
	assert.NotEqual(t, created.GetLogins(), actual.GetLogins(), "logins should have been modified by edit")
	require.Empty(t, cmp.Diff(expected, actual, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// Try editing the user a second time. This time the revisions will not match
	// since the created revision is stale.
	_, err = runEditCommand(t, clt, []string{"edit", "user/llama"}, withEditor(editor))
	assert.Error(t, err, "stale user was allowed to be updated")
	require.ErrorIs(t, err, backend.ErrIncorrectRevision, "expected an incorrect revision error, got %T", err)
}

func testEditClusterNetworkingConfig(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected := types.DefaultClusterNetworkingConfig()
	initial, err := clt.GetClusterNetworkingConfig(ctx)
	require.NoError(t, err, "getting initial networking config")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.SetRevision(initial.GetRevision())
		expected.SetKeepAliveCountMax(1)
		expected.SetCaseInsensitiveRouting(true)

		collection := &netConfigCollection{netConfig: expected}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the cnc and validate that the expected field is updated.
	_, err = runEditCommand(t, clt, []string{"edit", "cluster_networking_config"}, withEditor(editor))
	require.NoError(t, err, "expected editing cnc to succeed")

	actual, err := clt.GetClusterNetworkingConfig(ctx)
	require.NoError(t, err, "retrieving cnc after edit")
	assert.NotEqual(t, initial.GetKeepAliveCountMax(), actual.GetKeepAliveCountMax(), "keep alive count max should have been modified by edit")
	assert.NotEqual(t, initial.GetCaseInsensitiveRouting(), actual.GetCaseInsensitiveRouting(), "keep alive count max should have been modified by edit")
	require.Empty(t, cmp.Diff(expected, actual, cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Labels")))
	assert.Equal(t, types.OriginDynamic, actual.Origin())

	// Try editing the cnc a second time. This time the revisions will not match
	// since the created revision is stale.
	_, err = runEditCommand(t, clt, []string{"edit", "cluster_networking_config"}, withEditor(editor))
	assert.Error(t, err, "stale cnc was allowed to be updated")
	require.ErrorIs(t, err, backend.ErrIncorrectRevision, "expected an incorrect revision error, got %T", err)
}

func testEditAuthPreference(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected := types.DefaultAuthPreference()
	initial, err := clt.GetAuthPreference(ctx)
	require.NoError(t, err, "getting initial auth preference")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.SetRevision(initial.GetRevision())
		expected.SetSecondFactors(types.SecondFactorType_SECOND_FACTOR_TYPE_OTP, types.SecondFactorType_SECOND_FACTOR_TYPE_SSO)

		collection := &authPrefCollection{authPref: expected}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the cap and validate that the expected field is updated.
	_, err = runEditCommand(t, clt, []string{"edit", "cap"}, withEditor(editor))
	require.NoError(t, err, "expected editing cap to succeed")

	actual, err := clt.GetAuthPreference(ctx)
	require.NoError(t, err, "retrieving cap after edit")
	assert.NotEqual(t, initial.GetSecondFactors(), actual.GetSecondFactors(), "second factors should have been modified by edit")
	require.Empty(t, cmp.Diff(expected, actual, cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Labels")))
	assert.Equal(t, types.OriginDynamic, actual.Origin())

	// Try editing the cap a second time. This time the revisions will not match
	// since the created revision is stale.
	_, err = runEditCommand(t, clt, []string{"edit", "cap"}, withEditor(editor))
	assert.Error(t, err, "stale cap was allowed to be updated")
	require.ErrorIs(t, err, backend.ErrIncorrectRevision, "expected an incorrect revision error, got %T", err)
}

func testEditSessionRecordingConfig(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected := types.DefaultSessionRecordingConfig()
	initial, err := clt.GetSessionRecordingConfig(ctx)
	require.NoError(t, err, "getting initial session recording config")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.SetRevision(initial.GetRevision())
		expected.SetMode(types.RecordAtProxy)

		collection := &recConfigCollection{recConfig: expected}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the src and validate that the expected field is updated.
	_, err = runEditCommand(t, clt, []string{"edit", "session_recording_config"}, withEditor(editor))
	require.NoError(t, err, "expected editing src to succeed")

	actual, err := clt.GetSessionRecordingConfig(ctx)
	require.NoError(t, err, "retrieving src after edit")
	assert.NotEqual(t, initial.GetMode(), actual.GetMode(), "mode should have been modified by edit")
	require.Empty(t, cmp.Diff(expected, actual, cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Labels")))
	assert.Equal(t, types.OriginDynamic, actual.Origin())

	// Try editing the src a second time. This time the revisions will not match
	// since the created revision is stale.
	_, err = runEditCommand(t, clt, []string{"edit", "session_recording_config"}, withEditor(editor))
	assert.Error(t, err, "stale src was allowed to be updated")
	require.ErrorIs(t, err, backend.ErrIncorrectRevision, "expected an incorrect revision error, got %T", err)
}

// TestEditEnterpriseResources asserts that tctl edit
// behaves as expected for enterprise resources. These resources cannot
// be tested in parallel because they alter the modules to enable features.
// The tests are grouped to amortize the cost of creating and auth server since
// that is the most expensive part of testing editing the resource.
func TestEditEnterpriseResources(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.OIDC: {Enabled: true},
				entitlements.SAML: {Enabled: true},
			},
		},
	})
	log := utils.NewSlogLoggerForTests()
	process, err := testenv.NewTeleportProcess(t.TempDir(), testenv.WithLogger(log))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})
	rootClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rootClient.Close() })

	tests := []struct {
		kind string
		edit func(t *testing.T, clt *authclient.Client)
	}{
		{
			kind: types.KindOIDCConnector,
			edit: testEditOIDCConnector,
		},
		{
			kind: types.KindSAMLConnector,
			edit: testEditSAMLConnector,
		},
	}

	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			test.edit(t, rootClient)
		})
	}
}

func testEditOIDCConnector(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()
	expected, err := types.NewOIDCConnector("oidc", types.OIDCConnectorSpecV3{
		ClientID:     "12345",
		ClientSecret: "678910",
		RedirectURLs: []string{"https://proxy.example.com/v1/webapi/github/callback"},
		Display:      "OIDC",
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "test",
				Value: "test",
				Roles: []string{"access", "editor", "auditor"},
			},
		},
	})
	require.NoError(t, err, "creating initial connector resource")
	created, err := clt.CreateOIDCConnector(ctx, expected.(*types.OIDCConnectorV3))
	require.NoError(t, err, "persisting initial connector resource")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.SetRevision(created.GetRevision())
		expected.SetClientID("abcdef")

		collection := &connectorsCollection{oidc: []types.OIDCConnector{expected}}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the connector and validate that the expected field is updated.
	_, err = runEditCommand(t, clt, []string{"edit", "connector/oidc"}, withEditor(editor))
	require.NoError(t, err, "expected editing oidc connector to succeed")

	actual, err := clt.GetOIDCConnector(ctx, expected.GetName(), false)
	require.NoError(t, err, "retrieving oidc connector after edit")
	require.Empty(t, cmp.Diff(created, actual, cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
		cmpopts.IgnoreFields(types.OIDCConnectorSpecV3{}, "ClientID", "ClientSecret"),
	))
	require.NotEqual(t, created.GetClientID(), actual.GetClientID(), "client id should have been modified by edit")
	require.Equal(t, expected.GetClientID(), actual.GetClientID(), "client id should match the retrieved connector")

	// Try editing the connector a second time. This time the revisions will not match
	// since the created revision is stale.
	_, err = runEditCommand(t, clt, []string{"edit", "connector/oidc"}, withEditor(editor))
	assert.Error(t, err, "stale connector was allowed to be updated")
	require.ErrorIs(t, err, backend.ErrIncorrectRevision, "expected an incorrect revision error, got %T", err)
}

func testEditSAMLConnector(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected, err := types.NewSAMLConnector("saml", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "original-acs",
		EntityDescriptor: `<?xml version="1.0" encoding="UTF-8"?>
    <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="test">
      <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
        <md:KeyDescriptor use="signing">
          <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
            <ds:X509Data>
              <ds:X509Certificate></ds:X509Certificate>
            </ds:X509Data>
          </ds:KeyInfo>
        </md:KeyDescriptor>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://example.com" />
        <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://example.com" />
      </md:IDPSSODescriptor>
    </md:EntityDescriptor>`,
		Display: "SAML",
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "test",
				Value: "test",
				Roles: []string{"access"},
			},
		},
	})
	require.NoError(t, err, "creating initial connector resource")

	created, err := clt.CreateSAMLConnector(ctx, expected.(*types.SAMLConnectorV2))
	require.NoError(t, err, "persisting initial connector resource")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.SetRevision(created.GetRevision())
		expected.SetSigningKeyPair(created.GetSigningKeyPair())
		expected.SetAssertionConsumerService("updated-acs")

		collection := &connectorsCollection{saml: []types.SAMLConnector{expected}}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the connector and validate that the expected field is updated.
	_, err = runEditCommand(t, clt, []string{"edit", "connector/saml"}, withEditor(editor))
	require.NoError(t, err, "expected editing saml connector to succeed")

	actual, err := clt.GetSAMLConnector(ctx, expected.GetName(), true)
	require.NoError(t, err, "retrieving saml connector after edit")
	require.Empty(t, cmp.Diff(created, actual, cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
		cmpopts.IgnoreFields(types.SAMLConnectorSpecV2{}, "AssertionConsumerService"),
	))
	require.NotEqual(t, created.GetAssertionConsumerService(), actual.GetAssertionConsumerService(), "acs should have been modified by edit")
	require.Equal(t, expected.GetAssertionConsumerService(), actual.GetAssertionConsumerService(), "acs should match the retrieved connector")

	// Try editing the connector a second time this, time the revisions will not match
	// since the created revision is stale.
	_, err = runEditCommand(t, clt, []string{"edit", "connector/saml"}, withEditor(editor))
	assert.Error(t, err, "stale connector was allowed to be updated")
	require.ErrorIs(t, err, backend.ErrIncorrectRevision, "expected an incorrect revision error, got %T", err)
}

func testEditStaticHostUser(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected := userprovisioning.NewStaticHostUser("alice", &userprovisioningpb.StaticHostUserSpec{
		Matchers: []*userprovisioningpb.Matcher{
			{
				NodeLabels: []*labelv1.Label{
					{
						Name:   "foo",
						Values: []string{"bar"},
					},
				},
				Groups: []string{"foo", "bar"},
			},
		},
	})
	created, err := clt.StaticHostUserClient().CreateStaticHostUser(ctx, expected)
	require.NoError(t, err)

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.GetMetadata().Revision = created.GetMetadata().Revision
		expected.Spec.Matchers[0].Groups = []string{"baz", "quux"}

		collection := &staticHostUserCollection{items: []*userprovisioningpb.StaticHostUser{expected}}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	_, err = runEditCommand(t, clt, []string{"edit", "host_user/alice"}, withEditor(editor))
	require.NoError(t, err)

	actual, err := clt.StaticHostUserClient().GetStaticHostUser(ctx, expected.GetMetadata().Name)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	))

	_, err = runEditCommand(t, clt, []string{"edit", "host_user/alice"}, withEditor(editor))
	require.Error(t, err)
	require.True(t, trace.IsCompareFailed(err), "unexpected error: %v", err)
}

func testEditAutoUpdateConfig(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected, err := autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{
		Tools: &autoupdatev1pb.AutoUpdateConfigSpecTools{
			Mode: autoupdate.ToolsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)

	initial, err := autoupdate.NewAutoUpdateConfig(&autoupdatev1pb.AutoUpdateConfigSpec{
		Tools: &autoupdatev1pb.AutoUpdateConfigSpecTools{
			Mode: autoupdate.ToolsUpdateModeDisabled,
		},
	})
	require.NoError(t, err)

	serviceClient := autoupdatev1pb.NewAutoUpdateServiceClient(clt.GetConnection())
	initial, err = serviceClient.CreateAutoUpdateConfig(ctx, &autoupdatev1pb.CreateAutoUpdateConfigRequest{Config: initial})
	require.NoError(t, err, "creating initial autoupdate config")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}
		expected.GetMetadata().Revision = initial.GetMetadata().GetRevision()
		collection := &autoUpdateConfigCollection{config: expected}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the AutoUpdateConfig resource.
	_, err = runEditCommand(t, clt, []string{"edit", "autoupdate_config"}, withEditor(editor))
	require.NoError(t, err, "expected editing autoupdate config to succeed")

	actual, err := clt.GetAutoUpdateConfig(ctx)
	require.NoError(t, err, "failed to get autoupdate config after edit")
	assert.NotEqual(t, initial.GetSpec().GetTools().Mode, actual.GetSpec().GetTools().GetMode(),
		"tools_autoupdate should have been modified by edit")
	assert.Equal(t, expected.GetSpec().GetTools().GetMode(), actual.GetSpec().GetTools().GetMode())
}

func testEditAutoUpdateVersion(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected, err := autoupdate.NewAutoUpdateVersion(&autoupdatev1pb.AutoUpdateVersionSpec{
		Tools: &autoupdatev1pb.AutoUpdateVersionSpecTools{
			TargetVersion: "3.2.1",
		},
	})
	require.NoError(t, err)

	initial, err := autoupdate.NewAutoUpdateVersion(&autoupdatev1pb.AutoUpdateVersionSpec{
		Tools: &autoupdatev1pb.AutoUpdateVersionSpecTools{
			TargetVersion: "1.2.3",
		},
	})
	require.NoError(t, err)

	serviceClient := autoupdatev1pb.NewAutoUpdateServiceClient(clt.GetConnection())
	initial, err = serviceClient.CreateAutoUpdateVersion(ctx, &autoupdatev1pb.CreateAutoUpdateVersionRequest{Version: initial})
	require.NoError(t, err, "creating initial autoupdate version")

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}
		expected.GetMetadata().Revision = initial.GetMetadata().GetRevision()
		collection := &autoUpdateVersionCollection{version: expected}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the AutoUpdateVersion resource.
	_, err = runEditCommand(t, clt, []string{"edit", "autoupdate_version"}, withEditor(editor))
	require.NoError(t, err, "expected editing autoupdate version to succeed")

	actual, err := clt.GetAutoUpdateVersion(ctx)
	require.NoError(t, err, "failed to get autoupdate version after edit")
	assert.NotEqual(t, initial.GetSpec().GetTools().GetTargetVersion(), actual.GetSpec().GetTools().GetTargetVersion(),
		"tools_autoupdate should have been modified by edit")
	assert.Equal(t, expected.GetSpec().GetTools().GetTargetVersion(), actual.GetSpec().GetTools().GetTargetVersion())
}

func testEditDynamicWindowsDesktop(t *testing.T, clt *authclient.Client) {
	ctx := context.Background()

	expected, err := types.NewDynamicWindowsDesktopV1("test", nil, types.DynamicWindowsDesktopSpecV1{
		Addr: "test",
	})
	require.NoError(t, err)
	created, err := clt.DynamicDesktopClient().CreateDynamicWindowsDesktop(ctx, expected)
	require.NoError(t, err)

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}

		expected.SetRevision(created.GetRevision())
		expected.Spec.Addr = "test2"

		collection := &dynamicWindowsDesktopCollection{desktops: []types.DynamicWindowsDesktop{expected}}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	_, err = runEditCommand(t, clt, []string{"edit", "dynamic_windows_desktop/test"}, withEditor(editor))
	require.NoError(t, err)

	actual, err := clt.DynamicDesktopClient().GetDynamicWindowsDesktop(ctx, expected.GetName())
	require.NoError(t, err)
	expected.SetRevision(actual.GetRevision())
	require.Empty(t, cmp.Diff(expected, actual, protocmp.Transform()))
}

func testEditMultipleWithSubKind(t *testing.T, clt *authclient.Client) {
	t.Parallel()

	overwriteFile := func(f *os.File, valsYAML [][]byte) error {
		if err := f.Truncate(0); err != nil {
			return fmt.Errorf("truncate: %w", err)
		}
		if _, err := f.Seek(0, 0); err != nil {
			return fmt.Errorf("seek to zero: %w", err)
		}

		for i, val := range valsYAML {
			if i > 0 {
				if _, err := f.WriteString("---\n"); err != nil {
					return fmt.Errorf("write: %w", err)
				}
			}
			if _, err := f.Write(val); err != nil {
				return fmt.Errorf("write: %w", err)
			}
			if !bytes.HasSuffix(val, []byte("\n")) {
				if _, err := f.WriteString("\n"); err != nil {
					return fmt.Errorf("write: %w", err)
				}
			}
		}

		return nil
	}

	// Test add/remove detection when the number of resources stays the same.
	t.Run("add/remove", func(t *testing.T) {
		t.Parallel()

		cn, err := clt.GetClusterName()
		require.NoError(t, err, "read cluster name")

		const loadKeys = false
		userCA, err := clt.GetCertAuthority(t.Context(), types.CertAuthID{
			Type:       types.UserCA,
			DomainName: cn.GetClusterName(),
		}, loadKeys)
		require.NoError(t, err, "read User CA")

		userJSON, err := services.MarshalCertAuthority(userCA)
		require.NoError(t, err, "marshal User CA")
		userYAML, err := yaml.JSONToYAML(userJSON)
		require.NoError(t, err, "convert JSON to YAML")

		editor := func(name string) error {
			// Replace the editor file contents with the User CA.
			return os.WriteFile(name, userYAML, 0644)
		}

		// Edit cas/host, then replace it with cas/user.
		_, err = runEditCommand(t, clt,
			[]string{"edit", "cas/host/" + cn.GetClusterName()},
			withEditor(editor),
		)
		assert.ErrorContains(t, err, "was added or removed", "tctl edit error mismatch")
	})

	// Test replacing one of the resources with a duplicate of another.
	t.Run("duplicate", func(t *testing.T) {
		t.Parallel()

		editor := func(name string) error {
			f, err := os.OpenFile(name, os.O_RDWR, 0644)
			if err != nil {
				return fmt.Errorf("read editor file: %w", err)
			}
			defer f.Close()

			// Read CA YAMLs.
			dec := kyaml.NewYAMLOrJSONDecoder(f, defaults.LookaheadBufSize)
			var casYAML [][]byte
			for {
				var raw services.UnknownResource
				if err := dec.Decode(&raw); errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					return fmt.Errorf("decode raw resource: %w", err)
				}
				casYAML = append(casYAML, raw.Raw)
			}

			// Replace an item with a duplicate.
			casYAML[0] = casYAML[1]

			// Overwrite.
			if err := overwriteFile(f, casYAML); err != nil {
				return fmt.Errorf("write editor file: %w", err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("close editor file: %w", err)
			}

			return nil
		}

		// Edit all CAs, then replace one of them with a duplicate.
		_, err := runEditCommand(t, clt, []string{"edit", "cas"}, withEditor(editor))
		assert.ErrorContains(t, err, "duplicate kind/sub_kind/name", "tctl edit error mismatch")
	})

	t.Run("edit", func(t *testing.T) {
		t.Parallel()

		const caType1 = types.HostCA
		const caType2 = types.UserCA

		getCAs := func(t *testing.T) (_, _ types.CertAuthority) {
			ctx := t.Context()
			const loadKeys = false

			cas1, err := clt.GetCertAuthorities(ctx, caType1, loadKeys)
			require.NoError(t, err, "CA not found: %s", caType1)
			require.Len(t, cas1, 1)

			cas2, err := clt.GetCertAuthorities(ctx, caType2, loadKeys)
			require.NoError(t, err, "CA not found: %s", caType2)
			require.Len(t, cas2, 1)

			return cas1[0], cas2[0]
		}

		// Prepare wanted CAs.
		ca1, ca2 := getCAs(t)
		md := ca1.GetMetadata()
		md.Description = "description 1"
		ca1.SetMetadata(md)
		md = ca2.GetMetadata()
		md.Description = "description 2"
		ca2.SetMetadata(md)

		editor := func(name string) error {
			f, err := os.OpenFile(name, os.O_RDWR, 0644)
			if err != nil {
				return fmt.Errorf("read editor file: %w", err)
			}
			defer f.Close()

			// Parse/edit CAs.
			dec := kyaml.NewYAMLOrJSONDecoder(f, defaults.LookaheadBufSize)
			var casYAML [][]byte
			editCount := 0
			for {
				var raw services.UnknownResource
				if err := dec.Decode(&raw); errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					return fmt.Errorf("decode raw resource: %w", err)
				}
				ca, err := services.UnmarshalCertAuthority(raw.Raw)
				if err != nil {
					return fmt.Errorf("unmarshal CA resource: %w", err)
				}

				switch ca.GetType() {
				case ca1.GetType():
					ca.SetMetadata(ca1.GetMetadata())
					editCount++
				case ca2.GetType():
					ca.SetMetadata(ca2.GetMetadata())
					editCount++
				default:
					// Don't change non-edited YAMLs.
					casYAML = append(casYAML, raw.Raw)
					continue
				}

				caJSON, err := services.MarshalCertAuthority(ca)
				if err != nil {
					return fmt.Errorf("marshal CA: %w", err)
				}
				caYAML, err := yaml.JSONToYAML(caJSON)
				if err != nil {
					return fmt.Errorf("convert JSON to YAML: %w", err)
				}
				casYAML = append(casYAML, caYAML)
			}

			const wantEdits = 2
			if wantEdits != editCount {
				return fmt.Errorf("edit count mismatch (want %d, got %d)", wantEdits, editCount)
			}

			// Write edited CAs.
			if err := overwriteFile(f, casYAML); err != nil {
				return fmt.Errorf("write editor file: %w", err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("close editor file: %w", err)
			}

			return nil
		}

		_, err := runEditCommand(t, clt, []string{"edit", "cas"}, withEditor(editor))
		require.NoError(t, err, "tctl edit errored")

		// Verify CA updates.
		gotCA1, gotCA2 := getCAs(t)
		ca1.SetRevision(gotCA1.GetRevision())
		ca2.SetRevision(gotCA2.GetRevision())
		assert.Equal(t, ca1, gotCA1, "CA1 edit failed")
		assert.Equal(t, ca2, gotCA2, "CA2 edit failed")
	})
}

func TestMultipleRoles(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()
	process, err := testenv.NewTeleportProcess(t.TempDir(), testenv.WithLogger(log))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})
	rootClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rootClient.Close() })

	roleNames := []string{"test-role1", "test-role2"}
	for _, name := range roleNames {
		expected, err := types.NewRole(name, types.RoleSpecV6{})
		require.NoError(t, err, "creating initial role resource")
		_, err = rootClient.CreateRole(ctx, expected.(*types.RoleV6))
		require.NoError(t, err, "persisting initial role resource")
	}

	roles, err := rootClient.GetRoles(ctx)
	require.NoError(t, err)

	editor := func(name string) error {
		f, err := os.Create(name)
		if err != nil {
			return trace.Wrap(err, "opening file to edit")
		}
		for _, role := range roles {
			if !slices.Contains(roleNames, role.GetName()) {
				continue
			}
			role.SetLogins(types.Allow, []string{"abcdef"})
		}

		collection := &roleCollection{roles: roles}
		return trace.NewAggregate(writeYAML(collection, f), f.Close())
	}

	// Edit the role and validate that the expected field is updated.
	_, err = runEditCommand(t, rootClient, []string{"edit", "roles"},
		withEditor(editor),
	)
	require.NoError(t, err, "expected editing role to succeed")

	for _, role := range roles {
		actual, err := rootClient.GetRole(ctx, role.GetName())
		require.NoError(t, err, "retrieving role after edit")

		assert.Equal(t, role.GetLogins(types.Allow), actual.GetLogins(types.Allow), "logins should have been modified by edit")
		require.Empty(t, cmp.Diff(role, actual, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

		switch {
		case !slices.Contains(roleNames, role.GetName()):
			require.Equal(t, role.GetRevision(), actual.GetRevision(), "revision should not have been modified by edit")
		default:
			require.NotEqual(t, role.GetRevision(), actual.GetRevision(), "revision should have been modified by edit")
		}
	}
}
