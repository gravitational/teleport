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
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
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

func (g *samlTestingPrimitives) init(setup *testSetup) {
	g.setup = setup
}

func (g *samlTestingPrimitives) setupTeleportFixtures(ctx context.Context) error {
	err := teleportCreateDummyRole(ctx, "testRoleA", g.setup.tClient)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(teleportCreateDummyRole(ctx, "testRoleB", g.setup.tClient))
}

func (g *samlTestingPrimitives) createTeleportResource(ctx context.Context, name string) error {
	saml, err := types.NewSAMLConnector(name, *samlSpec)
	if err != nil {
		return trace.Wrap(err)
	}
	saml.SetOrigin(types.OriginKubernetes)
	return trace.Wrap(g.setup.tClient.UpsertSAMLConnector(ctx, saml))
}

func (g *samlTestingPrimitives) getTeleportResource(ctx context.Context, name string) (types.SAMLConnector, error) {
	return g.setup.tClient.GetSAMLConnector(ctx, name, false)
}

func (g *samlTestingPrimitives) deleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.tClient.DeleteSAMLConnector(ctx, name))
}

func (g *samlTestingPrimitives) createKubernetesResource(ctx context.Context, name string) error {
	saml := &resourcesv2.TeleportSAMLConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.namespace.Name,
		},
		Spec: resourcesv2.TeleportSAMLConnectorSpec(*samlSpec),
	}
	return trace.Wrap(g.setup.k8sClient.Create(ctx, saml))
}

func (g *samlTestingPrimitives) deleteKubernetesResource(ctx context.Context, name string) error {
	saml := &resourcesv2.TeleportSAMLConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.namespace.Name,
		},
	}
	return g.setup.k8sClient.Delete(ctx, saml)
}

func (g *samlTestingPrimitives) getKubernetesResource(ctx context.Context, name string) (*resourcesv2.TeleportSAMLConnector, error) {
	saml := &resourcesv2.TeleportSAMLConnector{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.namespace.Name,
	}
	err := g.setup.k8sClient.Get(ctx, obj, saml)
	return saml, trace.Wrap(err)
}

func (g *samlTestingPrimitives) modifyKubernetesResource(ctx context.Context, name string) error {
	saml, err := g.getKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	saml.Spec.AttributesToRoles[0].Roles = []string{"testRoleA", "testRoleB"}
	return trace.Wrap(g.setup.k8sClient.Update(ctx, saml))
}

func (g *samlTestingPrimitives) compareTeleportAndKubernetesResource(tResource types.SAMLConnector, kubeResource *resourcesv2.TeleportSAMLConnector) (bool, string) {
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
	testResourceCreation[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector](t, test)
}

func TestSAMLConnectorDeletionDrift(t *testing.T) {
	test := &samlTestingPrimitives{}
	testResourceDeletionDrift[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector](t, test)
}

func TestSAMLConnectorUpdate(t *testing.T) {
	test := &samlTestingPrimitives{}
	testResourceUpdate[types.SAMLConnector, *resourcesv2.TeleportSAMLConnector](t, test)
}
