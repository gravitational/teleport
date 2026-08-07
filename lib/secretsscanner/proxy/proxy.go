/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

// Package proxy implements a proxy service that proxies requests from the proxy unauthenticated
// gRPC service to the Auth's secret service.
package proxy

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	grpcutils "github.com/gravitational/teleport/lib/utils/grpc"
)

// AuthClient is a subset of the full Auth API that must be connected
type AuthClient interface {
	AccessGraphSecretsScannerClient() accessgraphsecretsv1pb.SecretsScannerServiceClient
}

// ServiceConfig is the configuration for the Service.
type ServiceConfig struct {
	// AuthClient is the client to the Auth service.
	AuthClient AuthClient
	// Log is the logger.
	Log *slog.Logger
}

// New creates a new Service.
func New(cfg ServiceConfig) (*Service, error) {
	if cfg.AuthClient == nil {
		return nil, trace.BadParameter("missing AuthClient")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &Service{
		authClient: cfg.AuthClient,
		log:        cfg.Log,
	}, nil
}

// Service is a service that proxies requests from the proxy to the Auth's secret service.
// It only implements the ReportSecrets method of the SecretsScannerService because it is the only method that needs to be proxied
// from the proxy to the Auth's secret service.
type Service struct {
	accessgraphsecretsv1pb.UnimplementedSecretsScannerServiceServer
	// authClient is the client to the Auth service.
	authClient AuthClient

	log *slog.Logger
}

// ReportSecrets proxies the ReportSecrets method from the proxy to the Auth's secret service.
func (s *Service) ReportSecrets(client accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer) error {
	err := grpcutils.ProxyBidiStream(s.log, client, func(ctx context.Context) (accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient, error) {
		server, err := s.authClient.AccessGraphSecretsScannerClient().ReportSecrets(ctx)
		return server, trace.Wrap(err)
	})
	return trace.Wrap(err)
}
