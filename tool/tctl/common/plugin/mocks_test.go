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

package plugin

import (
	"context"

	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/client/proto"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
)

type mockPluginsClient struct {
	mock.Mock
}

func (m *mockPluginsClient) CreatePlugin(ctx context.Context, in *pluginsv1.CreatePluginRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	result := m.Called(ctx, in, opts)
	return result.Get(0).(*emptypb.Empty), result.Error(1)
}

func (m *mockPluginsClient) GetPlugin(ctx context.Context, in *pluginsv1.GetPluginRequest, opts ...grpc.CallOption) (*types.PluginV1, error) {
	result := m.Called(ctx, in, opts)
	return result.Get(0).(*types.PluginV1), result.Error(1)
}

func (m *mockPluginsClient) UpdatePlugin(ctx context.Context, in *pluginsv1.UpdatePluginRequest, opts ...grpc.CallOption) (*types.PluginV1, error) {
	result := m.Called(ctx, in, opts)
	return result.Get(0).(*types.PluginV1), result.Error(1)
}

func (m *mockPluginsClient) NeedsCleanup(ctx context.Context, in *pluginsv1.NeedsCleanupRequest, opts ...grpc.CallOption) (*pluginsv1.NeedsCleanupResponse, error) {
	result := m.Called(ctx, in, opts)
	return result.Get(0).(*pluginsv1.NeedsCleanupResponse), result.Error(1)
}

func (m *mockPluginsClient) Cleanup(ctx context.Context, in *pluginsv1.CleanupRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	result := m.Called(ctx, in, opts)
	return result.Get(0).(*emptypb.Empty), result.Error(1)
}

func (m *mockPluginsClient) DeletePlugin(ctx context.Context, in *pluginsv1.DeletePluginRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	result := m.Called(ctx, in, opts)
	return result.Get(0).(*emptypb.Empty), result.Error(1)
}

func (m *mockPluginsClient) UpdatePluginStaticCredentials(ctx context.Context, in *pluginsv1.UpdatePluginStaticCredentialsRequest, opts ...grpc.CallOption) (*pluginsv1.UpdatePluginStaticCredentialsResponse, error) {
	result := m.Called(ctx, in, opts)
	var response *pluginsv1.UpdatePluginStaticCredentialsResponse

	if fn, ok := result.Get(0).(func(context.Context, *pluginsv1.UpdatePluginStaticCredentialsRequest, ...grpc.CallOption) (*pluginsv1.UpdatePluginStaticCredentialsResponse, error)); ok {
		return fn(ctx, in, opts...)
	}

	if r, ok := result.Get(0).(*pluginsv1.UpdatePluginStaticCredentialsResponse); ok {
		response = r
	}
	return response, result.Error(1)
}

type mockAuthClient struct {
	mock.Mock
}

func (m *mockAuthClient) GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error) {
	result := m.Called(ctx, id, withSecrets)
	return result.Get(0).(types.SAMLConnector), result.Error(1)
}
func (m *mockAuthClient) CreateSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	result := m.Called(ctx, connector)
	return result.Get(0).(types.SAMLConnector), result.Error(1)
}
func (m *mockAuthClient) UpsertSAMLConnector(ctx context.Context, connector types.SAMLConnector) (types.SAMLConnector, error) {
	result := m.Called(ctx, connector)
	return result.Get(0).(types.SAMLConnector), result.Error(1)
}
func (m *mockAuthClient) CreateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	result := m.Called(ctx, ig)
	return result.Get(0).(types.Integration), result.Error(1)
}
func (m *mockAuthClient) UpdateIntegration(ctx context.Context, ig types.Integration) (types.Integration, error) {
	result := m.Called(ctx, ig)
	return result.Get(0).(types.Integration), result.Error(1)
}

func (m *mockAuthClient) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	result := m.Called(ctx, name)
	return result.Get(0).(types.Integration), result.Error(1)
}

func (m *mockAuthClient) Ping(ctx context.Context) (proto.PingResponse, error) {
	result := m.Called(ctx)
	return result.Get(0).(proto.PingResponse), result.Error(1)
}

func (m *mockAuthClient) PerformMFACeremony(ctx context.Context, challengeRequest *proto.CreateAuthenticateChallengeRequest, promptOpts ...mfa.PromptOpt) (*proto.MFAAuthenticateResponse, error) {
	return &proto.MFAAuthenticateResponse{}, nil
}

func (m *mockAuthClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	result := m.Called(ctx, name)
	return result.Get(0).(types.Role), result.Error(1)
}

// anyContext is an argument matcher for testify mocks that matches any context.
var anyContext any = mock.MatchedBy(func(context.Context) bool { return true })
