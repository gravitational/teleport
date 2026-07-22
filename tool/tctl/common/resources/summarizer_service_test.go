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

	models   map[string]*summarizerv1.InferenceModel
	secrets  map[string]*summarizerv1.InferenceSecret
	policies map[string]*summarizerv1.InferencePolicy
}

func registerMockSummarizerServiceServer(svc grpc.ServiceRegistrar) {
	summarizerv1.RegisterSummarizerServiceServer(svc, &mockSummarizerServiceServer{
		models:   make(map[string]*summarizerv1.InferenceModel),
		secrets:  make(map[string]*summarizerv1.InferenceSecret),
		policies: make(map[string]*summarizerv1.InferencePolicy),
	})
}

func (m *mockSummarizerServiceServer) ListInferenceModels(context.Context, *summarizerv1.ListInferenceModelsRequest) (*summarizerv1.ListInferenceModelsResponse, error) {
	return summarizerv1.ListInferenceModelsResponse_builder{
		Models:        slices.Concat(slices.Collect(maps.Values(m.models))),
		NextPageToken: "",
	}.Build(), nil
}

func (m *mockSummarizerServiceServer) CreateInferenceModel(ctx context.Context, req *summarizerv1.CreateInferenceModelRequest) (*summarizerv1.CreateInferenceModelResponse, error) {
	name := req.GetModel().GetMetadata().GetName()
	if _, exists := m.models[name]; exists {
		return nil, trace.AlreadyExists("inference model %q already exists", name)
	}
	m.models[name] = req.GetModel()
	return summarizerv1.CreateInferenceModelResponse_builder{Model: req.GetModel()}.Build(), nil
}

func (m *mockSummarizerServiceServer) GetInferenceModel(ctx context.Context, req *summarizerv1.GetInferenceModelRequest) (*summarizerv1.GetInferenceModelResponse, error) {
	model, exists := m.models[req.GetName()]
	if !exists {
		return nil, trace.NotFound("inference model %q not found", req.GetName())
	}
	return summarizerv1.GetInferenceModelResponse_builder{Model: model}.Build(), nil
}

func (m *mockSummarizerServiceServer) UpdateInferenceModel(ctx context.Context, req *summarizerv1.UpdateInferenceModelRequest) (*summarizerv1.UpdateInferenceModelResponse, error) {
	name := req.GetModel().GetMetadata().GetName()
	if _, exists := m.models[name]; !exists {
		return nil, trace.NotFound("inference model %q not found", name)
	}
	req.GetModel().GetMetadata().SetRevision(uuid.NewString())
	m.models[name] = req.GetModel()
	return summarizerv1.UpdateInferenceModelResponse_builder{Model: req.GetModel()}.Build(), nil
}

func (m *mockSummarizerServiceServer) UpsertInferenceModel(ctx context.Context, req *summarizerv1.UpsertInferenceModelRequest) (*summarizerv1.UpsertInferenceModelResponse, error) {
	m.models[req.GetModel().GetMetadata().GetName()] = req.GetModel()
	return summarizerv1.UpsertInferenceModelResponse_builder{Model: req.GetModel()}.Build(), nil
}

func (m *mockSummarizerServiceServer) DeleteInferenceModel(ctx context.Context, req *summarizerv1.DeleteInferenceModelRequest) (*summarizerv1.DeleteInferenceModelResponse, error) {
	if _, exists := m.models[req.GetName()]; !exists {
		return nil, trace.NotFound("inference model %q not found", req.GetName())
	}
	delete(m.models, req.GetName())
	return &summarizerv1.DeleteInferenceModelResponse{}, nil
}

func (m *mockSummarizerServiceServer) ListInferenceSecrets(context.Context, *summarizerv1.ListInferenceSecretsRequest) (*summarizerv1.ListInferenceSecretsResponse, error) {
	return summarizerv1.ListInferenceSecretsResponse_builder{
		Secrets:       slices.Concat(slices.Collect(maps.Values(m.secrets))),
		NextPageToken: "",
	}.Build(), nil
}

func (m *mockSummarizerServiceServer) CreateInferenceSecret(ctx context.Context, req *summarizerv1.CreateInferenceSecretRequest) (*summarizerv1.CreateInferenceSecretResponse, error) {
	name := req.GetSecret().GetMetadata().GetName()
	if _, exists := m.secrets[name]; exists {
		return nil, trace.AlreadyExists("inference secret %q already exists", name)
	}
	m.secrets[name] = req.GetSecret()
	return summarizerv1.CreateInferenceSecretResponse_builder{Secret: req.GetSecret()}.Build(), nil
}

func (m *mockSummarizerServiceServer) GetInferenceSecret(ctx context.Context, req *summarizerv1.GetInferenceSecretRequest) (*summarizerv1.GetInferenceSecretResponse, error) {
	secret, exists := m.secrets[req.GetName()]
	if !exists {
		return nil, trace.NotFound("inference secret %q not found", req.GetName())
	}
	return summarizerv1.GetInferenceSecretResponse_builder{Secret: secret}.Build(), nil
}

func (m *mockSummarizerServiceServer) UpdateInferenceSecret(ctx context.Context, req *summarizerv1.UpdateInferenceSecretRequest) (*summarizerv1.UpdateInferenceSecretResponse, error) {
	name := req.GetSecret().GetMetadata().GetName()
	if _, exists := m.secrets[name]; !exists {
		return nil, trace.NotFound("inference secret %q not found", name)
	}
	req.GetSecret().GetMetadata().SetRevision(uuid.NewString())
	m.secrets[name] = req.GetSecret()
	return summarizerv1.UpdateInferenceSecretResponse_builder{Secret: req.GetSecret()}.Build(), nil
}

func (m *mockSummarizerServiceServer) UpsertInferenceSecret(ctx context.Context, req *summarizerv1.UpsertInferenceSecretRequest) (*summarizerv1.UpsertInferenceSecretResponse, error) {
	m.secrets[req.GetSecret().GetMetadata().GetName()] = req.GetSecret()
	return summarizerv1.UpsertInferenceSecretResponse_builder{Secret: req.GetSecret()}.Build(), nil
}

func (m *mockSummarizerServiceServer) DeleteInferenceSecret(ctx context.Context, req *summarizerv1.DeleteInferenceSecretRequest) (*summarizerv1.DeleteInferenceSecretResponse, error) {
	if _, exists := m.secrets[req.GetName()]; !exists {
		return nil, trace.NotFound("inference secret %q not found", req.GetName())
	}
	delete(m.secrets, req.GetName())
	return &summarizerv1.DeleteInferenceSecretResponse{}, nil
}

func (m *mockSummarizerServiceServer) ListInferencePolicies(context.Context, *summarizerv1.ListInferencePoliciesRequest) (*summarizerv1.ListInferencePoliciesResponse, error) {
	return summarizerv1.ListInferencePoliciesResponse_builder{
		Policies:      slices.Concat(slices.Collect(maps.Values(m.policies))),
		NextPageToken: "",
	}.Build(), nil
}

func (m *mockSummarizerServiceServer) CreateInferencePolicy(ctx context.Context, req *summarizerv1.CreateInferencePolicyRequest) (*summarizerv1.CreateInferencePolicyResponse, error) {
	name := req.GetPolicy().GetMetadata().GetName()
	if _, exists := m.policies[name]; exists {
		return nil, trace.AlreadyExists("inference policy %q already exists", name)
	}
	m.policies[name] = req.GetPolicy()
	return summarizerv1.CreateInferencePolicyResponse_builder{Policy: req.GetPolicy()}.Build(), nil
}

func (m *mockSummarizerServiceServer) GetInferencePolicy(ctx context.Context, req *summarizerv1.GetInferencePolicyRequest) (*summarizerv1.GetInferencePolicyResponse, error) {
	policy, exists := m.policies[req.GetName()]
	if !exists {
		return nil, trace.NotFound("inference policy %q not found", req.GetName())
	}
	return summarizerv1.GetInferencePolicyResponse_builder{Policy: policy}.Build(), nil
}

func (m *mockSummarizerServiceServer) UpdateInferencePolicy(ctx context.Context, req *summarizerv1.UpdateInferencePolicyRequest) (*summarizerv1.UpdateInferencePolicyResponse, error) {
	name := req.GetPolicy().GetMetadata().GetName()
	if _, exists := m.policies[name]; !exists {
		return nil, trace.NotFound("inference policy %q not found", name)
	}
	req.GetPolicy().GetMetadata().SetRevision(uuid.NewString())
	m.policies[name] = req.GetPolicy()
	return summarizerv1.UpdateInferencePolicyResponse_builder{Policy: req.GetPolicy()}.Build(), nil
}

func (m *mockSummarizerServiceServer) UpsertInferencePolicy(ctx context.Context, req *summarizerv1.UpsertInferencePolicyRequest) (*summarizerv1.UpsertInferencePolicyResponse, error) {
	m.policies[req.GetPolicy().GetMetadata().GetName()] = req.GetPolicy()
	return summarizerv1.UpsertInferencePolicyResponse_builder{Policy: req.GetPolicy()}.Build(), nil
}

func (m *mockSummarizerServiceServer) DeleteInferencePolicy(ctx context.Context, req *summarizerv1.DeleteInferencePolicyRequest) (*summarizerv1.DeleteInferencePolicyResponse, error) {
	if _, exists := m.policies[req.GetName()]; !exists {
		return nil, trace.NotFound("inference policy %q not found", req.GetName())
	}
	delete(m.policies, req.GetName())
	return &summarizerv1.DeleteInferencePolicyResponse{}, nil
}
