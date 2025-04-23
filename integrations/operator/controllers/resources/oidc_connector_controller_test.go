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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/secretlookup"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var oidcSpec = types.OIDCConnectorSpecV3{
	IssuerURL:    "https://issuer",
	ClientID:     "client id",
	ClientSecret: "client secret",
	ClaimsToRoles: []types.ClaimMapping{{
		Claim: "claim",
		Value: "value",
		Roles: []string{"roleA"},
	}},
	RedirectURLs: []string{"https://redirect"},
	MaxAge:       &types.MaxAge{Value: types.Duration(time.Hour)},
}

type oidcTestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithoutLabelsAdapter[types.OIDCConnector]
}

func (g *oidcTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *oidcTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *oidcTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	oidc, err := types.NewOIDCConnector(name, oidcSpec)
	if err != nil {
		return trace.Wrap(err)
	}
	oidc.SetOrigin(types.OriginKubernetes)
	_, err = g.setup.TeleportClient.CreateOIDCConnector(ctx, oidc)
	return trace.Wrap(err)
}

func (g *oidcTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.OIDCConnector, error) {
	return g.setup.TeleportClient.GetOIDCConnector(ctx, name, true)
}

func (g *oidcTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteOIDCConnector(ctx, name))
}

func (g *oidcTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	oidc := &resourcesv3.TeleportOIDCConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv3.TeleportOIDCConnectorSpec(oidcSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, oidc))
}

func (g *oidcTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	oidc := &resourcesv3.TeleportOIDCConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, oidc))
}

func (g *oidcTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv3.TeleportOIDCConnector, error) {
	oidc := &resourcesv3.TeleportOIDCConnector{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, oidc)
	return oidc, trace.Wrap(err)
}

func (g *oidcTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	oidc, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	oidc.Spec.RedirectURLs = []string{"https://redirect1", "https://redirect2"}
	return g.setup.K8sClient.Update(ctx, oidc)
}

func (g *oidcTestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.OIDCConnector, kubeResource *resourcesv3.TeleportOIDCConnector) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions()...)
	return diff == "", diff
}

func TestOIDCConnectorCreation(t *testing.T) {
	test := &oidcTestingPrimitives{}
	testlib.ResourceCreationTest[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector](t, test)
}

func TestOIDCConnectorDeletionDrift(t *testing.T) {
	test := &oidcTestingPrimitives{}
	testlib.ResourceDeletionDriftTest[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector](t, test)
}

func TestOIDCConnectorUpdate(t *testing.T) {
	test := &oidcTestingPrimitives{}
	testlib.ResourceUpdateTest[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector](t, test)
}

func TestOIDCConnectorSecretLookup(t *testing.T) {
	test := &oidcTestingPrimitives{}
	setup := testlib.SetupTestEnv(t)
	test.Init(setup)
	ctx := context.Background()

	crName := validRandomResourceName("oidc")
	secretName := validRandomResourceName("oidc-secret")
	secretKey := "client-secret"
	secretValue := validRandomResourceName("secret-value")

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: setup.Namespace.Name,
			Annotations: map[string]string{
				secretlookup.AllowLookupAnnotation: crName,
			},
		},
		StringData: map[string]string{
			secretKey: secretValue,
		},
		Type: v1.SecretTypeOpaque,
	}
	kubeClient := setup.K8sClient
	require.NoError(t, kubeClient.Create(ctx, secret))

	oidc := &resourcesv3.TeleportOIDCConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: setup.Namespace.Name,
		},
		Spec: resourcesv3.TeleportOIDCConnectorSpec(oidcSpec),
	}

	oidc.Spec.ClientSecret = "secret://" + secretName + "/" + secretKey

	require.NoError(t, kubeClient.Create(ctx, oidc))

	testlib.FastEventually(t, func() bool {
		oidc, err := test.GetTeleportResource(ctx, crName)
		if err != nil {
			return false
		}
		return oidc.GetClientSecret() == secretValue
	})
}
