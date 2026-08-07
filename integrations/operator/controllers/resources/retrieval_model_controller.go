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

package resources

import (
	"context"

	"github.com/gravitational/trace"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/client"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	resourcesv1 "github.com/gravitational/teleport/integrations/operator/apis/resources/v1"
	"github.com/gravitational/teleport/integrations/operator/controllers"
	"github.com/gravitational/teleport/integrations/operator/controllers/reconcilers"
)

// retrievalModelClient implements TeleportResourceClient and offers CRUD methods needed to reconcile RetrievalModel.
type retrievalModelClient struct {
	teleportClient *client.Client
}

// Get gets the Teleport RetrievalModel singleton. The name parameter is ignored.
func (r retrievalModelClient) Get(ctx context.Context, _ string) (*summarizerv1.RetrievalModel, error) {
	resp, err := r.teleportClient.SummarizerServiceClient().
		GetRetrievalModel(ctx, &summarizerv1.GetRetrievalModelRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Model, nil
}

// Create creates the Teleport RetrievalModel singleton.
func (r retrievalModelClient) Create(ctx context.Context, resource *summarizerv1.RetrievalModel) error {
	_, err := r.teleportClient.SummarizerServiceClient().
		CreateRetrievalModel(ctx, &summarizerv1.CreateRetrievalModelRequest{Model: resource})
	return trace.Wrap(err)
}

// Update upserts the Teleport RetrievalModel singleton.
func (r retrievalModelClient) Update(ctx context.Context, resource *summarizerv1.RetrievalModel) error {
	_, err := r.teleportClient.SummarizerServiceClient().
		UpsertRetrievalModel(ctx, &summarizerv1.UpsertRetrievalModelRequest{Model: resource})
	return trace.Wrap(err)
}

// Delete deletes the Teleport RetrievalModel singleton. The name parameter is ignored.
func (r retrievalModelClient) Delete(ctx context.Context, _ string) error {
	_, err := r.teleportClient.SummarizerServiceClient().
		DeleteRetrievalModel(ctx, &summarizerv1.DeleteRetrievalModelRequest{})
	return trace.Wrap(err)
}

// Mutate validates that the Kubernetes CR is named with the singleton name before
// creating or updating. RetrievalModel can only exist under types.MetaNameRetrievalModel
// in Teleport; a differently-named CR would silently map to the same singleton, so we
// reject it early with a clear status condition instead.
func (r retrievalModelClient) Mutate(_ context.Context, _, _ *summarizerv1.RetrievalModel, crKey kclient.ObjectKey) error {
	if crKey.Name != types.MetaNameRetrievalModel {
		return trace.BadParameter(
			"TeleportRetrievalModelV1 must be named %q, got %q; delete and recreate the resource with the correct name",
			types.MetaNameRetrievalModel, crKey.Name,
		)
	}
	return nil
}

// NewRetrievalModelV1Reconciler instantiates a new Kubernetes controller reconciling RetrievalModel resources.
func NewRetrievalModelV1Reconciler(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error) {
	rmClient := &retrievalModelClient{
		teleportClient: tClient,
	}

	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*summarizerv1.RetrievalModel, *resourcesv1.TeleportRetrievalModelV1,
	](
		client,
		rmClient,
	)

	return resourceReconciler, trace.Wrap(err, "building teleport resource reconciler")
}
