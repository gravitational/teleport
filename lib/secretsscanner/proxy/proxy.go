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
	"errors"
	"io"
	"log/slog"

	"github.com/gravitational/trace"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
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
	ctx, cancel := context.WithCancel(client.Context())
	defer cancel()
	upstream, err := s.authClient.AccessGraphSecretsScannerClient().ReportSecrets(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	errCh := make(chan error, 1)
	go func() {
		err := trace.Wrap(s.forwardClientToServer(ctx, client, upstream))
		if err != nil {
			cancel()
		}
		errCh <- err
	}()

	err = s.forwardServerToClient(ctx, client, upstream)
	return trace.NewAggregate(err, <-errCh)
}

func (s *Service) forwardClientToServer(ctx context.Context,
	client accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer,
	server accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient) (err error) {
	for {
		req, err := client.Recv()
		if errors.Is(err, io.EOF) {
			if err := server.CloseSend(); err != nil {
				s.log.WarnContext(ctx, "Failed to close upstream stream", "error", err)
			}
			break
		}
		if err != nil {
			s.log.WarnContext(ctx, "Failed to receive from client stream", "error", err)
			return trace.Wrap(err)
		}
		if err := server.Send(req); err != nil {
			s.log.WarnContext(ctx, "Failed to send to upstream stream", "error", err)
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *Service) forwardServerToClient(ctx context.Context,
	client accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer,
	server accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient) (err error) {
	for {
		out, err := server.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			s.log.WarnContext(ctx, "Failed to receive from upstream stream", "error", err)
			return trace.Wrap(err)
		}
		if err := client.Send(out); err != nil {
			s.log.WarnContext(ctx, "Failed to send to client stream", "error", err)
			return trace.Wrap(err)
		}
	}
}
