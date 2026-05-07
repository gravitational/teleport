/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var workloadIdentitySpec = &workloadidentityv1.WorkloadIdentitySpec{
	Rules: &workloadidentityv1.WorkloadIdentityRules{
		Allow: []*workloadidentityv1.WorkloadIdentityRule{
			{
				Conditions: []*workloadidentityv1.WorkloadIdentityCondition{
					{
						Attribute: "user.name",
						Operator: &workloadidentityv1.WorkloadIdentityCondition_Eq{
							Eq: &workloadidentityv1.WorkloadIdentityConditionEq{
								Value: "userA",
							},
						},
					},
				},
			},
		},
	},
	Spiffe: &workloadidentityv1.WorkloadIdentitySPIFFE{
		Id: "/test",
	},
}

type workloadIdentityTestingPrimitives struct {
	setup *testSetup
	reconcilers.Resource153Adapter[*workloadidentityv1.WorkloadIdentity]
}

func (g *workloadIdentityTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *workloadIdentityTestingPrimitives) SetupTeleportFixtures(
	ctx context.Context,
) error {
	return nil
}

func (g *workloadIdentityTestingPrimitives) CreateTeleportResource(
	ctx context.Context, name string,
) error {
	resource := &workloadidentityv1.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		},
		Spec: workloadIdentitySpec,
	}
	_, err := g.setup.TeleportClient.
		WorkloadIdentityResourceServiceClient().
		CreateWorkloadIdentity(
			ctx,
			&workloadidentityv1.CreateWorkloadIdentityRequest{
				WorkloadIdentity: resource,
			},
		)
	return trace.Wrap(err)
}

func (g *workloadIdentityTestingPrimitives) GetTeleportResource(
	ctx context.Context, name string,
) (*workloadidentityv1.WorkloadIdentity, error) {
	resp, err := g.setup.TeleportClient.
		WorkloadIdentityResourceServiceClient().
		GetWorkloadIdentity(
			ctx, &workloadidentityv1.GetWorkloadIdentityRequest{Name: name},
		)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (g *workloadIdentityTestingPrimitives) DeleteTeleportResource(
	ctx context.Context, name string,
) error {
	_, err := g.setup.TeleportClient.
		WorkloadIdentityResourceServiceClient().
		DeleteWorkloadIdentity(
			ctx, &workloadidentityv1.DeleteWorkloadIdentityRequest{Name: name},
		)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (g *workloadIdentityTestingPrimitives) CreateKubernetesResource(
	ctx context.Context, name string,
) error {
	bot := &resourcesv1.TeleportWorkloadIdentityV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportWorkloadIdentityV1Spec)(workloadIdentitySpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, bot))
}

func (g *workloadIdentityTestingPrimitives) DeleteKubernetesResource(
	ctx context.Context, name string,
) error {
	bot := &resourcesv1.TeleportWorkloadIdentityV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, bot))
}

func (g *workloadIdentityTestingPrimitives) GetKubernetesResource(
	ctx context.Context, name string,
) (*resourcesv1.TeleportWorkloadIdentityV1, error) {
	bot := &resourcesv1.TeleportWorkloadIdentityV1{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, bot)
	return bot, trace.Wrap(err)
}

func (g *workloadIdentityTestingPrimitives) ModifyKubernetesResource(
	ctx context.Context, name string,
) error {
	resource, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	resource.Spec.Spiffe.Id = "/changed"
	return trace.Wrap(g.setup.K8sClient.Update(ctx, resource))
}

func (g *workloadIdentityTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *workloadidentityv1.WorkloadIdentity,
	kubeResource *resourcesv1.TeleportWorkloadIdentityV1,
) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		testlib.ProtoCompareOptions()...,
	)
	return diff == "", diff
}

func TestWorkloadIdentityCreation(t *testing.T) {
	test := &workloadIdentityTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewWorkloadIdentityV1Reconciler, test)
}

func TestWorkloadIdentityDeletion(t *testing.T) {
	test := &workloadIdentityTestingPrimitives{}
	testlib.ResourceDeletionSynchronousTest(t, resources.NewWorkloadIdentityV1Reconciler, test)
}

func TestWorkloadIdentityDeletionDrift(t *testing.T) {
	test := &workloadIdentityTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewWorkloadIdentityV1Reconciler, test)
}

func TestWorkloadIdentityUpdate(t *testing.T) {
	test := &workloadIdentityTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewWorkloadIdentityV1Reconciler, test)
}
