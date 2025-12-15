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

var databaseV3Spec = types.DatabaseSpecV3{
	Protocol: types.DatabaseProtocolPostgreSQL,
	URI:      "database.namespace.svc.cluster.local:5432",
}

type databaseV3TestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithLabelsAdapter[types.Database]
}

func (g *databaseV3TestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *databaseV3TestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *databaseV3TestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	meta := types.Metadata{
		Name: name,
	}
	database, err := types.NewDatabaseV3(meta, databaseV3Spec)
	if err != nil {
		return trace.Wrap(err)
	}
	database.SetOrigin(types.OriginKubernetes)
	return trace.Wrap(g.setup.TeleportClient.CreateDatabase(ctx, database))
}

func (g *databaseV3TestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.Database, error) {
	return g.setup.TeleportClient.GetDatabase(ctx, name)
}

func (g *databaseV3TestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteDatabase(ctx, name))
}

func (g *databaseV3TestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	database := &resourcesv1.TeleportDatabaseV3{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportDatabaseV3Spec(databaseV3Spec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, database))
}

func (g *databaseV3TestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	database := &resourcesv1.TeleportDatabaseV3{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, database))
}

func (g *databaseV3TestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportDatabaseV3, error) {
	database := &resourcesv1.TeleportDatabaseV3{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, database)
	return database, trace.Wrap(err)
}

func (g *databaseV3TestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	database, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	database.Spec.Protocol = types.DatabaseProtocolMySQL
	return g.setup.K8sClient.Update(ctx, database)
}

func (g *databaseV3TestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.Database, kubeResource *resourcesv1.TeleportDatabaseV3) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions()...)
	return diff == "", diff
}

func TestTeleportDatabaseV3Creation(t *testing.T) {
	test := &databaseV3TestingPrimitives{}
	testlib.ResourceCreationTest[types.Database, *resourcesv1.TeleportDatabaseV3](t, test)
}

func TestTeleportDatabaseV3DeletionDrift(t *testing.T) {
	test := &databaseV3TestingPrimitives{}
	testlib.ResourceDeletionDriftTest[types.Database, *resourcesv1.TeleportDatabaseV3](t, test)
}

func TestTeleportDatabaseV3Update(t *testing.T) {
	test := &databaseV3TestingPrimitives{}
	testlib.ResourceUpdateTest[types.Database, *resourcesv1.TeleportDatabaseV3](t, test)
}
