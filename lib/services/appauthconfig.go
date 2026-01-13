/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"crypto/sha256"
	"encoding/hex"

	"github.com/gravitational/trace"

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	"github.com/gravitational/teleport/api/types"
)

// AppAuthConfigReader defines configs for reading app auth config resources.
type AppAuthConfigReader interface {
	// GetAppAuthConfig returns the specified AppAuthConfig.
	GetAppAuthConfig(ctx context.Context, name string) (*appauthconfigv1.AppAuthConfig, error)
	// ListAppAuthConfigs lists AppAuthConfig resources.
	ListAppAuthConfigs(ctx context.Context, limit int, startKey string) ([]*appauthconfigv1.AppAuthConfig, string, error)
}

// AppAuthConfig is a service that manages [appauthconfigv1.AppAuthConfig]
// resources.
type AppAuthConfig interface {
	AppAuthConfigReader

	// CreateAppAuthConfig creates a new AppAuthConfig.
	CreateAppAuthConfig(ctx context.Context, in *appauthconfigv1.AppAuthConfig) (*appauthconfigv1.AppAuthConfig, error)
	// UpdateAppAuthConfig updates an existing AppAuthConfig.
	UpdateAppAuthConfig(ctx context.Context, in *appauthconfigv1.AppAuthConfig) (*appauthconfigv1.AppAuthConfig, error)
	// UpsertAppAuthConfig creates or replaces a AppAuthConfig.
	UpsertAppAuthConfig(ctx context.Context, in *appauthconfigv1.AppAuthConfig) (*appauthconfigv1.AppAuthConfig, error)
	// DeleteAppAuthConfig deletes the specified AppAuthConfig.
	DeleteAppAuthConfig(ctx context.Context, name string) error
}

// AppAuthConfigSessions is a service that manages sessions using app auth
// config.
type AppAuthConfigSessions interface {
	// CreateAppSessionWithJWT creates an app session using JWT token.
	CreateAppSessionWithJWT(ctx context.Context, req *appauthconfigv1.CreateAppSessionWithJWTRequest) (types.WebSession, error)
}

// ValidateAppAuthConfig validates the given app auth config.
func ValidateAppAuthConfig(s *appauthconfigv1.AppAuthConfig) error {
	switch {
	case s == nil:
		return trace.BadParameter("app auth config must not be empty")
	case s.Version != types.V1:
		return trace.BadParameter("app auth config only supports version %q, got %q", types.V1, s.Version)
	case s.Kind != types.KindAppAuthConfig:
		return trace.BadParameter("app auth config kind must be %q, got %q", types.KindAppAuthConfig, s.Kind)
	case s.Metadata == nil:
		return trace.BadParameter("app auth config metadata is missing")
	case s.Metadata.Name == "":
		return trace.BadParameter("app auth config metadata.name is missing")
	case s.Spec == nil:
		return trace.BadParameter("app auth config spec is missing")
	case len(s.Spec.AppLabels) == 0:
		return trace.BadParameter("app auth config spec.app_labels must at least contain a single label, otherwise this config won't be effective")
	}

	for _, label := range s.Spec.AppLabels {
		if err := validateLabel(label); err != nil {
			return trace.BadParameter("invalid app auth config spec.app_labels: %v", err)
		}
	}

	switch spec := s.Spec.SubKindSpec.(type) {
	case *appauthconfigv1.AppAuthConfigSpec_Jwt:
		return validateJWTAppAuthConfig(spec.Jwt)
	default:
		return trace.BadParameter("unsupported app auth config type")
	}
}

func validateJWTAppAuthConfig(s *appauthconfigv1.AppAuthConfigJWTSpec) error {
	switch {
	case s == nil:
		return trace.BadParameter("app auth config spec.jwt is required")
	case s.Audience == "":
		return trace.BadParameter("app auth config spec.jwt.audience cannot be empty")
	case s.Issuer == "":
		return trace.BadParameter("app auth config spec.jwt.issuer cannot be empty")
	case s.GetJwksUrl() == "" && s.GetStaticJwks() == "":
		return trace.BadParameter("app auth config spec.jwt.jwks_url or spec.jwt.static_jwks must be provided")
	}

	return nil
}

// GenerateAppSessionIDFromJWT generates a app session id based on JWT token.
func GenerateAppSessionIDFromJWT(jwtToken string) string {
	jwtHash := sha256.Sum256([]byte(jwtToken))
	return hex.EncodeToString(jwtHash[:])
}
