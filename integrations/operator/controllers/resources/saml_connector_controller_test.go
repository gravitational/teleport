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
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv2 "github.com/gravitational/teleport/integrations/operator/apis/resources/v2"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var samlSpec = &types.SAMLConnectorSpecV2{
	Issuer:                   "issuer",
	SSO:                      "https://example.com",
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
	reconcilers.ResourceWithoutLabelsAdapter[types.SAMLConnector]
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
	_, err = g.setup.TeleportClient.CreateSAMLConnector(ctx, saml)
	return trace.Wrap(err)
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
	opts := testlib.CompareOptions(
		// SigningKeyPair is added server-side, it's expected
		cmpopts.IgnoreFields(types.SAMLConnectorSpecV2{}, "SigningKeyPair"),
	)
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), opts...)
	return diff == "", diff
}

// SAML tests are flaky in the CI for some reason.
// To troubleshoot them, samlTestingPrimitives supports the optional debug logger.
// This is only called in case of failed update test.
func (g *samlTestingPrimitives) DebugDrifts(t *testing.T, name string) {
	if g.setup.TeleportServer == nil {
		return
	}
	testClient := g.setup.TeleportClient
	uncachedClient := g.setup.TeleportServer.Process.GetAuthServer().Services
	cachedClient := g.setup.TeleportServer.Process.GetAuthServer()

	// t.Context() is already expired when this function is called by t.Cleanup().
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testResource, err := testClient.GetSAMLConnector(ctx, name, false)
	if err != nil {
		t.Log("Failed to get SAML Connector via the test client", err)
		return
	}
	uncachedResource, err := uncachedClient.GetSAMLConnector(ctx, name, false)
	if err != nil {
		t.Log("Failed to get SAML Connector via the uncached client", err)
		return
	}
	cachedResource, err := cachedClient.GetSAMLConnector(ctx, name, false)
	if err != nil {
		t.Log("Failed to get SAML Connector via the cached client", err)
		return
	}
	testJson, err := json.Marshal(testResource)
	if err != nil {
		t.Log("Failed to marshal test client SAML Connector", err)
		return
	}
	uncachedJson, err := json.Marshal(uncachedResource)
	if err != nil {
		t.Log("Failed to marshal uncached SAML Connector", err)
		return
	}
	cachedJson, err := json.Marshal(cachedResource)
	if err != nil {
		t.Log("Failed to marshal cached SAML Connector", err)
	}

	t.Log(testJson, uncachedJson, cachedJson)
}

func TestSAMLConnectorCreation(t *testing.T) {
	test := &samlTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewSAMLConnectorReconciler, test)
}

func TestSAMLConnectorDeletion(t *testing.T) {
	test := &samlTestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest(t, resources.NewSAMLConnectorReconciler, test)
}

func TestSAMLConnectorDeletionDrift(t *testing.T) {
	test := &samlTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewSAMLConnectorReconciler, test)
}

func TestSAMLConnectorUpdate(t *testing.T) {
	test := &samlTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewSAMLConnectorReconciler, test)
}
