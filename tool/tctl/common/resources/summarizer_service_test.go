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

	models map[string]*summarizerv1.InferenceModel
}

func registerMockSummarizerServiceServer(svc grpc.ServiceRegistrar) {
	summarizerv1.RegisterSummarizerServiceServer(svc, &mockSummarizerServiceServer{
		models: make(map[string]*summarizerv1.InferenceModel),
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
