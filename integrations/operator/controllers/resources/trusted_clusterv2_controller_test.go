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

var trustedClusterV2Spec = types.TrustedClusterSpecV2{
	Enabled:              true,
	Token:                "secret_token",
	ProxyAddress:         "root.example.com:443",
	ReverseTunnelAddress: "root.example.com:443",
	RoleMap: []types.RoleMapping{
		{
			Remote: "remote-role",
			Local:  []string{"local-role"},
		},
	},
}

type trustedClusterV2TestingPrimitives struct {
	setup *testSetup
	reconcilers.ResourceWithoutLabelsAdapter[types.TrustedCluster]
}

func (r *trustedClusterV2TestingPrimitives) Init(setup *testSetup) {
	r.setup = setup
}

func (r *trustedClusterV2TestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (r *trustedClusterV2TestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	trustedCluster, err := types.NewTrustedCluster(name, trustedClusterV2Spec)
	if err != nil {
		return trace.Wrap(err)
	}
	trustedCluster.SetOrigin(types.OriginKubernetes)
	_, err = r.setup.TeleportClient.UpsertTrustedCluster(ctx, trustedCluster)
	return trace.Wrap(err)
}

func (r *trustedClusterV2TestingPrimitives) GetTeleportResource(ctx context.Context, name string) (types.TrustedCluster, error) {
	return r.setup.TeleportClient.GetTrustedCluster(ctx, name)
}

func (r *trustedClusterV2TestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(r.setup.TeleportClient.DeleteTrustedCluster(ctx, name))
}

func (r *trustedClusterV2TestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	trustedCluster := &resourcesv1.TeleportTrustedClusterV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.setup.Namespace.Name,
		},
		Spec: resourcesv1.TeleportTrustedClusterV2Spec(trustedClusterV2Spec),
	}
	return trace.Wrap(r.setup.K8sClient.Create(ctx, trustedCluster))
}

func (r *trustedClusterV2TestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	trustedCluster := &resourcesv1.TeleportTrustedClusterV2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.setup.Namespace.Name,
		},
	}
	return trace.Wrap(r.setup.K8sClient.Delete(ctx, trustedCluster))
}

func (r *trustedClusterV2TestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportTrustedClusterV2, error) {
	trustedCluster := &resourcesv1.TeleportTrustedClusterV2{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: r.setup.Namespace.Name,
	}
	err := r.setup.K8sClient.Get(ctx, obj, trustedCluster)
	return trustedCluster, trace.Wrap(err)
}

func (r *trustedClusterV2TestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	trustedCluster, err := r.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	trustedCluster.Spec.RoleMap[0] = types.RoleMapping{
		Remote: "remote-role-2",
		Local:  []string{"local-role-2"},
	}
	return trace.Wrap(r.setup.K8sClient.Create(ctx, trustedCluster))
}

func (r *trustedClusterV2TestingPrimitives) CompareTeleportAndKubernetesResource(tResource types.TrustedCluster, kubeResource *resourcesv1.TeleportTrustedClusterV2) (bool, string) {
	diff := cmp.Diff(tResource, kubeResource.ToTeleport(), testlib.CompareOptions()...)
	return diff == "", diff
}

func TestTrustedClusterV2Creation(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	testlib.ResourceCreationTest(t, test)
}

func TestTrustedClusterV2DeletionDrift(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	testlib.ResourceDeletionDriftTest(t, test)
}

func TestTrustedClusterV2Update(t *testing.T) {
	test := &trustedClusterV2TestingPrimitives{}
	testlib.ResourceUpdateTest(t, test)
}
