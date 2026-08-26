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

var inferenceSecretSpec = &summarizerv1.InferenceSecretSpec{
	Value: "my-secret-value-123",
}

type inferenceSecretTestingPrimitives struct {
	setup *TestSetup
	reconcilers.Resource153Adapter[*summarizerv1.InferenceSecret]
}

func (p *inferenceSecretTestingPrimitives) Init(setup *TestSetup) {
	p.setup = setup
}

func (p *inferenceSecretTestingPrimitives) SetupTeleportFixtures(ctx context.Context) error {
	return nil
}

func (p *inferenceSecretTestingPrimitives) CreateTeleportResource(
	ctx context.Context, name string,
) error {
	secret := &summarizerv1.InferenceSecret{
		Kind:    types.KindInferenceSecret,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel: types.OriginKubernetes,
			},
		},
		Spec: inferenceSecretSpec,
	}
	_, err := p.setup.TeleportClient.
		SummarizerServiceClient().
		CreateInferenceSecret(ctx, &summarizerv1.CreateInferenceSecretRequest{Secret: secret})
	return trace.Wrap(err)
}

func (p *inferenceSecretTestingPrimitives) GetTeleportResource(
	ctx context.Context, name string,
) (*summarizerv1.InferenceSecret, error) {
	resp, err := p.setup.TeleportClient.
		SummarizerServiceClient().
		GetInferenceSecret(ctx, &summarizerv1.GetInferenceSecretRequest{Name: name})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Secret, nil
}

func (p *inferenceSecretTestingPrimitives) DeleteTeleportResource(
	ctx context.Context, name string,
) error {
	_, err := p.setup.TeleportClient.
		SummarizerServiceClient().
		DeleteInferenceSecret(ctx, &summarizerv1.DeleteInferenceSecretRequest{Name: name})
	return trace.Wrap(err)
}

func (p *inferenceSecretTestingPrimitives) CreateKubernetesResource(
	ctx context.Context, name string,
) error {
	secret := &resourcesv1.TeleportInferenceSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.setup.Namespace.Name,
		},
		Spec: (*resourcesv1.TeleportInferenceSecretSpec)(inferenceSecretSpec),
	}
	return trace.Wrap(p.setup.K8sClient.Create(ctx, secret))
}

func (p *inferenceSecretTestingPrimitives) DeleteKubernetesResource(
	ctx context.Context, name string,
) error {
	secret := &resourcesv1.TeleportInferenceSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.setup.Namespace.Name,
		},
	}
	return trace.Wrap(p.setup.K8sClient.Delete(ctx, secret))
}

func (p *inferenceSecretTestingPrimitives) GetKubernetesResource(
	ctx context.Context, name string,
) (*resourcesv1.TeleportInferenceSecret, error) {
	secret := &resourcesv1.TeleportInferenceSecret{}
	obj := kclient.ObjectKey{
		Name:      name,
		Namespace: p.setup.Namespace.Name,
	}
	err := p.setup.K8sClient.Get(ctx, obj, secret)
	return secret, trace.Wrap(err)
}

func (p *inferenceSecretTestingPrimitives) ModifyKubernetesResource(
	ctx context.Context, name string,
) error {
	secret, err := p.GetKubernetesResource(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}
	secret.Spec.Value = "my-modified-secret-value-456"
	return trace.Wrap(p.setup.K8sClient.Update(ctx, secret))
}

func (p *inferenceSecretTestingPrimitives) CompareTeleportAndKubernetesResource(
	tResource *summarizerv1.InferenceSecret,
	kubeResource *resourcesv1.TeleportInferenceSecret,
) (bool, string) {
	// InferenceSecret spec is write-only, so Teleport will return spec=nil. We
	// therefore only compare the metadata.
	diff := cmp.Diff(
		tResource.Metadata,
		kubeResource.ToTeleport().Metadata,
		ProtoCompareOptions()...,
	)
	return diff == "", diff
}

func InferenceSecretCreationTest(t *testing.T, clt *client.Client) {
	test := &inferenceSecretTestingPrimitives{}
	ResourceCreationSynchronousTest(
		t, resources.NewInferenceSecretReconciler, test, WithTeleportClient(clt),
	)
}

func InferenceSecretDeletionTest(t *testing.T, clt *client.Client) {
	test := &inferenceSecretTestingPrimitives{}
	ResourceDeletionSynchronousTest(
		t, resources.NewInferenceSecretReconciler, test, WithTeleportClient(clt),
	)
}

func InferenceSecretDeletionDriftTest(t *testing.T, clt *client.Client) {
	test := &inferenceSecretTestingPrimitives{}
	ResourceDeletionDriftSynchronousTest(
		t, resources.NewInferenceSecretReconciler, test, WithTeleportClient(clt),
	)
}

func InferenceSecretUpdateTest(t *testing.T, clt *client.Client) {
	test := &inferenceSecretTestingPrimitives{}
	ResourceUpdateTestSynchronous(
		t, resources.NewInferenceSecretReconciler, test, WithTeleportClient(clt),
	)
}
