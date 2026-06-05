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
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/access"
)

var scopedRoleSpec = accessv1.ScopedRoleSpec_builder{
	AssignableScopes: []string{"/staging"},
	Rules: []*accessv1.ScopedRule{
		accessv1.ScopedRule_builder{
			Resources: []string{"scoped_role"},
			Verbs:     []string{"readnosecrets", "list"},
		}.Build(),
	},
}.Build()

type scopedRoleTestingPrimitives struct {
	setup *testSetup
	reconcilers.Resource153Adapter[*accessv1.ScopedRole]
}

func (g *scopedRoleTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *scopedRoleTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *scopedRoleTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	role := accessv1.ScopedRole_builder{
		Kind:    access.KindScopedRole,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		}.Build(),
		Scope: "/staging",
		Spec:  scopedRoleSpec,
	}.Build()
	_, err := g.setup.TeleportClient.ScopedAccessServiceClient().CreateScopedRole(ctx, accessv1.CreateScopedRoleRequest_builder{
		Role: role,
	}.Build())
	return trace.Wrap(err)
}

func (g *scopedRoleTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (*accessv1.ScopedRole, error) {
	resp, err := g.setup.TeleportClient.ScopedAccessServiceClient().GetScopedRole(ctx, accessv1.GetScopedRoleRequest_builder{
		Name: name,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetRole(), nil
}

func (g *scopedRoleTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	_, err := g.setup.TeleportClient.ScopedAccessServiceClient().DeleteScopedRole(ctx, accessv1.DeleteScopedRoleRequest_builder{
		Name: name,
	}.Build())
	return trace.Wrap(err)
}

func (g *scopedRoleTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	role := &resourcesv1.TeleportScopedRoleV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Scope: "/staging",
		Spec:  (*resourcesv1.TeleportScopedRoleV1Spec)(scopedRoleSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, role))
}

func (g *scopedRoleTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	role := &resourcesv1.TeleportScopedRoleV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, role))
}

func (g *scopedRoleTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportScopedRoleV1, error) {
	role := &resourcesv1.TeleportScopedRoleV1{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, role)
	return role, trace.Wrap(err)
}

func (g *scopedRoleTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	role, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	role.Spec.AssignableScopes = []string{"/staging/aa", "/staging/bb"}
	role.Spec.Ssh = accessv1.ScopedRoleSSH_builder{
		HostSudoers: []string{"test"},
	}.Build()
	return trace.Wrap(g.setup.K8sClient.Update(ctx, role))
}

func (g *scopedRoleTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *accessv1.ScopedRole, kubeResource *resourcesv1.TeleportScopedRoleV1) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		testlib.ProtoCompareOptions()...,
	)
	return diff == "", diff
}

func TestScopedRoleCreation(t *testing.T) {
	t.Parallel()
	test := &scopedRoleTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewScopedRoleV1Reconciler, test, testlib.WithScopesFeatures(scopes.Features{Enabled: true}))
}

func TestScopedRoleDeletionDrift(t *testing.T) {
	t.Parallel()
	test := &scopedRoleTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewScopedRoleV1Reconciler, test, testlib.WithScopesFeatures(scopes.Features{Enabled: true}))
}

func TestScopedRoleUpdate(t *testing.T) {
	t.Parallel()
	test := &scopedRoleTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewScopedRoleV1Reconciler, test, testlib.WithScopesFeatures(scopes.Features{Enabled: true}))
}
