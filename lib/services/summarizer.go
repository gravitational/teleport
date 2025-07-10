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

// SummarizerResources is a service that provides methods to manage summarization
// inference configuration resources in the backend.
type SummarizerResources interface {
	// CreateInferenceModel creates a new summarization inference model in the
	// backend.
	CreateInferenceModel(ctx context.Context, model *summarizerv1.InferenceModel) (*summarizerv1.InferenceModel, error)
	// GetInferenceModel retrieves a summarization inference model from the
	// backend by name.
	GetInferenceModel(ctx context.Context, name string) (*summarizerv1.InferenceModel, error)
	// UpdateInferenceModel updates an existing summarization inference model in
	// the backend.
	UpdateInferenceModel(ctx context.Context, model *summarizerv1.InferenceModel) (*summarizerv1.InferenceModel, error)
	// UpsertInferenceModel creates or updates a summarization inference model in
	// the backend. If the model already exists, it will be updated.
	UpsertInferenceModel(ctx context.Context, model *summarizerv1.InferenceModel) (*summarizerv1.InferenceModel, error)
	// DeleteInferenceModel deletes a summarization inference model from the
	// backend by name.
	DeleteInferenceModel(ctx context.Context, name string) error
	// ListInferenceModels lists summarization inference models in the backend
	// with pagination support. Returns a slice of models and a next page token.
	ListInferenceModels(ctx context.Context, size int, pageToken string) ([]*summarizerv1.InferenceModel, string, error)

	// CreateInferenceSecret creates a new summarization inference secret in the
	// backend. The returned object contains the secret value and should be
	// handled with care.
	CreateInferenceSecret(ctx context.Context, secret *summarizerv1.InferenceSecret) (*summarizerv1.InferenceSecret, error)
	// GetInferenceSecret retrieves a summarization inference secret from the
	// backend by name. The returned object contains the secret value and should
	// be handled with care.
	GetInferenceSecret(ctx context.Context, name string) (*summarizerv1.InferenceSecret, error)
	// UpdateInferenceSecret updates an existing summarization inference secret
	// in the backend. The returned object contains the secret value and should
	// be handled with care.
	UpdateInferenceSecret(ctx context.Context, secret *summarizerv1.InferenceSecret) (*summarizerv1.InferenceSecret, error)
	// UpsertInferenceSecret creates or updates a summarization inference
	// secretin the backend. If the secret already exists, it will be updated.
	// The returned object contains the secret value and should be handled with
	// care.
	UpsertInferenceSecret(ctx context.Context, secret *summarizerv1.InferenceSecret) (*summarizerv1.InferenceSecret, error)
	// DeleteInferenceSecret deletes a summarization inference secret from the
	// backend by name.
	DeleteInferenceSecret(ctx context.Context, name string) error
	// ListInferenceSecrets lists summarization inference secrets in the backend
	// with pagination support. Returns a slice of secrets and a next page token.
	// The returned objects contain the secret values and should be handled with
	// care.
	ListInferenceSecrets(ctx context.Context, size int, pageToken string) ([]*summarizerv1.InferenceSecret, string, error)

	// CreateInferencePolicy creates a new summarization inference policy in the
	// backend.
	CreateInferencePolicy(ctx context.Context, policy *summarizerv1.InferencePolicy) (*summarizerv1.InferencePolicy, error)
	// GetInferencePolicy retrieves a summarization inference policy from the
	// backend by name.
	GetInferencePolicy(ctx context.Context, name string) (*summarizerv1.InferencePolicy, error)
	// UpdateInferencePolicy updates an existing summarization inference policy
	// in the backend.
	UpdateInferencePolicy(ctx context.Context, policy *summarizerv1.InferencePolicy) (*summarizerv1.InferencePolicy, error)
	// UpsertInferencePolicy creates or updates a summarization inference policy
	// in the backend. If the policy already exists, it will be updated.
	UpsertInferencePolicy(ctx context.Context, policy *summarizerv1.InferencePolicy) (*summarizerv1.InferencePolicy, error)
	// DeleteInferencePolicy deletes a summarization inference policy from the
	// backend by name.
	DeleteInferencePolicy(ctx context.Context, name string) error
	// ListInferencePolicies lists summarization inference policies in the
	// backend with pagination support. Returns a slice of policies and a next
	// page token.
	ListInferencePolicies(ctx context.Context, size int, pageToken string) ([]*summarizerv1.InferencePolicy, string, error)
	// AllInferencePolicies retrieves all summarization inference policies from
	// the backend, without pagination.
	AllInferencePolicies(ctx context.Context) iter.Seq2[*summarizerv1.InferencePolicy, error]
}
