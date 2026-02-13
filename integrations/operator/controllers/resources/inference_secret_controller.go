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

package resources

import (
	"context"

	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/secretlookup"
)

// inferenceSecretClient implements [reconcilers.resourceClient] and offers
// CRUD methods needed to reconcile InferenceSecret.
type inferenceSecretClient struct {
	teleportClient *client.Client
	kubeClient     kclient.Client
}

// Get gets an inference secret with a given name from Teleport.
func (c inferenceSecretClient) Get(ctx context.Context, name string) (*summarizerv1.InferenceSecret, error) {
	resp, err := c.teleportClient.SummarizerServiceClient().GetInferenceSecret(
		ctx, &summarizerv1.GetInferenceSecretRequest{Name: name},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.Secret, nil
}

// Create creates an inference secret in Teleport.
func (c inferenceSecretClient) Create(ctx context.Context, secret *summarizerv1.InferenceSecret) error {
	_, err := c.teleportClient.SummarizerServiceClient().CreateInferenceSecret(
		ctx, &summarizerv1.CreateInferenceSecretRequest{Secret: secret},
	)
	return trace.Wrap(err)
}

// Update updates an existing inference secret in Teleport.
func (c inferenceSecretClient) Update(ctx context.Context, secret *summarizerv1.InferenceSecret) error {
	_, err := c.teleportClient.SummarizerServiceClient().UpdateInferenceSecret(
		ctx, &summarizerv1.UpdateInferenceSecretRequest{Secret: secret},
	)
	return trace.Wrap(err)
}

// Delete deletes an inference secret with a given name from Teleport.
func (c inferenceSecretClient) Delete(ctx context.Context, name string) error {
	_, err := c.teleportClient.SummarizerServiceClient().DeleteInferenceSecret(
		ctx, &summarizerv1.DeleteInferenceSecretRequest{Name: name},
	)
	return trace.Wrap(err)
}

// Mutate resolves secret:// references in the InferenceSecret value field
func (c inferenceSecretClient) Mutate(ctx context.Context, new, _ *summarizerv1.InferenceSecret, crKey kclient.ObjectKey) error {
	secretValue := new.GetSpec().GetValue()
	if secretlookup.IsNeeded(secretValue) {
		resolvedSecret, err := secretlookup.Try(ctx, c.kubeClient, crKey.Name, crKey.Namespace, secretValue)
		if err != nil {
			return trace.Wrap(err)
		}
		new.Spec.Value = resolvedSecret
	}
	return nil
}

// NewInferenceSecretReconciler creates a new Kubernetes controller reconciling
// inference_secret resources.
func NewInferenceSecretReconciler(
	client kclient.Client, tClient *client.Client,
) (controllers.Reconciler, error) {
	secretClient := &inferenceSecretClient{
		teleportClient: tClient,
		kubeClient:     client,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*summarizerv1.InferenceSecret, *resourcesv1.TeleportInferenceSecret,
	](
		client,
		secretClient,
	)

	return resourceReconciler, trace.Wrap(err)
}
