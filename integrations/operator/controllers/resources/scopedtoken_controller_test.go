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
	"google.golang.org/protobuf/testing/protocmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	tokenv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

var scopedTokenSpec = &tokenv1.ScopedTokenSpec{
	AssignedScope: "/staging/foo",
	Roles:         []string{types.RoleNode.String()},
	JoinMethod:    string(types.JoinMethodToken),
	UsageMode:     "unlimited",
	ImmutableLabels: &tokenv1.ImmutableLabels{
		Ssh: map[string]string{
			"env": "prod",
		},
	},
}

type scopedTokenTestingPrimitives struct {
	setup *testSetup
	reconcilers.Resource153Adapter[*tokenv1.ScopedToken]
}

func (g *scopedTokenTestingPrimitives) Init(setup *testSetup) {
	g.setup = setup
}

func (g *scopedTokenTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (g *scopedTokenTestingPrimitives) CreateTeleportResource(ctx context.Context, name string) error {
	token := &tokenv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		},
		Scope: "/staging",
		Spec:  scopedTokenSpec,
	}
	_, err := g.setup.TeleportClient.CreateScopedToken(ctx, token)
	return trace.Wrap(err)
}

func (g *scopedTokenTestingPrimitives) GetTeleportResource(ctx context.Context, name string) (*tokenv1.ScopedToken, error) {
	return g.setup.TeleportClient.GetScopedToken(ctx, name, true)
}

func (g *scopedTokenTestingPrimitives) DeleteTeleportResource(ctx context.Context, name string) error {
	return trace.Wrap(g.setup.TeleportClient.DeleteScopedToken(ctx, name))
}

func (g *scopedTokenTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	token := &resourcesv1.TeleportScopedTokenV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
		Scope: "/staging",
		Spec:  (*resourcesv1.TeleportScopedTokenV1Spec)(scopedTokenSpec),
	}
	return trace.Wrap(g.setup.K8sClient.Create(ctx, token))
}

func (g *scopedTokenTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	token := &resourcesv1.TeleportScopedTokenV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.setup.Namespace.Name,
		},
	}
	return trace.Wrap(g.setup.K8sClient.Delete(ctx, token))
}

func (g *scopedTokenTestingPrimitives) GetKubernetesResource(ctx context.Context, name string) (*resourcesv1.TeleportScopedTokenV1, error) {
	token := &resourcesv1.TeleportScopedTokenV1{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: g.setup.Namespace.Name,
	}
	err := g.setup.K8sClient.Get(ctx, obj, token)
	return token, trace.Wrap(err)
}

func (g *scopedTokenTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	token, err := g.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	token.Spec.ImmutableLabels.Ssh = map[string]string{
		"env": "staging",
	}
	return trace.Wrap(g.setup.K8sClient.Update(ctx, token))
}

func (g *scopedTokenTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *tokenv1.ScopedToken, kubeResource *resourcesv1.TeleportScopedTokenV1) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		testlib.ProtoCompareOptions(
			protocmp.IgnoreFields(&tokenv1.ScopedToken{}, "status"),
		)...,
	)
	return diff == "", diff
}

func TestScopedTokenCreation(t *testing.T) {
	test := &scopedTokenTestingPrimitives{}
	testlib.ResourceCreationSynchronousTest(t, resources.NewScopedTokenV1Reconciler, test)
}

func TestScopedTokenDeletionDrift(t *testing.T) {
	test := &scopedTokenTestingPrimitives{}
	testlib.ResourceDeletionDriftSynchronousTest(t, resources.NewScopedTokenV1Reconciler, test)
}

func TestScopedTokenUpdate(t *testing.T) {
	test := &scopedTokenTestingPrimitives{}
	testlib.ResourceUpdateTestSynchronous(t, resources.NewScopedTokenV1Reconciler, test)
}
