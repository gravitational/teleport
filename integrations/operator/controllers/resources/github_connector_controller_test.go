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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/types"
	resourcesv3 "github.com/gravitational/teleport/integrations/operator/apis/resources/v3"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/secretlookup"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var githubSpec = types.GithubConnectorSpecV3{
	ClientID:      "client id",
	ClientSecret:  "client secret",
	RedirectURL:   "https://redirect",
	TeamsToLogins: nil,
	Display:       "",
	TeamsToRoles: []types.TeamRolesMapping{{
		Organization: "test",
		Team:         "test",
		Roles:        []string{"test"},
	}},
}

type githubTestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithoutLabelsAdapter[types.GithubConnector]
}

func (g *githubTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *githubTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *githubTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	github, err := types.NewGithubConnector(name, githubSpec)
	if err != nil {
		return trace.Wrap(err)
	}
	github.SetOrigin(types.OriginKubernetes)
	_, err = g.setup.TeleportClient.CreateGithubConnector(ctx, github)
	return trace.Wrap(err)
}

func (g *githubTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.GithubConnector, error) {
	return g.setup.TeleportClient.GetGithubConnector(ctx, name, true)
}

func (g *githubTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteGithubConnector(ctx, name))
}

func (g *githubTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	github := &resourcesv3.TeleportGithubConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv3.TeleportGithubConnectorSpec(githubSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, github))
}

func (g *githubTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	github := &resourcesv3.TeleportGithubConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, github))
}

func (g *githubTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv3.TeleportGithubConnector, error) {
	github := &resourcesv3.TeleportGithubConnector{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, github)
	return github, trace.Wrap(err)
}

func (g *githubTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	github, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	github.Spec.TeamsToRoles[0].Roles = []string{"foo", "bar"}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, github))
}

func (g *githubTestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.GithubConnector, kubeResource *resourcesv3.TeleportGithubConnector) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions()...)
	return diff == "", diff
}

func TestGithubConnectorCreation(t *testing.T) {
	test := &githubTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewGithubConnectorReconciler, test)
}

func TestGithubConnectorDeletion(t *testing.T) {
	test := &githubTestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest(t, resources.NewGithubConnectorReconciler, test)
}

func TestGithubConnectorDeletionDrift(t *testing.T) {
	test := &githubTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewGithubConnectorReconciler, test)
}

func TestGithubConnectorUpdate(t *testing.T) {
	test := &githubTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewGithubConnectorReconciler, test)
}

func TestGithubConnectorSecretLookup(t *testing.T) {
	test := &githubTestingPrimitives{}
	setup := testlib.SetupFakeKubeTestEnv(t)
	test.Init(setup)
	ctx := context.Background()

	crName := validRandomResourceName("github")
	secretName := validRandomResourceName("github-secret")
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
		// Real kube servers convert stringData into data.
		// The fake client does not, so we must use Data instead.
		Data: map[string][]byte{
			secretKey: []byte(secretValue),
		},
		StringData: map[string]string{
			secretKey: secretValue,
		},
		Type: v1.SecretTypeOpaque,
	}
	kubeClient := setup.K8sClient
	require.NoError(t, kubeClient.Create(ctx, secret))

	github := &resourcesv3.TeleportGithubConnector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: setup.Namespace.Name,
		},
		Spec: resourcesv3.TeleportGithubConnectorSpec(githubSpec),
	}

	github.Spec.ClientSecret = "secret://" + secretName + "/" + secretKey

	require.NoError(t, kubeClient.Create(ctx, github))

	reconciler, err := resources.NewGithubConnectorReconciler(kubeClient, setup.TeleportClient)
	require.NoError(t, err)
	// Test execution: Kick off the reconciliation.
	req := reconcile.Request{
		NamespacedName: apimachinerytypes.NamespacedName{
			Namespace: setup.Namespace.Name,
			Name:      crName,
		},
	}
	// First reconciliation should set the finalizer and exit.
	_, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	// Second reconciliation should create the Teleport resource.
	// In a real cluster we should receive the event of our own finalizer change
	// and this wakes us for a second round.
	_, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)

	testlib.FastEventually(t, func() bool {
		gh, err := test.GetTeleportResource(ctx, crName)
		if err != nil {
			return false
		}
		return gh.GetClientSecret() == secretValue
	})
}
