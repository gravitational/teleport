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
)

// inferenceModelClient implements [reconcilers.resourceClient] and offers CRUD
// methods needed to reconcile InferenceModel
type inferenceModelClient struct {
	teleportClient *client.Client
}

// Get gets an inference model with a given name from Teleport.
func (c inferenceModelClient) Get(
	ctx context.Context, name string,
) (*summarizerv1.InferenceModel, error) {
	resp, err := c.teleportClient.SummarizerServiceClient().GetInferenceModel(
		ctx, &summarizerv1.GetInferenceModelRequest{Name: name},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.Model, nil
}

// Create creates an inference model in Teleport.
func (c inferenceModelClient) Create(
	ctx context.Context, model *summarizerv1.InferenceModel,
) error {
	_, err := c.teleportClient.SummarizerServiceClient().CreateInferenceModel(
		ctx, &summarizerv1.CreateInferenceModelRequest{Model: model},
	)
	return trace.Wrap(err)
}

// Update updates an existing inference model in Teleport.
func (c inferenceModelClient) Update(
	ctx context.Context, model *summarizerv1.InferenceModel,
) error {
	_, err := c.teleportClient.SummarizerServiceClient().UpdateInferenceModel(
		ctx, &summarizerv1.UpdateInferenceModelRequest{Model: model},
	)
	return trace.Wrap(err)
}

// Delete deletes an inference model with a given name from Teleport.
func (c inferenceModelClient) Delete(ctx context.Context, name string) error {
	_, err := c.teleportClient.SummarizerServiceClient().DeleteInferenceModel(
		ctx, &summarizerv1.DeleteInferenceModelRequest{Name: name},
	)
	return trace.Wrap(err)
}

// NewInferenceModelReconciler creates a new Kubernetes controller reconciling
// inference_model resources.
func NewInferenceModelReconciler(
	client kclient.Client, tClient *client.Client,
) (controllers.Reconciler, error) {
	inferenceModelClient := &inferenceModelClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*summarizerv1.InferenceModel, *resourcesv1.TeleportInferenceModel,
	](
		client,
		inferenceModelClient,
	)

	return resourceReconciler, trace.Wrap(err)
}
