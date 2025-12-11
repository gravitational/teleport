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

package resources

import (
	"context"
	"maps"
	"slices"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

type mockSummarizerServiceServer struct {
	summarizerv1.SummarizerServiceServer

	models  map[string]*summarizerv1.InferenceModel
	secrets map[string]*summarizerv1.InferenceSecret
}

func registerMockSummarizerServiceServer(svc grpc.ServiceRegistrar) {
	summarizerv1.RegisterSummarizerServiceServer(svc, &mockSummarizerServiceServer{
		models:  make(map[string]*summarizerv1.InferenceModel),
		secrets: make(map[string]*summarizerv1.InferenceSecret),
	})
}

func (m *mockSummarizerServiceServer) ListInferenceModels(context.Context, *summarizerv1.ListInferenceModelsRequest) (*summarizerv1.ListInferenceModelsResponse, error) {
	return &summarizerv1.ListInferenceModelsResponse{
		Models:        slices.Concat(slices.Collect(maps.Values(m.models))),
		NextPageToken: "",
	}, nil
}

func (m *mockSummarizerServiceServer) CreateInferenceModel(ctx context.Context, req *summarizerv1.CreateInferenceModelRequest) (*summarizerv1.CreateInferenceModelResponse, error) {
	name := req.Model.Metadata.Name
	if _, exists := m.models[name]; exists {
		return nil, trace.AlreadyExists("inference model %q already exists", name)
	}
	m.models[name] = req.Model
	return &summarizerv1.CreateInferenceModelResponse{Model: req.Model}, nil
}

func (m *mockSummarizerServiceServer) GetInferenceModel(ctx context.Context, req *summarizerv1.GetInferenceModelRequest) (*summarizerv1.GetInferenceModelResponse, error) {
	model, exists := m.models[req.Name]
	if !exists {
		return nil, trace.NotFound("inference model %q not found", req.Name)
	}
	return &summarizerv1.GetInferenceModelResponse{Model: model}, nil
}

func (m *mockSummarizerServiceServer) UpdateInferenceModel(ctx context.Context, req *summarizerv1.UpdateInferenceModelRequest) (*summarizerv1.UpdateInferenceModelResponse, error) {
	name := req.Model.Metadata.Name
	if _, exists := m.models[name]; !exists {
		return nil, trace.NotFound("inference model %q not found", name)
	}
	req.Model.Metadata.Revision = uuid.NewString()
	m.models[name] = req.Model
	return &summarizerv1.UpdateInferenceModelResponse{Model: req.Model}, nil
}

func (m *mockSummarizerServiceServer) UpsertInferenceModel(ctx context.Context, req *summarizerv1.UpsertInferenceModelRequest) (*summarizerv1.UpsertInferenceModelResponse, error) {
	m.models[req.Model.Metadata.Name] = req.Model
	return &summarizerv1.UpsertInferenceModelResponse{Model: req.Model}, nil
}

func (m *mockSummarizerServiceServer) DeleteInferenceModel(ctx context.Context, req *summarizerv1.DeleteInferenceModelRequest) (*summarizerv1.DeleteInferenceModelResponse, error) {
	if _, exists := m.models[req.Name]; !exists {
		return nil, trace.NotFound("inference model %q not found", req.Name)
	}
	delete(m.models, req.Name)
	return &summarizerv1.DeleteInferenceModelResponse{}, nil
}

func (m *mockSummarizerServiceServer) ListInferenceSecrets(context.Context, *summarizerv1.ListInferenceSecretsRequest) (*summarizerv1.ListInferenceSecretsResponse, error) {
	return &summarizerv1.ListInferenceSecretsResponse{
		Secrets:       slices.Concat(slices.Collect(maps.Values(m.secrets))),
		NextPageToken: "",
	}, nil
}

func (m *mockSummarizerServiceServer) CreateInferenceSecret(ctx context.Context, req *summarizerv1.CreateInferenceSecretRequest) (*summarizerv1.CreateInferenceSecretResponse, error) {
	name := req.Secret.Metadata.Name
	if _, exists := m.secrets[name]; exists {
		return nil, trace.AlreadyExists("inference secret %q already exists", name)
	}
	m.secrets[name] = req.Secret
	return &summarizerv1.CreateInferenceSecretResponse{Secret: req.Secret}, nil
}

func (m *mockSummarizerServiceServer) GetInferenceSecret(ctx context.Context, req *summarizerv1.GetInferenceSecretRequest) (*summarizerv1.GetInferenceSecretResponse, error) {
	secret, exists := m.secrets[req.Name]
	if !exists {
		return nil, trace.NotFound("inference secret %q not found", req.Name)
	}
	return &summarizerv1.GetInferenceSecretResponse{Secret: secret}, nil
}

func (m *mockSummarizerServiceServer) UpdateInferenceSecret(ctx context.Context, req *summarizerv1.UpdateInferenceSecretRequest) (*summarizerv1.UpdateInferenceSecretResponse, error) {
	name := req.Secret.Metadata.Name
	if _, exists := m.secrets[name]; !exists {
		return nil, trace.NotFound("inference secret %q not found", name)
	}
	req.Secret.Metadata.Revision = uuid.NewString()
	m.secrets[name] = req.Secret
	return &summarizerv1.UpdateInferenceSecretResponse{Secret: req.Secret}, nil
}

func (m *mockSummarizerServiceServer) UpsertInferenceSecret(ctx context.Context, req *summarizerv1.UpsertInferenceSecretRequest) (*summarizerv1.UpsertInferenceSecretResponse, error) {
	m.secrets[req.Secret.Metadata.Name] = req.Secret
	return &summarizerv1.UpsertInferenceSecretResponse{Secret: req.Secret}, nil
}

func (m *mockSummarizerServiceServer) DeleteInferenceSecret(ctx context.Context, req *summarizerv1.DeleteInferenceSecretRequest) (*summarizerv1.DeleteInferenceSecretResponse, error) {
	if _, exists := m.secrets[req.Name]; !exists {
		return nil, trace.NotFound("inference secret %q not found", req.Name)
	}
	delete(m.secrets, req.Name)
	return &summarizerv1.DeleteInferenceSecretResponse{}, nil
}
