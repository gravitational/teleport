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

// inferencePolicyClient implements [reconcilers.resourceClient] and offers CRUD
// methods needed to reconcile InferencePolicy
type inferencePolicyClient struct {
	teleportClient *client.Client
}

// Get gets an inference policy with a given name from Teleport.
func (c inferencePolicyClient) Get(
	ctx context.Context, name string,
) (*summarizerv1.InferencePolicy, error) {
	resp, err := c.teleportClient.SummarizerServiceClient().GetInferencePolicy(
		ctx, &summarizerv1.GetInferencePolicyRequest{Name: name},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.Policy, nil
}

// Create creates an inference policy in Teleport.
func (c inferencePolicyClient) Create(
	ctx context.Context, policy *summarizerv1.InferencePolicy,
) error {
	_, err := c.teleportClient.SummarizerServiceClient().CreateInferencePolicy(
		ctx, &summarizerv1.CreateInferencePolicyRequest{Policy: policy},
	)
	return trace.Wrap(err)
}

// Update updates an existing inference policy in Teleport.
func (c inferencePolicyClient) Update(
	ctx context.Context, policy *summarizerv1.InferencePolicy,
) error {
	_, err := c.teleportClient.SummarizerServiceClient().UpdateInferencePolicy(
		ctx, &summarizerv1.UpdateInferencePolicyRequest{Policy: policy},
	)
	return trace.Wrap(err)
}

// Delete deletes an inference policy with a given name from Teleport.
func (c inferencePolicyClient) Delete(ctx context.Context, name string) error {
	_, err := c.teleportClient.SummarizerServiceClient().DeleteInferencePolicy(
		ctx, &summarizerv1.DeleteInferencePolicyRequest{Name: name},
	)
	return trace.Wrap(err)
}

// NewInferencePolicyReconciler creates a new Kubernetes controller reconciling
// inference_policy resources.
func NewInferencePolicyReconciler(
	client kclient.Client, tClient *client.Client,
) (controllers.Reconciler, error) {
	inferencePolicyClient := &inferencePolicyClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*summarizerv1.InferencePolicy, *resourcesv1.TeleportInferencePolicy,
	](
		client,
		inferencePolicyClient,
	)

	return resourceReconciler, trace.Wrap(err)
}
