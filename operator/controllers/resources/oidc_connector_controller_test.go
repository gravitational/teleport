/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv3 "github.com/gravitational/teleport/operator/apis/resources/v3"
	"github.com/gravitational/teleport/operator/controllers/resources/testlib"
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
}

type oidcTestingPrimitives struct {
	setup *testSetup
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
	return trace.Wrap(g.setup.TeleportClient.UpsertOIDCConnector(ctx, oidc))
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
	teleportMap, _ := teleportResourceToMap(tResource)
	kubernetesMap, _ := teleportResourceToMap(kubeResource.ToTeleport())

	equal := cmp.Equal(teleportMap["spec"], kubernetesMap["spec"])
	if !equal {
		return equal, cmp.Diff(teleportMap["spec"], kubernetesMap["spec"])
	}

	return equal, ""
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
