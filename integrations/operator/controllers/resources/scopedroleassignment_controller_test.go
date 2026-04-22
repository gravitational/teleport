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

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
	"github.com/gravitational/teleport/lib/scopes/access"
)

var scopedRoleAssignmentSpec = &accessv1.ScopedRoleAssignmentSpec{
	User: "test-user",
	Assignments: []*accessv1.Assignment{
		{
			Role:  "test-role",
			Scope: "/staging",
		},
	},
}

type scopedRoleAssignmentTestingPrimitives struct {
	setup *testSetup
	reconcilers.Resource153Adapter[*accessv1.ScopedRoleAssignment]
}

func (g *scopedRoleAssignmentTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *scopedRoleAssignmentTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *scopedRoleAssignmentTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	assignment := &accessv1.ScopedRoleAssignment{
		Kind:    access.KindScopedRoleAssignment,
		Version: types.V1,
		SubKind: access.SubKindDynamic,
		Metadata: &headerv1.Metadata{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		},
		Scope: "/staging",
		Spec:  scopedRoleAssignmentSpec,
	}
	_, err := g.setup.TeleportClient.ScopedAccessServiceClient().CreateScopedRoleAssignment(ctx, &accessv1.CreateScopedRoleAssignmentRequest{
		Assignment: assignment,
	})
	return trace.Wrap(err)
}

func (g *scopedRoleAssignmentTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (*accessv1.ScopedRoleAssignment, error) {
	resp, err := g.setup.TeleportClient.ScopedAccessServiceClient().GetScopedRoleAssignment(ctx, &accessv1.GetScopedRoleAssignmentRequest{
		Name:    name,
		SubKind: access.SubKindDynamic,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetAssignment(), nil
}

func (g *scopedRoleAssignmentTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	_, err := g.setup.TeleportClient.ScopedAccessServiceClient().DeleteScopedRoleAssignment(ctx, &accessv1.DeleteScopedRoleAssignmentRequest{
		Name:    name,
		SubKind: access.SubKindDynamic,
	})
	return trace.Wrap(err)
}

func (g *scopedRoleAssignmentTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	assignment := &resourcesv1.TeleportScopedRoleAssignmentV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Scope: "/staging",
		Spec:  (*resourcesv1.TeleportScopedRoleAssignmentV1Spec)(scopedRoleAssignmentSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, assignment))
}

func (g *scopedRoleAssignmentTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	assignment := &resourcesv1.TeleportScopedRoleAssignmentV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, assignment))
}

func (g *scopedRoleAssignmentTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportScopedRoleAssignmentV1, error) {
	assignment := &resourcesv1.TeleportScopedRoleAssignmentV1{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, assignment)
	return assignment, trace.Wrap(err)
}

func (g *scopedRoleAssignmentTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	assignment, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	assignment.Spec.Assignments = []*accessv1.Assignment{
		{
			Role:  "test-role",
			Scope: "/staging/aa",
		},
	}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, assignment))
}

func (g *scopedRoleAssignmentTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *accessv1.ScopedRoleAssignment, kubeResource *resourcesv1.TeleportScopedRoleAssignmentV1) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		testlib.ProtoCompareOptions()...,
	)
	return diff == "", diff
}

func TestScopedRoleAssignmentCreation(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	test := &scopedRoleAssignmentTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewScopedRoleAssignmentV1Reconciler, test, testlib.WithResourceName(uuid.New().String()))
}

func TestScopedRoleAssignmentDeletionDrift(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	test := &scopedRoleAssignmentTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewScopedRoleAssignmentV1Reconciler, test, testlib.WithResourceName(uuid.New().String()))
}

func TestScopedRoleAssignmentUpdate(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	test := &scopedRoleAssignmentTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewScopedRoleAssignmentV1Reconciler, test, testlib.WithResourceName(uuid.New().String()))
}
