// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package services

import (
	"context"
	"iter"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

// Summarizer is a service that provides methods to manage summarization
// inference configuration resources in the backend.
type Summarizer interface {
	// CreateSummarizationInferenceModel creates a new summarization inference
	// model in the backend.
	CreateSummarizationInferenceModel(ctx context.Context, model *summarizerv1.SummarizationInferenceModel) (*summarizerv1.SummarizationInferenceModel, error)
	// GetSummarizationInferenceModel retrieves a summarization inference model
	// from the backend by name.
	GetSummarizationInferenceModel(ctx context.Context, name string) (*summarizerv1.SummarizationInferenceModel, error)
	// UpdateSummarizationInferenceModel updates an existing summarization
	// inference model in the backend.
	UpdateSummarizationInferenceModel(ctx context.Context, model *summarizerv1.SummarizationInferenceModel) (*summarizerv1.SummarizationInferenceModel, error)
	// UpsertSummarizationInferenceModel creates or updates a summarization
	// inference model in the backend. If the model already exists, it will be
	// updated.
	UpsertSummarizationInferenceModel(ctx context.Context, model *summarizerv1.SummarizationInferenceModel) (*summarizerv1.SummarizationInferenceModel, error)
	// DeleteSummarizationInferenceModel deletes a summarization inference model
	// from the backend by name.
	DeleteSummarizationInferenceModel(ctx context.Context, name string) error
	// ListSummarizationInferenceModels lists summarization inference models in
	// the backend with pagination support. Returns a slice of models and a next
	// page token.
	ListSummarizationInferenceModels(ctx context.Context, size int, pageToken string) ([]*summarizerv1.SummarizationInferenceModel, string, error)

	// CreateSummarizationInferenceSecret creates a new summarization inference
	// secret in the backend. The returned object contains the secret value and
	// should be handled with care.
	CreateSummarizationInferenceSecret(ctx context.Context, secret *summarizerv1.SummarizationInferenceSecret) (*summarizerv1.SummarizationInferenceSecret, error)
	// GetSummarizationInferenceSecret retrieves a summarization inference secret
	// from the backend by name. The returned object contains the secret value
	// and should be handled with care.
	GetSummarizationInferenceSecret(ctx context.Context, name string) (*summarizerv1.SummarizationInferenceSecret, error)
	// UpdateSummarizationInferenceSecret updates an existing summarization
	// inference secret in the backend. The returned object contains the secret
	// value and should be handled with care.
	UpdateSummarizationInferenceSecret(ctx context.Context, secret *summarizerv1.SummarizationInferenceSecret) (*summarizerv1.SummarizationInferenceSecret, error)
	// UpsertSummarizationInferenceSecret creates or updates a summarization
	// inference secretin the backend. If the secret already exists, it will be
	// updated. The returned object contains the secret value and should be
	// handled with care.
	UpsertSummarizationInferenceSecret(ctx context.Context, secret *summarizerv1.SummarizationInferenceSecret) (*summarizerv1.SummarizationInferenceSecret, error)
	// DeleteSummarizationInferenceSecret deletes a summarization inference
	// secret from the backend by name.
	DeleteSummarizationInferenceSecret(ctx context.Context, name string) error
	// ListSummarizationInferenceSecrets lists summarization inference secrets in
	// the backend with pagination support. Returns a slice of secrets and a next
	// page token. The returned objects contain the secret values and should be
	// handled with care.
	ListSummarizationInferenceSecrets(ctx context.Context, size int, pageToken string) ([]*summarizerv1.SummarizationInferenceSecret, string, error)

	// CreateSummarizationInferencePolicy creates a new summarization inference
	// policy in the backend.
	CreateSummarizationInferencePolicy(ctx context.Context, policy *summarizerv1.SummarizationInferencePolicy) (*summarizerv1.SummarizationInferencePolicy, error)
	// GetSummarizationInferencePolicy retrieves a summarization inference policy
	// from the backend by name.
	GetSummarizationInferencePolicy(ctx context.Context, name string) (*summarizerv1.SummarizationInferencePolicy, error)
	// UpdateSummarizationInferencePolicy updates an existing summarization
	// inference policy in the backend.
	UpdateSummarizationInferencePolicy(ctx context.Context, policy *summarizerv1.SummarizationInferencePolicy) (*summarizerv1.SummarizationInferencePolicy, error)
	// UpsertSummarizationInferencePolicy creates or updates a summarization
	// inference policy in the backend. If the policy already exists, it will be
	// updated.
	UpsertSummarizationInferencePolicy(ctx context.Context, policy *summarizerv1.SummarizationInferencePolicy) (*summarizerv1.SummarizationInferencePolicy, error)
	// DeleteSummarizationInferencePolicy deletes a summarization inference
	// policy from the backend by name.
	DeleteSummarizationInferencePolicy(ctx context.Context, name string) error
	// ListSummarizationInferencePolicies lists summarization inference policies
	// in the backend with pagination support. Returns a slice of policies and a
	// next page token.
	ListSummarizationInferencePolicies(ctx context.Context, size int, pageToken string) ([]*summarizerv1.SummarizationInferencePolicy, string, error)
	// AllSummarizationInferencePolicies retrieves all summarization inference
	// policies from the backend, without pagination.
	AllSummarizationInferencePolicies(ctx context.Context) iter.Seq2[*summarizerv1.SummarizationInferencePolicy, error]
}
