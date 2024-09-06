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

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var opensshEICEServerV2Spec = types.ServerSpecV2{
	Addr:     "127.0.0.1:22",
	Hostname: "test.local",
	CloudMetadata: &types.CloudMetadata{AWS: &types.AWSInfo{
		AccountID:   "123",
		InstanceID:  "123",
		Region:      "us-east-1",
		VPCID:       "123",
		Integration: "foo",
		SubnetID:    "123",
	}},
}

type opensshEICEServerV2TestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithLabelsAdapter[types.Server]
}

func (g *opensshEICEServerV2TestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *opensshEICEServerV2TestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *opensshEICEServerV2TestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	node, err := types.NewNode(name, types.SubKindOpenSSHEICENode, opensshEICEServerV2Spec, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	node.SetOrigin(types.OriginKubernetes)
	_, err = g.setup.TeleportClient.UpsertNode(ctx, node)
	return trace.Wrap(err)
}

func (g *opensshEICEServerV2TestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.Server, error) {
	return g.setup.TeleportClient.GetNode(ctx, defaults.Namespace, name)
}

func (g *opensshEICEServerV2TestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteNode(ctx, defaults.Namespace, name))
}

func (g *opensshEICEServerV2TestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	node := &resourcesv1.TeleportOpenSSHEICEServerV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportOpenSSHEICEServerV2Spec(opensshEICEServerV2Spec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, node))
}

func (g *opensshEICEServerV2TestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	node := &resourcesv1.TeleportOpenSSHEICEServerV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, node))
}

func (g *opensshEICEServerV2TestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportOpenSSHEICEServerV2, error) {
	node := &resourcesv1.TeleportOpenSSHEICEServerV2{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, node)
	return node, trace.Wrap(err)
}

func (g *opensshEICEServerV2TestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	node, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	node.Spec.Addr = "127.0.0.1:23"
	return g.setup.K8sClient.Update(ctx, node)
}

func (g *opensshEICEServerV2TestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.Server, kubeResource *resourcesv1.TeleportOpenSSHEICEServerV2) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions()...)
	return diff == "", diff
}

func TestTeleportOpensshEICEServerV2Creation(t *testing.T) {
	test := &opensshEICEServerV2TestingPrimitives{}
	testlib.ResourceCreationTest[types.Server, *resourcesv1.TeleportOpenSSHEICEServerV2](t, test)
}

func TestTeleportOpensshEICEServerV2DeletionDrift(t *testing.T) {
	test := &opensshEICEServerV2TestingPrimitives{}
	testlib.ResourceDeletionDriftTest[types.Server, *resourcesv1.TeleportOpenSSHEICEServerV2](t, test)
}

func TestTeleportOpensshEICEServerV2Update(t *testing.T) {
	test := &opensshEICEServerV2TestingPrimitives{}
	testlib.ResourceUpdateTest[types.Server, *resourcesv1.TeleportOpenSSHEICEServerV2](t, test)
}
