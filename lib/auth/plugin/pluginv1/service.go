// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package pluginv1

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// NotImplementedService is a [pluginspb.PluginServiceServer] which returns
// errors for all RPCs that indicate that enterprise is required to use the
// service. Using a [pluginspb.UnimplementedPluginServiceServer] would result
// in ambiguous not implemented errors being returned from open source.
type NotImplementedService struct {
	pluginspb.UnimplementedPluginServiceServer
}

func (NotImplementedService) CreatePlugin(context.Context, *pluginspb.CreatePluginRequest) (*emptypb.Empty, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) GetPlugin(context.Context, *pluginspb.GetPluginRequest) (*types.PluginV1, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) UpdatePlugin(context.Context, *pluginspb.UpdatePluginRequest) (*types.PluginV1, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) DeletePlugin(context.Context, *pluginspb.DeletePluginRequest) (*emptypb.Empty, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) ListPlugins(context.Context, *pluginspb.ListPluginsRequest) (*pluginspb.ListPluginsResponse, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) SetPluginCredentials(context.Context, *pluginspb.SetPluginCredentialsRequest) (*emptypb.Empty, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) SetPluginStatus(context.Context, *pluginspb.SetPluginStatusRequest) (*emptypb.Empty, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) GetAvailablePluginTypes(context.Context, *pluginspb.GetAvailablePluginTypesRequest) (*pluginspb.GetAvailablePluginTypesResponse, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) SearchPluginStaticCredentials(context.Context, *pluginspb.SearchPluginStaticCredentialsRequest) (*pluginspb.SearchPluginStaticCredentialsResponse, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) UpdatePluginStaticCredentials(context.Context, *pluginspb.UpdatePluginStaticCredentialsRequest) (*pluginspb.UpdatePluginStaticCredentialsResponse, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) NeedsCleanup(context.Context, *pluginspb.NeedsCleanupRequest) (*pluginspb.NeedsCleanupResponse, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) Cleanup(context.Context, *pluginspb.CleanupRequest) (*emptypb.Empty, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) CreatePluginOauthToken(context.Context, *pluginspb.CreatePluginOauthTokenRequest) (*pluginspb.CreatePluginOauthTokenResponse, error) {
	return nil, services.ErrRequiresEnterprise
}
