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

func (s *SummarizerService) CreateInferenceModel(
	ctx context.Context, req *pb.CreateInferenceModelRequest,
) (*pb.InferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) GetInferenceModel(
	ctx context.Context, req *pb.GetInferenceModelRequest,
) (*pb.InferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpdateInferenceModel(
	ctx context.Context, req *pb.UpdateInferenceModelRequest,
) (*pb.InferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpsertInferenceModel(
	ctx context.Context, req *pb.UpsertInferenceModelRequest,
) (*pb.InferenceModel, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) DeleteInferenceModel(
	ctx context.Context, req *pb.DeleteInferenceModelRequest,
) (*emptypb.Empty, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) ListInferenceModels(
	ctx context.Context, req *pb.ListInferenceModelsRequest,
) (*pb.ListInferenceModelsResponse, error) {
	return nil, s.requireEnterprise()
}

// CRUD operations for secrets

func (s *SummarizerService) CreateInferenceSecret(
	ctx context.Context, req *pb.CreateInferenceSecretRequest,
) (*pb.InferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) GetInferenceSecret(
	ctx context.Context, req *pb.GetInferenceSecretRequest,
) (*pb.InferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpdateInferenceSecret(
	ctx context.Context, req *pb.UpdateInferenceSecretRequest,
) (*pb.InferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpsertInferenceSecret(
	ctx context.Context, req *pb.UpsertInferenceSecretRequest,
) (*pb.InferenceSecret, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) DeleteInferenceSecret(
	ctx context.Context, req *pb.DeleteInferenceSecretRequest,
) (*emptypb.Empty, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) ListInferenceSecrets(
	ctx context.Context, req *pb.ListInferenceSecretsRequest,
) (*pb.ListInferenceSecretsResponse, error) {
	return nil, s.requireEnterprise()
}

// CRUD operations for policies

func (s *SummarizerService) CreateInferencePolicy(
	ctx context.Context, req *pb.CreateInferencePolicyRequest,
) (*pb.InferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) GetInferencePolicy(
	ctx context.Context, req *pb.GetInferencePolicyRequest,
) (*pb.InferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpdateInferencePolicy(
	ctx context.Context, req *pb.UpdateInferencePolicyRequest,
) (*pb.InferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) UpsertInferencePolicy(
	ctx context.Context, req *pb.UpsertInferencePolicyRequest,
) (*pb.InferencePolicy, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) DeleteInferencePolicy(
	ctx context.Context, req *pb.DeleteInferencePolicyRequest,
) (*emptypb.Empty, error) {
	return nil, s.requireEnterprise()
}

func (s *SummarizerService) ListInferencePolicies(
	ctx context.Context, req *pb.ListInferencePoliciesRequest,
) (*pb.ListInferencePoliciesResponse, error) {
	return nil, s.requireEnterprise()
}

func (*SummarizerService) requireEnterprise() error {
	return trace.AccessDenied("session recording summarization is only available with an enterprise license")
}
