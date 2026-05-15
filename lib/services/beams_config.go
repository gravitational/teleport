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

package services

import (
	"context"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

// BeamsConfigGetter is an interface for getting the cluster singleton BeamsConfig.
type BeamsConfigGetter interface {
	// GetBeamsConfig returns the singleton BeamsConfig resource.
	GetBeamsConfig(ctx context.Context) (*beamsv1.BeamsConfig, error)
}

// BeamsConfigService is an interface for the BeamsConfig service.
type BeamsConfigService interface {
	BeamsConfigGetter

	// CreateBeamsConfig creates a new BeamsConfig resource.
	CreateBeamsConfig(ctx context.Context, config *beamsv1.BeamsConfig) (*beamsv1.BeamsConfig, error)

	// UpdateBeamsConfig updates an existing BeamsConfig resource using conditional update.
	UpdateBeamsConfig(ctx context.Context, config *beamsv1.BeamsConfig) (*beamsv1.BeamsConfig, error)

	// DeleteBeamsConfig deletes the singleton BeamsConfig resource.
	DeleteBeamsConfig(ctx context.Context) error
}

// DefaultCloudManagedBeamsConfig returns the default BeamsConfig with
// cloud-managed LLM endpoints for Anthropic and OpenAI.
func DefaultCloudManagedBeamsConfig() *beamsv1.BeamsConfig {
	return beamsv1.BeamsConfig_builder{
		Kind:    types.KindBeamsConfig,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameBeamsConfig,
		},
		Spec: beamsv1.BeamsConfigSpec_builder{
			Llm: beamsv1.LLMConfig_builder{
				Anthropic: beamsv1.LLMEndpointConfig_builder{
					AppName:       "anthropic",
					ProvisionMode: types.BeamsProvisionModeCloudManaged,
				}.Build(),
				Openai: beamsv1.LLMEndpointConfig_builder{
					AppName:       "openai",
					ProvisionMode: types.BeamsProvisionModeCloudManaged,
				}.Build(),
			}.Build(),
		}.Build(),
	}.Build()
}
