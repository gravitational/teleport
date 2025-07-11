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

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// SummarizerService summarizes session recordings using language model
// inference. It contains gRPC methods for CRUD operations on the configuration
// resources, as well as the actual summarization method.
type SummarizerService interface {
	// Summarize summarizes a session recording with a given ID. The
	// sessionEndEvent is optional, but should be specified if possible, as it
	// lets us skip reading the session stream just to find the end event.
	Summarize(ctx context.Context, sessionID session.ID, sessionEndEvent *events.OneOf) error
	pb.SummarizerServiceServer
}

func NewSummarizerWrapper() *SummarizerWrapper {
	return &SummarizerWrapper{
		SummarizerService: &unimplementedSummarizer{},
	}
}

// SummarizerWrapper is a wrapper around the SummarizerService interface. Its
// purpose is to allow substituting the wrapped service after a dependent
// service has been configured with the wrapper as the service implementation.
type SummarizerWrapper struct {
	SummarizerService
}

// NewSummarizerService creates a new OSS version of the SummarizerService. It
// returns a licensing error from every RPC.
func NewSummarizerService() SummarizerService {
	return &unimplementedSummarizer{}
}

// unimplementedSummarizer is an OSS version of the unimplementedSummarizer. It returns
// a licensing error from every RPC.
type unimplementedSummarizer struct {
	pb.UnimplementedSummarizerServiceServer
}

var _ SummarizerService = (*unimplementedSummarizer)(nil)

func (s *unimplementedSummarizer) Summarize(
	ctx context.Context, sessionID session.ID, sessionEndEvent *events.OneOf,
) error {
	return requireEnterprise()
}

// CRUD operations for models

func (s *unimplementedSummarizer) CreateInferenceModel(
	ctx context.Context, req *pb.CreateInferenceModelRequest,
) (*pb.CreateInferenceModelResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) GetInferenceModel(
	ctx context.Context, req *pb.GetInferenceModelRequest,
) (*pb.GetInferenceModelResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) UpdateInferenceModel(
	ctx context.Context, req *pb.UpdateInferenceModelRequest,
) (*pb.UpdateInferenceModelResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) UpsertInferenceModel(
	ctx context.Context, req *pb.UpsertInferenceModelRequest,
) (*pb.UpsertInferenceModelResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) DeleteInferenceModel(
	ctx context.Context, req *pb.DeleteInferenceModelRequest,
) (*pb.DeleteInferenceModelResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) ListInferenceModels(
	ctx context.Context, req *pb.ListInferenceModelsRequest,
) (*pb.ListInferenceModelsResponse, error) {
	return nil, requireEnterprise()
}

// CRUD operations for secrets

func (s *unimplementedSummarizer) CreateInferenceSecret(
	ctx context.Context, req *pb.CreateInferenceSecretRequest,
) (*pb.CreateInferenceSecretResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) GetInferenceSecret(
	ctx context.Context, req *pb.GetInferenceSecretRequest,
) (*pb.GetInferenceSecretResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) UpdateInferenceSecret(
	ctx context.Context, req *pb.UpdateInferenceSecretRequest,
) (*pb.UpdateInferenceSecretResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) UpsertInferenceSecret(
	ctx context.Context, req *pb.UpsertInferenceSecretRequest,
) (*pb.UpsertInferenceSecretResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) DeleteInferenceSecret(
	ctx context.Context, req *pb.DeleteInferenceSecretRequest,
) (*pb.DeleteInferenceSecretResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) ListInferenceSecrets(
	ctx context.Context, req *pb.ListInferenceSecretsRequest,
) (*pb.ListInferenceSecretsResponse, error) {
	return nil, requireEnterprise()
}

// CRUD operations for policies

func (s *unimplementedSummarizer) CreateInferencePolicy(
	ctx context.Context, req *pb.CreateInferencePolicyRequest,
) (*pb.CreateInferencePolicyResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) GetInferencePolicy(
	ctx context.Context, req *pb.GetInferencePolicyRequest,
) (*pb.GetInferencePolicyResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) UpdateInferencePolicy(
	ctx context.Context, req *pb.UpdateInferencePolicyRequest,
) (*pb.UpdateInferencePolicyResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) UpsertInferencePolicy(
	ctx context.Context, req *pb.UpsertInferencePolicyRequest,
) (*pb.UpsertInferencePolicyResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) DeleteInferencePolicy(
	ctx context.Context, req *pb.DeleteInferencePolicyRequest,
) (*pb.DeleteInferencePolicyResponse, error) {
	return nil, requireEnterprise()
}

func (s *unimplementedSummarizer) ListInferencePolicies(
	ctx context.Context, req *pb.ListInferencePoliciesRequest,
) (*pb.ListInferencePoliciesResponse, error) {
	return nil, requireEnterprise()
}

func requireEnterprise() error {
	return trace.AccessDenied(
		"session recording summarization is only available with an enterprise license that supports Teleport Identity Security")
}
