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

package services

import (
	"context"

	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/types"
)

// ClusterBeamConfigReader defines methods for reading the cluster beam config.
type ClusterBeamConfigReader interface {
	// GetClusterBeamConfig returns the cluster-wide beam configuration.
	GetClusterBeamConfig(ctx context.Context) (*beamsv1.ClusterBeamConfig, error)
}

// ClusterBeamConfig is a service that manages the cluster-wide beam configuration.
type ClusterBeamConfig interface {
	ClusterBeamConfigReader

	// CreateClusterBeamConfig creates the cluster-wide beam configuration.
	CreateClusterBeamConfig(ctx context.Context, cfg *beamsv1.ClusterBeamConfig) (*beamsv1.ClusterBeamConfig, error)
	// UpdateClusterBeamConfig updates the cluster-wide beam configuration.
	UpdateClusterBeamConfig(ctx context.Context, cfg *beamsv1.ClusterBeamConfig) (*beamsv1.ClusterBeamConfig, error)
	// UpsertClusterBeamConfig creates or updates the cluster-wide beam configuration.
	UpsertClusterBeamConfig(ctx context.Context, cfg *beamsv1.ClusterBeamConfig) (*beamsv1.ClusterBeamConfig, error)
	// DeleteClusterBeamConfig removes the cluster-wide beam configuration.
	DeleteClusterBeamConfig(ctx context.Context) error
}

// ValidateClusterBeamConfig validates the given cluster beam config resource.
func ValidateClusterBeamConfig(cfg *beamsv1.ClusterBeamConfig) error {
	switch {
	case cfg == nil:
		return trace.BadParameter("cluster beam config must not be nil")
	case cfg.Version != types.V1:
		return trace.BadParameter("cluster beam config only supports version %q, got %q", types.V1, cfg.Version)
	case cfg.Kind != types.KindClusterBeamConfig:
		return trace.BadParameter("cluster beam config kind must be %q, got %q", types.KindClusterBeamConfig, cfg.Kind)
	case cfg.Metadata == nil:
		return trace.BadParameter("cluster beam config metadata is missing")
	case cfg.Metadata.Name != types.MetaNameClusterBeamConfig:
		return trace.BadParameter("cluster beam config metadata.name must be %q", types.MetaNameClusterBeamConfig)
	case cfg.Spec == nil:
		return trace.BadParameter("cluster beam config spec is missing")
	case cfg.Spec.Llm == nil:
		return trace.BadParameter("cluster beam config spec.llm is missing")
	case cfg.Spec.Llm.Openai == nil || cfg.Spec.Llm.Openai.App == "":
		return trace.BadParameter("cluster beam config spec.llm.openai.app is required")
	case cfg.Spec.Llm.Anthropic == nil || cfg.Spec.Llm.Anthropic.App == "":
		return trace.BadParameter("cluster beam config spec.llm.anthropic.app is required")
	}
	return nil
}
