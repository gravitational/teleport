/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var appV3Spec = types.AppSpecV3{
	URI: "http://test.namespace.svc.cluster.local:8080",
	Rewrite: &types.Rewrite{
		Headers: []*types.Header{
			{
				Name:  "C-Custom-Header",
				Value: "example",
			},
		},
	},
}

type appV3TestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithLabelsAdapter[types.Application]
}

func (g *appV3TestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *appV3TestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *appV3TestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	meta := types.Metadata{
		Name: name,
	}
	app, err := types.NewAppV3(meta, appV3Spec)
	if err != nil {
		return trace.Wrap(err)
	}
	app.SetOrigin(types.OriginKubernetes)
	return trace.Wrap(g.setup.TeleportClient.CreateApp(ctx, app))
}

func (g *appV3TestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.Application, error) {
	return g.setup.TeleportClient.GetApp(ctx, name)
}

func (g *appV3TestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteApp(ctx, name))
}

func (g *appV3TestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	app := &resourcesv1.TeleportAppV3{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportAppV3Spec(appV3Spec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, app))
}

func (g *appV3TestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	app := &resourcesv1.TeleportAppV3{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, app))
}

func (g *appV3TestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportAppV3, error) {
	app := &resourcesv1.TeleportAppV3{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, app)
	return app, trace.Wrap(err)
}

func (g *appV3TestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	app, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	app.Spec.Rewrite.Headers = append(app.Spec.Rewrite.Headers, &types.Header{Name: "Content-Type", Value: "application/json"})
	return g.setup.K8sClient.Update(ctx, app)
}

func (g *appV3TestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.Application, kubeResource *resourcesv1.TeleportAppV3) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions()...)
	return diff == "", diff
}

func TestTeleportAppV3Creation(t *testing.T) {
	test := &appV3TestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewAppV3Reconciler, test)
}

func TestTeleportAppV3Deletion(t *testing.T) {
	test := &appV3TestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest(t, resources.NewAppV3Reconciler, test)
}

func TestTeleportAppV3DeletionDrift(t *testing.T) {
	test := &appV3TestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewAppV3Reconciler, test)
}

func TestTeleportAppV3Update(t *testing.T) {
	test := &appV3TestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewAppV3Reconciler, test)
}

func TestTeleportAppV3AllNamespacesSameName(t *testing.T) {
	ctx := t.Context()

	setup := testlib.SetupFakeKubeTestEnv(t)
	reconciler, err := resources.NewAppV3ReconcilerWithNamespaceSuffix(setup.K8sClient, setup.TeleportClient, true)
	require.NoError(t, err)

	secondNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testlib.ValidRandomResourceName("ns-")}}
	require.NoError(t, setup.K8sClient.Create(ctx, secondNS))

	resourceName := testlib.ValidRandomResourceName("app-")

	app1 := &resourcesv1.TeleportAppV3{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportAppV3Spec(appV3Spec),
	}
	app2 := &resourcesv1.TeleportAppV3{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: secondNS.Name,
		},
		Spec: resourcesv1.TeleportAppV3Spec(appV3Spec),
	}

	require.NoError(t, setup.K8sClient.Create(ctx, app1))
	require.NoError(t, setup.K8sClient.Create(ctx, app2))

	req1 := reconcile.Request{NamespacedName: apimachinerytypes.NamespacedName{Namespace: app1.Namespace, Name: app1.Name}}
	req2 := reconcile.Request{NamespacedName: apimachinerytypes.NamespacedName{Namespace: app2.Namespace, Name: app2.Name}}

	_, err = reconciler.Reconcile(ctx, req1)
	require.NoError(t, err)
	_, err = reconciler.Reconcile(ctx, req1)
	require.NoError(t, err)

	_, err = reconciler.Reconcile(ctx, req2)
	require.NoError(t, err)
	_, err = reconciler.Reconcile(ctx, req2)
	require.NoError(t, err)

	name1 := resourceName + "-" + app1.Namespace
	name2 := resourceName + "-" + app2.Namespace

	testlib.FastEventuallyWithT(t, func(t *assert.CollectT) {
		_, err := setup.TeleportClient.GetApp(ctx, name1)
		require.NoError(t, err)
		_, err = setup.TeleportClient.GetApp(ctx, name2)
		require.NoError(t, err)
	})
}
