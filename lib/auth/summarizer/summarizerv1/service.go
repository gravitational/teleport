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

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"
)

// NewSummarizerService creates a new OSS version of the SummarizerService. It
// returns a licensing error from every RPC.
func NewSummarizerService() *SummarizerService {
	return &SummarizerService{}
}

// SummarizerService is an OSS version of the SummarizerService. It returns
// a licensing error from every RPC.
type SummarizerService struct {
	pb.UnimplementedSummarizerServiceServer
}

// CRUD operations for models

func (s *SummarizerService) CreateSummarizationInferenceModel(
	ctx context.Context, req *pb.CreateSummarizationInferenceModelRequest,
) (*pb.SummarizationInferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) GetSummarizationInferenceModel(
	ctx context.Context, req *pb.GetSummarizationInferenceModelRequest,
) (*pb.SummarizationInferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpdateSummarizationInferenceModel(
	ctx context.Context, req *pb.UpdateSummarizationInferenceModelRequest,
) (*pb.SummarizationInferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpsertSummarizationInferenceModel(
	ctx context.Context, req *pb.UpsertSummarizationInferenceModelRequest,
) (*pb.SummarizationInferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) DeleteSummarizationInferenceModel(
	ctx context.Context, req *pb.DeleteSummarizationInferenceModelRequest,
) (*emptypb.Empty, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) ListSummarizationInferenceModels(
	ctx context.Context, req *pb.ListSummarizationInferenceModelsRequest,
) (*pb.ListSummarizationInferenceModelsResponse, error) {
	return nil, s.requireEnterprise()
}

// CRUD operations for secrets

func (s *SummarizerService) CreateSummarizationInferenceSecret(
	ctx context.Context, req *pb.CreateSummarizationInferenceSecretRequest,
) (*pb.SummarizationInferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) GetSummarizationInferenceSecret(
	ctx context.Context, req *pb.GetSummarizationInferenceSecretRequest,
) (*pb.SummarizationInferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpdateSummarizationInferenceSecret(
	ctx context.Context, req *pb.UpdateSummarizationInferenceSecretRequest,
) (*pb.SummarizationInferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpsertSummarizationInferenceSecret(
	ctx context.Context, req *pb.UpsertSummarizationInferenceSecretRequest,
) (*pb.SummarizationInferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) DeleteSummarizationInferenceSecret(
	ctx context.Context, req *pb.DeleteSummarizationInferenceSecretRequest,
) (*emptypb.Empty, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) ListSummarizationInferenceSecrets(
	ctx context.Context, req *pb.ListSummarizationInferenceSecretsRequest,
) (*pb.ListSummarizationInferenceSecretsResponse, error) {
	return nil, s.requireEnterprise()
}

// CRUD operations for policies

func (s *SummarizerService) CreateSummarizationInferencePolicy(
	ctx context.Context, req *pb.CreateSummarizationInferencePolicyRequest,
) (*pb.SummarizationInferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) GetSummarizationInferencePolicy(
	ctx context.Context, req *pb.GetSummarizationInferencePolicyRequest,
) (*pb.SummarizationInferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpdateSummarizationInferencePolicy(
	ctx context.Context, req *pb.UpdateSummarizationInferencePolicyRequest,
) (*pb.SummarizationInferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpsertSummarizationInferencePolicy(
	ctx context.Context, req *pb.UpsertSummarizationInferencePolicyRequest,
) (*pb.SummarizationInferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) DeleteSummarizationInferencePolicy(
	ctx context.Context, req *pb.DeleteSummarizationInferencePolicyRequest,
) (*emptypb.Empty, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) ListSummarizationInferencePolicies(
	ctx context.Context, req *pb.ListSummarizationInferencePoliciesRequest,
) (*pb.ListSummarizationInferencePoliciesResponse, error) {
	return nil, s.requireEnterprise()
}

func (*SummarizerService) requireEnterprise() error {
	return trace.AccessDenied("session recording summarization is only available with an enterprise license")
}
