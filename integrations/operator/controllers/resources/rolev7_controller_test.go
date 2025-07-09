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
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var roleV7Spec = types.RoleSpecV6{
	Allow: types.RoleConditions{
		Logins:           []string{"foo"},
		KubernetesLabels: types.Labels{"env": {"dev", "prod"}},
		KubernetesResources: []types.KubernetesResource{
			{
				Kind:      "*",
				Namespace: "monitoring",
				Name:      "^prometheus-.*",
			},
		},
	},
	Deny: types.RoleConditions{},
}

type roleV7TestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithLabelsAdapter[types.Role]
}

func (g *roleV7TestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *roleV7TestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *roleV7TestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	role, err := types.NewRoleWithVersion(name, types.V7, roleV7Spec)
	if err != nil {
		return trace.Wrap(err)
	}
	role.SetOrigin(types.OriginKubernetes)
	_, err = g.setup.TeleportClient.CreateRole(ctx, role)
	return trace.Wrap(err)
}

func (g *roleV7TestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.Role, error) {
	return g.setup.TeleportClient.GetRole(ctx, name)
}

func (g *roleV7TestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteRole(ctx, name))
}

func (g *roleV7TestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	role := &resourcesv1.TeleportRoleV7{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportRoleV7Spec(roleV7Spec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, role))
}

func (g *roleV7TestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	role := &resourcesv1.TeleportRoleV7{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, role))
}

func (g *roleV7TestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportRoleV7, error) {
	role := &resourcesv1.TeleportRoleV7{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, role)
	return role, trace.Wrap(err)
}

func (g *roleV7TestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	role, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	role.Spec.Allow.Logins = []string{"foo", "bar"}
	return g.setup.K8sClient.Update(ctx, role)
}

func (g *roleV7TestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.Role, kubeResource *resourcesv1.TeleportRoleV7) (bool, string) {
	ignoreServerSideDefaults := []cmp.Option{
		cmpopts.IgnoreFields(types.RoleSpecV6{}, "Options"),
		cmpopts.IgnoreFields(types.RoleConditions{}, "Namespaces"),
	}
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions(ignoreServerSideDefaults...)...)
	return diff == "", diff
}

func TestTeleportRoleV7Creation(t *testing.T) {
	test := &roleV7TestingPrimitives{}
	testlib.ResourceCreationTest[types.Role, *resourcesv1.TeleportRoleV7](t, test)
}

func TestTeleportRoleV7DeletionDrift(t *testing.T) {
	test := &roleV7TestingPrimitives{}
	testlib.ResourceDeletionDriftTest[types.Role, *resourcesv1.TeleportRoleV7](t, test)
}

func TestTeleportRoleV7Update(t *testing.T) {
	test := &roleV7TestingPrimitives{}
	testlib.ResourceUpdateTest[types.Role, *resourcesv1.TeleportRoleV7](t, test)
}
