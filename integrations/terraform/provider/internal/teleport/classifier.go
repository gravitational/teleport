// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
