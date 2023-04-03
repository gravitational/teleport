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
	resourcesv2 "github.com/gravitational/teleport/operator/apis/resources/v2"
	"github.com/gravitational/teleport/operator/controllers/resources/testlib"
)

var samlSpec = &types.SAMLConnectorSpecV2{
	Issuer:                   "issuer",
	SSO:                      "sso",
	AssertionConsumerService: "acs",
	Audience:                 "audience",
	ServiceProviderIssuer:    "spi",
	AttributesToRoles: []types.AttributeMapping{{
		Name:  "test",
		Value: "test",
		Roles: []string{"testRoleA"},
	}},
}

type samlTestingPrimitives struct {
	setup *testSetup
}

func (g *samlTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *samlTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	err := teleportCreateDummyRole(ctx, "testRoleA", g.setup.TeleportClient)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(teleportCreateDummyRole(ctx, "testRoleB", g.setup.TeleportClient))
}

func (g *samlTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	saml, err := types.NewSAMLConnector(name, *samlSpec)
	if err != nil {
		return trace.Wrap(err)
	}
	saml.SetOrigin(types.OriginKubernetes)
	return trace.Wrap(g.setup.TeleportClient.UpsertSAMLConnector(ctx, saml))
}

func (g *samlTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.SAMLConnector, error) {
	return g.setup.TeleportClient.GetSAMLConnector(ctx, name, false)
}

func (g *samlTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteSAMLConnector(ctx, name))
}

func (g *samlTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	saml := &resourcesv2.TeleportSAMLConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv2.TeleportSAMLConnectorSpec(*samlSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, saml))
}

func (g *samlTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	saml := &resourcesv2.TeleportSAMLConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return g.setup.K8sClient.Delete(ctx, saml)
}

func (g *samlTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv2.TeleportSAMLConnector, error) {
	saml := &resourcesv2.TeleportSAMLConnector{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, saml)
	return saml, trace.Wrap(err)
}

func (g *samlTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	saml, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	saml.Spec.AttributesToRoles[0].Roles = []string{"testRoleA", "testRoleB"}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, saml))
}

func (g *samlTestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.SAMLConnector, kubeResource *resourcesv2.TeleportSAMLConnector) (bool, string) {
	teleportMap, _ := teleportResourceToMap(tResource)
	kubernetesMap, _ := teleportResourceToMap(kubeResource.ToTeleport())

	// Signing key pair is populated server-side here
	delete(teleportMap["spec"].(map[string]interface{}), "signing_key_pair")

	equal := cmp.Equal(teleportMap["spec"], kubernetesMap["spec"])
	if !equal {
		return equal, cmp.Diff(teleportMap["spec"], kubernetesMap["spec"])
	}

	return equal, ""
}

func TestSAMLConnectorCreation(t *testing.T) {
	test := &samlTestingPrimitives{}
	testlib.ResourceCreationTest[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector](t, test)
}

func TestSAMLConnectorDeletionDrift(t *testing.T) {
	test := &samlTestingPrimitives{}
	testlib.ResourceDeletionDriftTest[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector](t, test)
}

func TestSAMLConnectorUpdate(t *testing.T) {
	test := &samlTestingPrimitives{}
	testlib.ResourceUpdateTest[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector](t, test)
}
