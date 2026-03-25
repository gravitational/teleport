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

package summarizerv1

import (
	"context"

	"github.com/gravitational/trace"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

// NewService creates a new OSS version of the SummarizerService. It
// returns a licensing error from every RPC.
func NewService() *UnimplementedService {
	return &UnimplementedService{}
}

// UnimplementedService is an OSS version of the UnimplementedService. It
// returns a licensing error from every RPC.
type UnimplementedService struct {
	summarizerv1pb.UnimplementedSummarizerServiceServer
}

// CRUD operations for models

// CreateInferenceModel is supposed to create a new inference model, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) CreateInferenceModel(
	ctx context.Context, req *summarizerv1pb.CreateInferenceModelRequest,
) (*summarizerv1pb.CreateInferenceModelResponse, error) {
	return nil, requireEnterprise()
}

// GetInferenceModel is supposed to retrieve an inference model by name, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) GetInferenceModel(
	ctx context.Context, req *summarizerv1pb.GetInferenceModelRequest,
) (*summarizerv1pb.GetInferenceModelResponse, error) {
	return nil, requireEnterprise()
}

// UpdateInferenceModel is supposed to update an existing inference model, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) UpdateInferenceModel(
	ctx context.Context, req *summarizerv1pb.UpdateInferenceModelRequest,
) (*summarizerv1pb.UpdateInferenceModelResponse, error) {
	return nil, requireEnterprise()
}

// UpsertInferenceModel is supposed to create or update an inference model, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) UpsertInferenceModel(
	ctx context.Context, req *summarizerv1pb.UpsertInferenceModelRequest,
) (*summarizerv1pb.UpsertInferenceModelResponse, error) {
	return nil, requireEnterprise()
}

// DeleteInferenceModel is supposed to delete an inference model by name, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) DeleteInferenceModel(
	ctx context.Context, req *summarizerv1pb.DeleteInferenceModelRequest,
) (*summarizerv1pb.DeleteInferenceModelResponse, error) {
	return nil, requireEnterprise()
}

// ListInferenceModels is supposed to list inference models with pagination
// support, but returns an error indicating that this feature is only available
// in the enterprise version of Teleport.
func (s *UnimplementedService) ListInferenceModels(
	ctx context.Context, req *summarizerv1pb.ListInferenceModelsRequest,
) (*summarizerv1pb.ListInferenceModelsResponse, error) {
	return nil, requireEnterprise()
}

// CRUD operations for secrets

// CreateInferenceSecret is supposed to create a new inference secret, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) CreateInferenceSecret(
	ctx context.Context, req *summarizerv1pb.CreateInferenceSecretRequest,
) (*summarizerv1pb.CreateInferenceSecretResponse, error) {
	return nil, requireEnterprise()
}

// GetInferenceSecret is supposed to retrieve an inference secret by name, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) GetInferenceSecret(
	ctx context.Context, req *summarizerv1pb.GetInferenceSecretRequest,
) (*summarizerv1pb.GetInferenceSecretResponse, error) {
	return nil, requireEnterprise()
}

// UpdateInferenceSecret is supposed to update an existing inference secret,
// but returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) UpdateInferenceSecret(
	ctx context.Context, req *summarizerv1pb.UpdateInferenceSecretRequest,
) (*summarizerv1pb.UpdateInferenceSecretResponse, error) {
	return nil, requireEnterprise()
}

// UpsertInferenceSecret is supposed to create or update an inference secret,
// but returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) UpsertInferenceSecret(
	ctx context.Context, req *summarizerv1pb.UpsertInferenceSecretRequest,
) (*summarizerv1pb.UpsertInferenceSecretResponse, error) {
	return nil, requireEnterprise()
}

// DeleteInferenceSecret is supposed to delete an inference secret by name, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) DeleteInferenceSecret(
	ctx context.Context, req *summarizerv1pb.DeleteInferenceSecretRequest,
) (*summarizerv1pb.DeleteInferenceSecretResponse, error) {
	return nil, requireEnterprise()
}

// ListInferenceSecrets is supposed to list inference secrets with pagination
// support, but returns an error indicating that this feature is only available
// in the enterprise version of Teleport.
func (s *UnimplementedService) ListInferenceSecrets(
	ctx context.Context, req *summarizerv1pb.ListInferenceSecretsRequest,
) (*summarizerv1pb.ListInferenceSecretsResponse, error) {
	return nil, requireEnterprise()
}

// CRUD operations for policies

// CreateInferencePolicy is supposed to create a new inference policy, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) CreateInferencePolicy(
	ctx context.Context, req *summarizerv1pb.CreateInferencePolicyRequest,
) (*summarizerv1pb.CreateInferencePolicyResponse, error) {
	return nil, requireEnterprise()
}

// GetInferencePolicy is supposed to retrieve an inference policy by name, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) GetInferencePolicy(
	ctx context.Context, req *summarizerv1pb.GetInferencePolicyRequest,
) (*summarizerv1pb.GetInferencePolicyResponse, error) {
	return nil, requireEnterprise()
}

// UpdateInferencePolicy is supposed to update an existing inference policy,
// but returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) UpdateInferencePolicy(
	ctx context.Context, req *summarizerv1pb.UpdateInferencePolicyRequest,
) (*summarizerv1pb.UpdateInferencePolicyResponse, error) {
	return nil, requireEnterprise()
}

// UpsertInferencePolicy is supposed to create or update an inference policy,
// but returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) UpsertInferencePolicy(
	ctx context.Context, req *summarizerv1pb.UpsertInferencePolicyRequest,
) (*summarizerv1pb.UpsertInferencePolicyResponse, error) {
	return nil, requireEnterprise()
}

// DeleteInferencePolicy is supposed to delete an inference policy by name, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) DeleteInferencePolicy(
	ctx context.Context, req *summarizerv1pb.DeleteInferencePolicyRequest,
) (*summarizerv1pb.DeleteInferencePolicyResponse, error) {
	return nil, requireEnterprise()
}

// ListInferencePolicies is supposed to list inference policies with pagination
// support, but returns an error indicating that this feature is only available
// in the enterprise version of Teleport.
func (s *UnimplementedService) ListInferencePolicies(
	ctx context.Context, req *summarizerv1pb.ListInferencePoliciesRequest,
) (*summarizerv1pb.ListInferencePoliciesResponse, error) {
	return nil, requireEnterprise()
}

// IsEnabled is supposed to tell if the summarizer is enabled by the license
// and configured, but it always returns false. In contrast to other methods
// here, it doesn't return an error, since not having an enterprise license is
// still a valid case from this method's perspective.
func (s *UnimplementedService) IsEnabled(
	ctx context.Context, req *summarizerv1pb.IsEnabledRequest,
) (*summarizerv1pb.IsEnabledResponse, error) {
	return &summarizerv1pb.IsEnabledResponse{Enabled: false}, nil
}

func requireEnterprise() error {
	return trace.AccessDenied(
		"session recording summarization is only available with an enterprise license that supports Teleport Identity Security")
}
