/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package resources

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestBeamsConfigCollection_WriteText(t *testing.T) {
	config := services.DefaultBeamsConfig()

	table := asciitable.MakeTable(
		[]string{"Anthropic App", "OpenAI App"},
		[]string{"anthropic", "openai"},
	)
	formatted := table.AsBuffer().String()

	collectionFormatTest(t, &beamsConfigCollection{config: config}, formatted, formatted)
}

type mockBeamsConfigServiceServer struct {
	beamsv1.UnimplementedBeamsConfigServiceServer

	storage *local.BeamsConfigService
}

func newMockBeamsConfigServiceServer(t *testing.T) *mockBeamsConfigServiceServer {
	t.Helper()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	storage, err := local.NewBeamsConfigService(bk)
	require.NoError(t, err)
	return &mockBeamsConfigServiceServer{storage: storage}
}

func (m *mockBeamsConfigServiceServer) register(svc grpc.ServiceRegistrar) {
	beamsv1.RegisterBeamsConfigServiceServer(svc, m)
}

func (m *mockBeamsConfigServiceServer) GetBeamsConfig(ctx context.Context, _ *beamsv1.GetBeamsConfigRequest) (*beamsv1.GetBeamsConfigResponse, error) {
	config, err := m.storage.GetBeamsConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return beamsv1.GetBeamsConfigResponse_builder{
		BeamsConfig: config,
	}.Build(), nil
}

func (m *mockBeamsConfigServiceServer) CreateBeamsConfig(ctx context.Context, req *beamsv1.CreateBeamsConfigRequest) (*beamsv1.CreateBeamsConfigResponse, error) {
	config, err := m.storage.CreateBeamsConfig(ctx, req.GetBeamsConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return beamsv1.CreateBeamsConfigResponse_builder{
		BeamsConfig: config,
	}.Build(), nil
}

func (m *mockBeamsConfigServiceServer) UpdateBeamsConfig(ctx context.Context, req *beamsv1.UpdateBeamsConfigRequest) (*beamsv1.UpdateBeamsConfigResponse, error) {
	config, err := m.storage.UpdateBeamsConfig(ctx, req.GetBeamsConfig())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return beamsv1.UpdateBeamsConfigResponse_builder{
		BeamsConfig: config,
	}.Build(), nil
}

func (m *mockBeamsConfigServiceServer) DeleteBeamsConfig(ctx context.Context, _ *beamsv1.DeleteBeamsConfigRequest) (*beamsv1.DeleteBeamsConfigResponse, error) {
	if err := m.storage.DeleteBeamsConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return beamsv1.DeleteBeamsConfigResponse_builder{}.Build(), nil
}
