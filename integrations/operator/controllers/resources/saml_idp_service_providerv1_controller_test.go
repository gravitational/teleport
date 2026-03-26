/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"slices"
	"testing"

	"github.com/crewjam/saml/samlsp"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
	"github.com/gravitational/teleport/lib/services"
)

var samlIdPServiceProviderSpec = &types.SAMLIdPServiceProviderSpecV1{
	EntityDescriptor: services.NewSAMLTestSPMetadata("https://example.com/saml/entity", "https://example.com/saml/acs"),
	EntityID:         "https://example.com/saml/entity",
	ACSURL:           "https://example.com/saml/acs",
	AttributeMapping: []*types.SAMLAttributeMapping{
		{
			Name:       "display_name",
			NameFormat: types.SAMLURINameFormat,
			Value:      "external.display_name",
		},
		{
			Name:       "email",
			NameFormat: types.SAMLURINameFormat,
			Value:      "external.email",
		},
	},
	Preset:     "",
	RelayState: "relay-state",
	LaunchURLs: []string{"https://example.com/saml/launch"},
}

type samlIdPServiceProviderTestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithLabelsAdapter[types.SAMLIdPServiceProvider]
}

func (g *samlIdPServiceProviderTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *samlIdPServiceProviderTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *samlIdPServiceProviderTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	sp, err := types.NewSAMLIdPServiceProvider(types.Metadata{
		Name: name,
	}, *samlIdPServiceProviderSpec)
	if err != nil {
		return trace.Wrap(err)
	}
	sp.SetOrigin(types.OriginKubernetes)
	return trace.Wrap(g.setup.TeleportClient.CreateSAMLIdPServiceProvider(ctx, sp))
}

func (g *samlIdPServiceProviderTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	return g.setup.TeleportClient.GetSAMLIdPServiceProvider(ctx, name)
}

func (g *samlIdPServiceProviderTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteSAMLIdPServiceProvider(ctx, name))
}

func (g *samlIdPServiceProviderTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	sp := &resourcesv1.TeleportSAMLIdPServiceProviderV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportSAMLIdPServiceProviderV1Spec(*samlIdPServiceProviderSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, sp))
}

func (g *samlIdPServiceProviderTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	sp := &resourcesv1.TeleportSAMLIdPServiceProviderV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, sp))
}

func (g *samlIdPServiceProviderTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportSAMLIdPServiceProviderV1, error) {
	sp := &resourcesv1.TeleportSAMLIdPServiceProviderV1{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, sp)
	return sp, trace.Wrap(err)
}

func (g *samlIdPServiceProviderTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	sp, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	sp.Spec.RelayState = "updated-relay-state"
	return trace.Wrap(g.setup.K8sClient.Update(ctx, sp))
}

func (g *samlIdPServiceProviderTestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.SAMLIdPServiceProvider, kubeResource *resourcesv1.TeleportSAMLIdPServiceProviderV1) (bool, string) {
	kubeTeleportResource := kubeResource.ToTeleport()
	diff := cmp.Diff(
		tResource,
		kubeTeleportResource,
		testlib.CompareOptions(
			// Tradeoff: we avoid byte-level entity_descriptor checks because the
			// server normalizes that field, and those assertions become brittle and
			// API-coupled. We still validate minimal descriptor semantics below to
			// catch controller mapping regressions for entity_id and acs_url.
			cmpopts.IgnoreFields(types.SAMLIdPServiceProviderSpecV1{}, "EntityDescriptor"),
		)...,
	)
	if diff != "" {
		return false, diff
	}

	descriptorDiff, err := compareEntityDescriptorSemantics(tResource.GetEntityDescriptor(), kubeTeleportResource)
	if err != nil {
		return false, trace.Wrap(err).Error()
	}
	return descriptorDiff == "", descriptorDiff
}

func TestSAMLIdPServiceProviderCreation(t *testing.T) {
	test := &samlIdPServiceProviderTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewSAMLIdPServiceProviderV1Reconciler, test)
}

func TestSAMLIdPServiceProviderDeletion(t *testing.T) {
	test := &samlIdPServiceProviderTestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest(t, resources.NewSAMLIdPServiceProviderV1Reconciler, test)
}

func TestSAMLIdPServiceProviderDeletionDrift(t *testing.T) {
	test := &samlIdPServiceProviderTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewSAMLIdPServiceProviderV1Reconciler, test)
}

func TestSAMLIdPServiceProviderUpdate(t *testing.T) {
	test := &samlIdPServiceProviderTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewSAMLIdPServiceProviderV1Reconciler, test)
}

// This test ensures the controller behavior for Teleport API validation
// failures. Specifically, the server rejects the request if both an
// EntityDescriptor and an EntityID+ACSURL are provided but they do not match.
// The CR status reports a reconciliation error.
func TestSAMLIdPServiceProviderCreateValidationError(t *testing.T) {
	ctx := t.Context()
	setup := testlib.SetupFakeKubeTestEnv(t)
	reconciler, err := resources.NewSAMLIdPServiceProviderV1Reconciler(setup.K8sClient, setup.TeleportClient)
	require.NoError(t, err)

	name := validRandomResourceName("saml-sp-")
	sp := &resourcesv1.TeleportSAMLIdPServiceProviderV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportSAMLIdPServiceProviderV1Spec(types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: services.NewSAMLTestSPMetadata(
				"https://example.com/saml/entity-from-descriptor",
				"https://example.com/saml/acs-from-descriptor",
			),
			EntityID: "https://example.com/saml/entity-from-top-level",
			ACSURL:   "https://example.com/saml/acs-from-top-level",
			AttributeMapping: []*types.SAMLAttributeMapping{
				{
					Name:       "email",
					NameFormat: types.SAMLURINameFormat,
					Value:      "external.email",
				},
			},
			RelayState: "relay-state",
			LaunchURLs: []string{"https://example.com/saml/launch"},
		}),
	}
	require.NoError(t, setup.K8sClient.Create(ctx, sp))

	req := reconcile.Request{
		NamespacedName: apimachinerytypes.NamespacedName{
			Namespace: setup.Namespace.Name,
			Name:      name,
		},
	}

	// First reconciliation should set the finalizer and exit.
	_, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	// Second reconciliation should attempt the Teleport create and surface the
	// server-side validation error.
	_, err = reconciler.Reconcile(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "entity ID parsed from the entity descriptor does not match")

	_, err = setup.TeleportClient.GetSAMLIdPServiceProvider(ctx, name)
	require.True(t, trace.IsNotFound(err), "Teleport resource should not be created when Teleport rejects spec validation")

	// Wait until the controller status update is persisted on the Kubernetes CR.
	fastEventually(t, func() bool {
		current := &resourcesv1.TeleportSAMLIdPServiceProviderV1{}
		getErr := setup.K8sClient.Get(ctx, kclient.ObjectKey{Name: name, Namespace: setup.Namespace.Name}, current)
		if getErr != nil {
			return false
		}
		condition := meta.FindStatusCondition(current.Status.Conditions, reconcilers.ConditionTypeSuccessfullyReconciled)
		return condition != nil &&
			condition.Status == metav1.ConditionFalse &&
			condition.Reason == reconcilers.ConditionReasonTeleportError
	})

	// Re-fetch the CR and assert the exact reconciliation condition values once
	// the eventually loop has observed the status update.
	current := &resourcesv1.TeleportSAMLIdPServiceProviderV1{}
	require.NoError(t, setup.K8sClient.Get(ctx, kclient.ObjectKey{Name: name, Namespace: setup.Namespace.Name}, current))
	condition := meta.FindStatusCondition(current.Status.Conditions, reconcilers.ConditionTypeSuccessfullyReconciled)
	require.NotNil(t, condition)
	require.Equal(t, metav1.ConditionFalse, condition.Status)
	require.Equal(t, reconcilers.ConditionReasonTeleportError, condition.Reason)

	// Defensive cleanup for consistency with other controller tests: this test
	// runs in an isolated fake environment, but we still drive finalizer removal.
	require.NoError(t, setup.K8sClient.Delete(ctx, sp))
	_, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
}

func compareEntityDescriptorSemantics(entityDescriptor string, expectedResource types.SAMLIdPServiceProvider) (string, error) {
	descriptor, err := samlsp.ParseMetadata([]byte(entityDescriptor))
	if err != nil {
		return "", trace.BadParameter("failed to parse Teleport entity_descriptor: %v", err)
	}

	if descriptor.EntityID != expectedResource.GetEntityID() {
		return cmp.Diff(expectedResource.GetEntityID(), descriptor.EntityID), nil
	}

	descriptorACSURLs := []string{}
	for _, ssoDescriptor := range descriptor.SPSSODescriptors {
		for _, acs := range ssoDescriptor.AssertionConsumerServices {
			descriptorACSURLs = append(descriptorACSURLs, acs.Location)
		}
	}
	if !slices.Contains(descriptorACSURLs, expectedResource.GetACSURL()) {
		return cmp.Diff([]string{expectedResource.GetACSURL()}, descriptorACSURLs), nil
	}
	return "", nil
}
