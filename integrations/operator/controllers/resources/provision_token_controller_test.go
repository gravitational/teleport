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

package resources_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var tokenSpec = &types.ProvisionTokenSpecV2{
	Roles: []types.SystemRole{types.RoleNode},
	Allow: []*types.TokenRule{
		{
			AWSAccount: "333333333333",
			AWSARN:     "arn:aws:sts::333333333333:assumed-role/teleport-node-role/i-*",
		},
	},
	JoinMethod: types.JoinMethodIAM,
}

var teleportTokenGVK = schema.GroupVersionKind{
	Group:   resourcesv2.GroupVersion.Group,
	Version: resourcesv2.GroupVersion.Version,
	Kind:    "TeleportProvisionToken",
}

// newProvisionTokenFromSpecNoExpire returns a new provision token with the given spec without expiration set.
func newProvisionTokenFromSpecNoExpire(token string, spec types.ProvisionTokenSpecV2) (types.ProvisionToken, error) {
	t := &types.ProvisionTokenV2{
		Metadata: types.Metadata{
			Name: token,
		},
		Spec: spec,
	}
	if err := t.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return t, nil
}

type tokenTestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithoutLabelsAdapter[types.ProvisionToken]
}

func (g *tokenTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *tokenTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	err := teleportCreateDummyRole(ctx, "testRoleA", g.setup.TeleportClient)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(teleportCreateDummyRole(ctx, "testRoleB", g.setup.TeleportClient))
}

func (g *tokenTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	token, err := newProvisionTokenFromSpecNoExpire(name, *tokenSpec)
	if err != nil {
		return trace.Wrap(err)
	}
	token.SetOrigin(types.OriginKubernetes)
	return trace.Wrap(g.setup.TeleportClient.UpsertToken(ctx, token))
}

func (g *tokenTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.ProvisionToken, error) {
	return g.setup.TeleportClient.GetToken(ctx, name)
}

func (g *tokenTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteToken(ctx, name))
}

func (g *tokenTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	token := &resourcesv2.TeleportProvisionToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv2.TeleportProvisionTokenSpec(*tokenSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, token))
}

func (g *tokenTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	saml := &resourcesv2.TeleportProvisionToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return g.setup.K8sClient.Delete(ctx, saml)
}

func (g *tokenTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv2.TeleportProvisionToken, error) {
	saml := &resourcesv2.TeleportProvisionToken{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, saml)
	return saml, trace.Wrap(err)
}

func (g *tokenTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	saml, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	saml.Spec.Roles = []types.SystemRole{types.RoleNode, types.RoleProxy}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, saml))
}

func (g *tokenTestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.ProvisionToken, kubeResource *resourcesv2.TeleportProvisionToken) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions()...)
	return diff == "", diff
}

func TestProvisionTokenCreation(t *testing.T) {
	test := &tokenTestingPrimitives{}
	testlib.ResourceCreationTest[types.ProvisionToken, *resourcesv2.TeleportProvisionToken](t, test)
}

func TestProvisionTokenDeletionDrift(t *testing.T) {
	test := &tokenTestingPrimitives{}
	testlib.ResourceDeletionDriftTest[types.ProvisionToken, *resourcesv2.TeleportProvisionToken](t, test)
}

func TestProvisionTokenUpdate(t *testing.T) {
	test := &tokenTestingPrimitives{}
	testlib.ResourceUpdateTest[types.ProvisionToken, *resourcesv2.TeleportProvisionToken](t, test)
}

// This test checks the operator can create Token resources in Teleport for a
// typical GitHub Action MachineID setup: the token allows a bot to join from
// GitHub Actions.
//
// Proto messages for GitHub provision tokens have one
// specificity: the Rule message is defined as a sub message of
// ProvisionTokenSpecV2GitHub. this caused several issues in the CRD generation
// tooling and required its own dedicated test.
func TestProvisionTokenCreation_GitHubBot(t *testing.T) {
	// Test setup
	ctx := context.Background()
	setup := setupTestEnv(t)
	require.NoError(t, teleportCreateDummyRole(ctx, "a", setup.TeleportClient))

	tokenSpecYAML := `
roles: 
  - Bot
join_method: github
bot_name: my-bot
github:
  allow:
    - repository: org/repo
`
	expectedSpec := &types.ProvisionTokenSpecV2{
		Roles:      types.SystemRoles{types.RoleBot},
		JoinMethod: types.JoinMethodGitHub,
		BotName:    "my-bot",
		GitHub:     &types.ProvisionTokenSpecV2GitHub{Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{{Repository: "org/repo"}}},
	}

	// Creating the Kubernetes resource. We are using an untyped client to be able to create invalid resources.
	tokenManifest := map[string]any{}
	err := yaml.Unmarshal([]byte(tokenSpecYAML), &tokenManifest)
	require.NoError(t, err)

	tokenName := validRandomResourceName("token-")

	obj, err := reconcilers.GetUnstructuredObjectFromGVK(teleportTokenGVK)
	require.NoError(t, err)
	obj.Object["spec"] = tokenManifest
	obj.SetName(tokenName)
	obj.SetNamespace(setup.Namespace.Name)

	// Doing the test: we create the TeleportProvisionToken in Kubernetes
	err = setup.K8sClient.Create(ctx, obj)
	require.NoError(t, err)

	// Then we wait for the token to be created in Teleport
	fastEventually(t, func() bool {
		tToken, err := setup.TeleportClient.GetToken(ctx, tokenName)
		// If the resource creation should succeed we check the resource was found and validate ownership labels
		if trace.IsNotFound(err) {
			return false
		}
		require.NoError(t, err)

		require.Equal(t, tokenName, tToken.GetName())
		require.Contains(t, tToken.GetMetadata().Labels, types.OriginLabel)
		require.Equal(t, types.OriginKubernetes, tToken.GetMetadata().Labels[types.OriginLabel])
		expectedToken := &types.ProvisionTokenV2{
			Metadata: types.Metadata{},
			Spec:     *expectedSpec,
		}
		_ = expectedToken.CheckAndSetDefaults()
		compareTokenSpecs(t, expectedToken, tToken)

		return true
	})
	// Test Teardown

	require.NoError(t, setup.K8sClient.Delete(ctx, obj))
	// We wait for the role deletion in Teleport
	fastEventually(t, func() bool {
		_, err := setup.TeleportClient.GetToken(ctx, tokenName)
		return trace.IsNotFound(err)
	})
}

func compareTokenSpecs(t *testing.T, expectedUser, actualUser types.ProvisionToken) {
	expected, err := teleportResourceToMap(expectedUser)
	require.NoError(t, err)
	actual, err := teleportResourceToMap(actualUser)
	require.NoError(t, err)

	require.Equal(t, expected["spec"], actual["spec"])
}
