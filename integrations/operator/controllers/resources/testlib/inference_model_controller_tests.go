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

var inferenceModelSpec = summarizerv1.InferenceModelSpec_builder{
	Bedrock: summarizerv1.BedrockProvider_builder{
		Region:         "us-east-1",
		BedrockModelId: "anthropic.claude-3-haiku-20240307-v1:0",
		Integration:    "some-integration",
	}.Build(),
	MaxSessionLengthBytes: 1234567,
}.Build()

type inferenceModelTestingPrimitives struct {
	setup *TestSetup
	reconcilers.Resource153Adapter[*summarizerv1.InferenceModel]
}

func (p *inferenceModelTestingPrimitives) Init(setup *TestSetup) {
	p.setup = setup
}

func (p *inferenceModelTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (p *inferenceModelTestingPrimitives) CreateTeleportResource(
	ctx context.Context, name string,
) error {
	model := summarizerv1.InferenceModel_builder{
		Kind:    types.KindInferenceModel,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		}.Build(),
		Spec: inferenceModelSpec,
	}.Build()
	_, err := p.setup.TeleportClient.
		SummarizerServiceClient().
		CreateInferenceModel(ctx, summarizerv1.CreateInferenceModelRequest_builder{Model: model}.Build())
	return trace.Wrap(err)
}

func (p *inferenceModelTestingPrimitives) GetTeleportResource(
	ctx context.Context, name string,
) (*summarizerv1.InferenceModel, error) {
	resp, err := p.setup.TeleportClient.
		SummarizerServiceClient().
		GetInferenceModel(ctx, summarizerv1.GetInferenceModelRequest_builder{Name: name}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.GetModel(), nil
}

func (p *inferenceModelTestingPrimitives) DeleteTeleportResource(
	ctx context.Context, name string,
) error {
	_, err := p.setup.TeleportClient.
		SummarizerServiceClient().
		DeleteInferenceModel(ctx, summarizerv1.DeleteInferenceModelRequest_builder{Name: name}.Build())
	return trace.Wrap(err)
}

func (p *inferenceModelTestingPrimitives) CreateKubernetesResource(
	ctx context.Context, name string,
) error {
	model := &resourcesv1.TeleportInferenceModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportInferenceModelSpec)(inferenceModelSpec),
	}
	return trace.Wrap(p.setup.K8sClient.Create(ctx, model))
}

func (p *inferenceModelTestingPrimitives) DeleteKubernetesResource(
	ctx context.Context, name string,
) error {
	model := &resourcesv1.TeleportInferenceModel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.setup.Namespace.Name,
		},
	}
	return trace.Wrap(p.setup.K8sClient.Delete(ctx, model))
}

func (p *inferenceModelTestingPrimitives) GetKubernetesResource(
	ctx context.Context, name string,
) (*resourcesv1.TeleportInferenceModel, error) {
	model := &resourcesv1.TeleportInferenceModel{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: p.setup.Namespace.Name,
	}
	err := p.setup.K8sClient.Get(ctx, obj, model)
	return model, trace.Wrap(err)
}

func (p *inferenceModelTestingPrimitives) ModifyKubernetesResource(
	ctx context.Context, name string,
) error {
	model, err := p.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	model.Spec.MaxSessionLengthBytes = 7654321
	(*summarizerv1.InferenceModelSpec)(model.Spec).GetBedrock().SetBedrockModelId(
		"anthropic.claude-3-5-sonnet-20240620-v1:0")
	return trace.Wrap(p.setup.K8sClient.Update(ctx, model))
}

func (p *inferenceModelTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *summarizerv1.InferenceModel,
	kubeResource *resourcesv1.TeleportInferenceModel,
) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		ProtoCompareOptions()...,
	)
	return diff == "", diff
}

func InferenceModelCreationTest(t *testing.T, clt *client.Client) {
	test := &inferenceModelTestingPrimitives{}
	ResourceCreationSynchronousTest(
		t, resources.NewInferenceModelReconciler, test, WithTeleportClient(clt),
	)
}

func InferenceModelDeletionTest(t *testing.T, clt *client.Client) {
	test := &inferenceModelTestingPrimitives{}
	ResourceDeletionSynchronousTest(
		t, resources.NewInferenceModelReconciler, test, WithTeleportClient(clt),
	)
}

func InferenceModelDeletionDriftTest(t *testing.T, clt *client.Client) {
	test := &inferenceModelTestingPrimitives{}
	ResourceDeletionDriftSynchronousTest(
		t, resources.NewInferenceModelReconciler, test, WithTeleportClient(clt),
	)
}

func InferenceModelUpdateTest(t *testing.T, clt *client.Client) {
	test := &inferenceModelTestingPrimitives{}
	ResourceUpdateTestSynchronous(
		t, resources.NewInferenceModelReconciler, test, WithTeleportClient(clt),
	)
}
