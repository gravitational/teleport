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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
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
	testlib.ResourceCreationTest[types.Application, *resourcesv1.TeleportAppV3](t, test)
}

func TestTeleportAppV3DeletionDrift(t *testing.T) {
	test := &appV3TestingPrimitives{}
	testlib.ResourceDeletionDriftTest[types.Application, *resourcesv1.TeleportAppV3](t, test)
}

func TestTeleportAppV3Update(t *testing.T) {
	test := &appV3TestingPrimitives{}
	testlib.ResourceUpdateTest[types.Application, *resourcesv1.TeleportAppV3](t, test)
}
