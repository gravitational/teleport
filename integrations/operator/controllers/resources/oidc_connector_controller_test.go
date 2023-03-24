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

package resources

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
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

func (g *oidcTestingPrimitives) init(setup *testSetup) {
	g.setup = setup
}

func (g *oidcTestingPrimitives) setupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *oidcTestingPrimitives) createTeleportResource(ctx context.Context, name string) error {
	oidc, err := types.NewOIDCConnector(name, oidcSpec)
	if err != nil {
		return trace.Wrap(err)
	}
	oidc.SetOrigin(types.OriginKubernetes)
	return trace.Wrap(g.setup.tClient.UpsertOIDCConnector(ctx, oidc))
}

func (g *oidcTestingPrimitives) getTeleportResource(ctx context.Context, name string) (types.OIDCConnector, error) {
	return g.setup.tClient.GetOIDCConnector(ctx, name, true)
}

func (g *oidcTestingPrimitives) deleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.tClient.DeleteOIDCConnector(ctx, name))
}

func (g *oidcTestingPrimitives) createKubernetesResource(ctx context.Context, name string) error {
	oidc := &resourcesv3.TeleportOIDCConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.namespace.Name,
		},
		Spec: resourcesv3.TeleportOIDCConnectorSpec(oidcSpec),
	}
	return trace.Wrap(g.setup.k8sClient.Create(ctx, oidc))
}

func (g *oidcTestingPrimitives) deleteKubernetesResource(ctx context.Context, name string) error {
	oidc := &resourcesv3.TeleportOIDCConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.namespace.Name,
		},
	}
	return trace.Wrap(g.setup.k8sClient.Delete(ctx, oidc))
}

func (g *oidcTestingPrimitives) getKubernetesResource(ctx context.Context, name string) (*resourcesv3.TeleportOIDCConnector, error) {
	oidc := &resourcesv3.TeleportOIDCConnector{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.namespace.Name,
	}
	err := g.setup.k8sClient.Get(ctx, obj, oidc)
	return oidc, trace.Wrap(err)
}

func (g *oidcTestingPrimitives) modifyKubernetesResource(ctx context.Context, name string) error {
	oidc, err := g.getKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	oidc.Spec.RedirectURLs = []string{"https://redirect1", "https://redirect2"}
	return g.setup.k8sClient.Update(ctx, oidc)
}

func (g *oidcTestingPrimitives) compareTeleportAndKubernetesResource(tResource types.OIDCConnector, kubeResource *resourcesv3.TeleportOIDCConnector) (bool, string) {
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
	testResourceCreation[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector](t, test)
}

func TestOIDCConnectorDeletionDrift(t *testing.T) {
	test := &oidcTestingPrimitives{}
	testResourceDeletionDrift[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector](t, test)
}

func TestOIDCConnectorUpdate(t *testing.T) {
	test := &oidcTestingPrimitives{}
	testResourceUpdate[types.OIDCConnector, *resourcesv3.TeleportOIDCConnector](t, test)
}
