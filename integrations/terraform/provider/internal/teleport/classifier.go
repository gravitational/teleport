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

package teleport

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewClassifierClient returns a classifier client.
func NewClassifierClient(c *client.Client) ClassifierClient {
	return ClassifierClient{client: c}
}

// ClassifierClient manages classifier resources.
type ClassifierClient struct {
	client *client.Client
}

// Get reads a classifier by name.
func (r ClassifierClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*summarizerv1.Classifier, error) {
	classifier, err := r.client.SummarizerClient().GetClassifier(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return classifier, nil
}

// Create creates a classifier.
func (r ClassifierClient) Create(ctx context.Context, classifier *summarizerv1.Classifier) error {
	if _, err := r.client.SummarizerClient().CreateClassifier(ctx, classifier); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a classifier.
func (r ClassifierClient) Upsert(ctx context.Context, classifier *summarizerv1.Classifier) error {
	if _, err := r.client.SummarizerClient().UpsertClassifier(ctx, classifier); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a classifier by name.
func (r ClassifierClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.SummarizerClient().DeleteClassifier(ctx, id.Name))
}
