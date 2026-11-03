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

type classifierClient struct {
	teleportClient *client.Client
}

func (c classifierClient) Get(
	ctx context.Context, key reconcilers.ResourceKey,
) (*summarizerv1.Classifier, error) {
	resp, err := c.teleportClient.SummarizerServiceClient().GetClassifier(
		ctx, summarizerv1.GetClassifierRequest_builder{Name: key.Name}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.GetClassifier(), nil
}

func (c classifierClient) Create(ctx context.Context, classifier *summarizerv1.Classifier) error {
	_, err := c.teleportClient.SummarizerServiceClient().CreateClassifier(
		ctx, summarizerv1.CreateClassifierRequest_builder{Classifier: classifier}.Build(),
	)
	return trace.Wrap(err)
}

func (c classifierClient) Update(ctx context.Context, classifier *summarizerv1.Classifier) error {
	_, err := c.teleportClient.SummarizerServiceClient().UpdateClassifier(
		ctx, summarizerv1.UpdateClassifierRequest_builder{Classifier: classifier}.Build(),
	)
	return trace.Wrap(err)
}

func (c classifierClient) Delete(ctx context.Context, key reconcilers.ResourceKey) error {
	_, err := c.teleportClient.SummarizerServiceClient().DeleteClassifier(
		ctx, summarizerv1.DeleteClassifierRequest_builder{Name: key.Name}.Build(),
	)
	return trace.Wrap(err)
}

// NewClassifierV1Reconciler creates a Kubernetes controller reconciling classifier resources.
func NewClassifierV1Reconciler(
	client kclient.Client, tClient *client.Client, _ reconcilers.OperatorMetadata,
) (controllers.Reconciler, error) {
	resourceReconciler, err := reconcilers.NewTeleportResource153Reconciler[
		*summarizerv1.Classifier, *resourcesv1.TeleportClassifierV1,
	](
		client,
		&classifierClient{teleportClient: tClient},
		reconcilers.Config{
			CheckFeatures: controllers.RequireSessionSummaries,
		},
	)

	return resourceReconciler, trace.Wrap(err)
}
