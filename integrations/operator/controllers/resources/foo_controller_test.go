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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	foov1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/foo/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
	"github.com/gravitational/teleport/lib/foos"
	"github.com/gravitational/teleport/lib/scopes"
)

var fooSpec = foov1.FooSpec_builder{
	Value: "value0",
}.Build()

type fooTestingPrimitives struct {
	setup *testSetup
	reconcilers.Resource153Adapter[*foov1.Foo]
}

func (g *fooTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *fooTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *fooTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	foo := foov1.Foo_builder{
		Kind:    foos.Kind,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		}.Build(),
		Spec: fooSpec,
	}.Build()
	_, err := g.setup.TeleportClient.FooClient().CreateFoo(ctx, foov1.CreateFooRequest_builder{
		Foo: foo,
	}.Build())
	return trace.Wrap(err)
}

func (g *fooTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (*foov1.Foo, error) {
	resp, err := g.setup.TeleportClient.FooClient().GetFoo(ctx, foov1.GetFooRequest_builder{
		Name: name,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetFoo(), nil
}

func (g *fooTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	_, err := g.setup.TeleportClient.FooClient().DeleteFoo(ctx, foov1.DeleteFooRequest_builder{
		Name: name,
	}.Build())
	return trace.Wrap(err)
}

func (g *fooTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	foo := &resourcesv1.TeleportFooV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportFooV1Spec)(fooSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, foo))
}

func (g *fooTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	foo := &resourcesv1.TeleportFooV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, foo))
}

func (g *fooTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportFooV1, error) {
	foo := &resourcesv1.TeleportFooV1{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, foo)
	return foo, trace.Wrap(err)
}

func (g *fooTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	foo, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	foo.Spec.Value = "value1"
	return trace.Wrap(g.setup.K8sClient.Update(ctx, foo))
}

func (g *fooTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *foov1.Foo, kubeResource *resourcesv1.TeleportFooV1) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		testlib.ProtoCompareOptions()...,
	)
	return diff == "", diff
}

func TestFooCreation(t *testing.T) {
	t.Parallel()
	test := &fooTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewFooV1Reconciler, test, testlib.WithResourceName(uuid.New().String()), testlib.WithScopesFeatures(scopes.Features{Enabled: true}))
}

func TestFooDeletionDrift(t *testing.T) {
	t.Parallel()
	test := &fooTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewFooV1Reconciler, test, testlib.WithResourceName(uuid.New().String()), testlib.WithScopesFeatures(scopes.Features{Enabled: true}))
}

func TestFooUpdate(t *testing.T) {
	t.Parallel()
	test := &fooTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewFooV1Reconciler, test, testlib.WithResourceName(uuid.New().String()), testlib.WithScopesFeatures(scopes.Features{Enabled: true}))
}
