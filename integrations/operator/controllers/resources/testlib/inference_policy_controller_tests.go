// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package testlib

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
)

var inferencePolicySpec = &summarizerv1.InferencePolicySpec{
	Kinds:  []string{"ssh", "k8s"},
	Model:  "test-model",
	Filter: "resource.metadata.labels[\"env\"] == \"production\"",
}

type inferencePolicyTestingPrimitives struct {
	setup *TestSetup
	reconcilers.Resource153Adapter[*summarizerv1.InferencePolicy]
}

func (p *inferencePolicyTestingPrimitives) Init(setup *TestSetup) {
	p.setup = setup
}

func (p *inferencePolicyTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (p *inferencePolicyTestingPrimitives) CreateTeleportResource(
	ctx context.Context, name string,
) error {
	policy := &summarizerv1.InferencePolicy{
		Kind:    types.KindInferencePolicy,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		},
		Spec: inferencePolicySpec,
	}
	_, err := p.setup.TeleportClient.
		SummarizerServiceClient().
		CreateInferencePolicy(ctx, &summarizerv1.CreateInferencePolicyRequest{Policy: policy})
	return trace.Wrap(err)
}

func (p *inferencePolicyTestingPrimitives) GetTeleportResource(
	ctx context.Context, name string,
) (*summarizerv1.InferencePolicy, error) {
	resp, err := p.setup.TeleportClient.
		SummarizerServiceClient().
		GetInferencePolicy(ctx, &summarizerv1.GetInferencePolicyRequest{Name: name})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Policy, nil
}

func (p *inferencePolicyTestingPrimitives) DeleteTeleportResource(
	ctx context.Context, name string,
) error {
	_, err := p.setup.TeleportClient.
		SummarizerServiceClient().
		DeleteInferencePolicy(ctx, &summarizerv1.DeleteInferencePolicyRequest{Name: name})
	return trace.Wrap(err)
}

func (p *inferencePolicyTestingPrimitives) CreateKubernetesResource(
	ctx context.Context, name string,
) error {
	policy := &resourcesv1.TeleportInferencePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportInferencePolicySpec)(inferencePolicySpec),
	}
	return trace.Wrap(p.setup.K8sClient.Create(ctx, policy))
}

func (p *inferencePolicyTestingPrimitives) DeleteKubernetesResource(
	ctx context.Context, name string,
) error {
	policy := &resourcesv1.TeleportInferencePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.setup.Namespace.Name,
		},
	}
	return trace.Wrap(p.setup.K8sClient.Delete(ctx, policy))
}

func (p *inferencePolicyTestingPrimitives) GetKubernetesResource(
	ctx context.Context, name string,
) (*resourcesv1.TeleportInferencePolicy, error) {
	policy := &resourcesv1.TeleportInferencePolicy{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: p.setup.Namespace.Name,
	}
	err := p.setup.K8sClient.Get(ctx, obj, policy)
	return policy, trace.Wrap(err)
}

func (p *inferencePolicyTestingPrimitives) ModifyKubernetesResource(
	ctx context.Context, name string,
) error {
	policy, err := p.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	policy.Spec.Kinds = []string{"ssh", "db"}
	policy.Spec.Filter = "resource.metadata.labels[\"env\"] == \"staging\""
	return trace.Wrap(p.setup.K8sClient.Update(ctx, policy))
}

func (p *inferencePolicyTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *summarizerv1.InferencePolicy,
	kubeResource *resourcesv1.TeleportInferencePolicy,
) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		ProtoCompareOptions()...,
	)
	return diff == "", diff
}

func InferencePolicyCreationTest(t *testing.T, clt *client.Client) {
	test := &inferencePolicyTestingPrimitives{}
	ResourceCreationSynchronousTest(
		t, resources.NewInferencePolicyReconciler, test, WithTeleportClient(clt),
	)
}

func InferencePolicyDeletionTest(t *testing.T, clt *client.Client) {
	test := &inferencePolicyTestingPrimitives{}
	ResourceDeletionSynchronousTest(
		t, resources.NewInferencePolicyReconciler, test, WithTeleportClient(clt),
	)
}

func InferencePolicyDeletionDriftTest(t *testing.T, clt *client.Client) {
	test := &inferencePolicyTestingPrimitives{}
	ResourceDeletionDriftSynchronousTest(
		t, resources.NewInferencePolicyReconciler, test, WithTeleportClient(clt),
	)
}

func InferencePolicyUpdateTest(t *testing.T, clt *client.Client) {
	test := &inferencePolicyTestingPrimitives{}
	ResourceUpdateTestSynchronous(
		t, resources.NewInferencePolicyReconciler, test, WithTeleportClient(clt),
	)
}
