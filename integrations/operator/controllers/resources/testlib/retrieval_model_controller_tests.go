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

package testlib

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gravitational/teleport/api/client"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources"
)

var retrievalModelSpec = &summarizerv1.RetrievalModelSpec{
	EmbeddingsProvider: &summarizerv1.RetrievalModelSpec_Bedrock{
		Bedrock: &summarizerv1.BedrockProvider{
			Region:         "us-west-2",
			BedrockModelId: "amazon.titan-embed-text-v2:0",
		},
	},
	InferenceModelName: "test-inference-model",
}

type retrievalModelTestingPrimitives struct {
	setup *TestSetup
	reconcilers.Resource153Adapter[*summarizerv1.RetrievalModel]
}

func (p *retrievalModelTestingPrimitives) Init(setup *TestSetup) {
	p.setup = setup
}

func (p *retrievalModelTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	model := &summarizerv1.InferenceModel{
		Kind:    types.KindInferenceModel,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: retrievalModelSpec.InferenceModelName,
		},
		Spec: inferenceModelSpec,
	}
	_, err := p.setup.TeleportClient.SummarizerClient().UpsertInferenceModel(ctx, model)
	return trace.Wrap(err)
}

func (p *retrievalModelTestingPrimitives) CreateTeleportResource(ctx context.Context, _ string) error {
	model := &summarizerv1.RetrievalModel{
		Kind:    types.KindRetrievalModel,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameRetrievalModel,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		},
		Spec: retrievalModelSpec,
	}
	_, err := p.setup.TeleportClient.SummarizerServiceClient().
		CreateRetrievalModel(ctx, &summarizerv1.CreateRetrievalModelRequest{Model: model})
	return trace.Wrap(err)
}

func (p *retrievalModelTestingPrimitives) GetTeleportResource(
	ctx context.Context, _ string,
) (*summarizerv1.RetrievalModel, error) {
	resp, err := p.setup.TeleportClient.SummarizerServiceClient().
		GetRetrievalModel(ctx, &summarizerv1.GetRetrievalModelRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Model, nil
}

func (p *retrievalModelTestingPrimitives) DeleteTeleportResource(ctx context.Context, _ string) error {
	_, err := p.setup.TeleportClient.SummarizerServiceClient().
		DeleteRetrievalModel(ctx, &summarizerv1.DeleteRetrievalModelRequest{})
	return trace.Wrap(err)
}

func (p *retrievalModelTestingPrimitives) CreateKubernetesResource(ctx context.Context, name string) error {
	model := &resourcesv1.TeleportRetrievalModelV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportRetrievalModelV1Spec)(retrievalModelSpec),
	}
	return trace.Wrap(p.setup.K8sClient.Create(ctx, model))
}

func (p *retrievalModelTestingPrimitives) DeleteKubernetesResource(ctx context.Context, name string) error {
	model := &resourcesv1.TeleportRetrievalModelV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.setup.Namespace.Name,
		},
	}
	return trace.Wrap(p.setup.K8sClient.Delete(ctx, model))
}

func (p *retrievalModelTestingPrimitives) GetKubernetesResource(
	ctx context.Context, name string,
) (*resourcesv1.TeleportRetrievalModelV1, error) {
	model := &resourcesv1.TeleportRetrievalModelV1{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: p.setup.Namespace.Name,
	}
	err := p.setup.K8sClient.Get(ctx, obj, model)
	return model, trace.Wrap(err)
}

func (p *retrievalModelTestingPrimitives) ModifyKubernetesResource(ctx context.Context, name string) error {
	model, err := p.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	model.Spec.EmbeddingsProvider = &summarizerv1.RetrievalModelSpec_Bedrock{
		Bedrock: &summarizerv1.BedrockProvider{
			Region:         "us-west-1",
			BedrockModelId: "amazon.titan-embed-text-v2:0",
		},
	}
	return trace.Wrap(p.setup.K8sClient.Update(ctx, model))
}

func (p *retrievalModelTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *summarizerv1.RetrievalModel,
	kubeResource *resourcesv1.TeleportRetrievalModelV1,
) (bool, string) {
	diff := cmp.Diff(
		tResource,
		kubeResource.ToTeleport(),
		ProtoCompareOptions(
			// The singleton name is forced to types.MetaNameRetrievalModel in ToTeleport(),
			// so it will never match the k8s CR name used in tests.
			protocmp.IgnoreFields(&headerv1.Metadata{}, "name"),
		)...,
	)
	return diff == "", diff
}

func RetrievalModelCreationTest(t *testing.T, clt *client.Client) {
	test := &retrievalModelTestingPrimitives{}
	ResourceCreationSynchronousTest(
		t, resources.NewRetrievalModelV1Reconciler, test,
		WithTeleportClient(clt),
		WithResourceName(types.MetaNameRetrievalModel),
	)
}

func RetrievalModelDeletionTest(t *testing.T, clt *client.Client) {
	test := &retrievalModelTestingPrimitives{}
	ResourceDeletionSynchronousTest(
		t, resources.NewRetrievalModelV1Reconciler, test,
		WithTeleportClient(clt),
		WithResourceName(types.MetaNameRetrievalModel),
	)
}

func RetrievalModelDeletionDriftTest(t *testing.T, clt *client.Client) {
	test := &retrievalModelTestingPrimitives{}
	ResourceDeletionDriftSynchronousTest(
		t, resources.NewRetrievalModelV1Reconciler, test,
		WithTeleportClient(clt),
		WithResourceName(types.MetaNameRetrievalModel),
	)
}

func RetrievalModelUpdateTest(t *testing.T, clt *client.Client) {
	test := &retrievalModelTestingPrimitives{}
	ResourceUpdateTestSynchronous(
		t, resources.NewRetrievalModelV1Reconciler, test,
		WithTeleportClient(clt),
		WithResourceName(types.MetaNameRetrievalModel),
	)
}

func RetrievalModelWrongNameTest(t *testing.T, clt *client.Client) {
	ctx := t.Context()
	setup := SetupFakeKubeTestEnv(t, WithTeleportClient(clt))

	reconciler, err := resources.NewRetrievalModelV1Reconciler(setup.K8sClient, setup.TeleportClient)
	require.NoError(t, err)

	const wrongName = "not-retrieval-model"
	cr := &resourcesv1.TeleportRetrievalModelV1{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wrongName,
			Namespace: setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportRetrievalModelV1Spec)(retrievalModelSpec),
	}
	require.NoError(t, setup.K8sClient.Create(ctx, cr))

	req := reconcile.Request{
		NamespacedName: apimachinerytypes.NamespacedName{
			Namespace: setup.Namespace.Name,
			Name:      wrongName,
		},
	}
	// First reconciliation adds the finalizer.
	_, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
	// Second reconciliation calls Mutate and fails.
	_, err = reconciler.Reconcile(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be named")

	// Wait for the MutationError status condition to be written back to the CR.
	FastEventuallyWithT(t, func(t *assert.CollectT) {
		current := &resourcesv1.TeleportRetrievalModelV1{}
		require.NoError(t, setup.K8sClient.Get(ctx, kclient.ObjectKey{Name: wrongName, Namespace: setup.Namespace.Name}, current))
		condition := apimeta.FindStatusCondition(current.Status.Conditions, reconcilers.ConditionTypeSuccessfullyReconciled)
		require.NotNil(t, condition)
		require.Equal(t, metav1.ConditionFalse, condition.Status)
		require.Equal(t, reconcilers.ConditionReasonMutationError, condition.Reason)
	})

	// Cleanup: remove the CR and drain the finalizer.
	require.NoError(t, setup.K8sClient.Delete(ctx, cr))
	_, err = reconciler.Reconcile(ctx, req)
	require.NoError(t, err)
}
