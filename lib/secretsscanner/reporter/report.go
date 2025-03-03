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

package reporter

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/gravitational/trace"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dtassert "github.com/gravitational/teleport/lib/devicetrust/assert"
	secretsscannerclient "github.com/gravitational/teleport/lib/secretsscanner/client"
)

// AssertCeremonyBuilderFunc is a function that builds the device authentication ceremony.
type AssertCeremonyBuilderFunc func() (*dtassert.Ceremony, error)

// Config specifies the configuration for the reporter.
type Config struct {
	// Client is a client for the SecretsScannerService.
	Client secretsscannerclient.Client
	// Log is the logger.
	Log *slog.Logger
	// BatchSize is the number of secrets to send in a single batch. Defaults to [defaultBatchSize] if not set.
	BatchSize int
	// AssertCeremonyBuilder is the device authentication ceremony builder.
	// If not set, the default device authentication ceremony will be used.
	// Used for testing, avoid in production code.
	AssertCeremonyBuilder AssertCeremonyBuilderFunc
}

// Reporter reports secrets to the Teleport Proxy.
type Reporter struct {
	client                secretsscannerclient.Client
	log                   *slog.Logger
	batchSize             int
	assertCeremonyBuilder AssertCeremonyBuilderFunc
}

// New creates a new reporter instance.
func New(cfg Config) (*Reporter, error) {
	if cfg.Client == nil {
		return nil, trace.BadParameter("missing client")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.BatchSize == 0 {
		const defaultBatchSize = 100
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.AssertCeremonyBuilder == nil {
		cfg.AssertCeremonyBuilder = func() (*dtassert.Ceremony, error) {
			return dtassert.NewCeremony()
		}
	}
	return &Reporter{
		client:                cfg.Client,
		log:                   cfg.Log,
		batchSize:             cfg.BatchSize,
		assertCeremonyBuilder: cfg.AssertCeremonyBuilder,
	}, nil
}

// ReportPrivateKeys reports the private keys to the Teleport server.
// This function performs the following steps:
// 1. Create a new gRPC client to the Teleport Proxy.
// 2. Run the device assertion ceremony.
// 3. Report the private keys to the Teleport cluster.
// 4. Wait for the server to acknowledge the report.
func (r *Reporter) ReportPrivateKeys(ctx context.Context, pks []*accessgraphsecretsv1pb.PrivateKey) error {

	stream, err := r.client.ReportSecrets(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to create client")
	}

	if err := r.runAssertionCeremony(ctx, stream); err != nil {
		return trace.Wrap(err, "failed to run assertion ceremony")
	}

	if err := r.reportPrivateKeys(stream, pks); err != nil {
		return trace.Wrap(err, "failed to report private keys")
	}

	return trace.Wrap(r.terminateAndWaitAcknowledge(stream), "server failed to acknowledge the report")
}

// runAssertionCeremony runs the device assertion ceremony.
func (r *Reporter) runAssertionCeremony(ctx context.Context, stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient) error {
	// Create a new device authentication ceremony.
	assertCeremony, err := r.assertCeremonyBuilder()
	if err != nil {
		return trace.Wrap(err, "failed to create assertCeremony")
	}

	// Run the device authentication ceremony.
	// If successful, the device will be authenticated and the device can report its secrets.
	err = assertCeremony.Run(
		ctx,
		reportToAssertStreamAdapter{stream},
	)
	return trace.Wrap(err, "failed to run device authentication ceremony")
}

// reportPrivateKeys reports the private keys to the Teleport server in batches of size [r.batchSize] using the given stream.
func (r *Reporter) reportPrivateKeys(stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient, privateKeys []*accessgraphsecretsv1pb.PrivateKey) error {
	batchSize := r.batchSize
	for i := 0; len(privateKeys) > i; i += batchSize {
		start := i
		end := i + batchSize
		if end > len(privateKeys) {
			end = len(privateKeys)
		}
		if err := stream.Send(&accessgraphsecretsv1pb.ReportSecretsRequest{
			Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_PrivateKeys{
				PrivateKeys: &accessgraphsecretsv1pb.ReportPrivateKeys{
					Keys: privateKeys[start:end],
				},
			},
		}); err != nil && !errors.Is(err, io.EOF) {
			// [io.EOF] indicates that the server has closed the stream.
			// The client should handle the underlying error on the subsequent Recv call.
			// All other errors are client-side errors and should be returned.
			return trace.Wrap(err, "failed to send private keys")
		}
	}
	return nil
}

// terminateAndWaitAcknowledge terminates the client side of the stream and waits for the server to acknowledge the report.
func (r *Reporter) terminateAndWaitAcknowledge(stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient) error {
	// Inform the server that there are no more private keys to report.
	if err := stream.CloseSend(); err != nil {
		return trace.Wrap(err, "failed to close send")
	}

	// Wait for the server to acknowledge the report.
	if _, err := stream.Recv(); err != nil && !errors.Is(err, io.EOF) {
		return trace.Wrap(err, "error closing the stream")
	}
	return nil
}

// reportToAssertStreamAdapter is a wrapper for the [accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient] that implements the
// [assert.AssertDeviceClientStream] interface.
//
// This adapter allows the [accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient] to be used with the [assert.AssertDeviceClientStream]
// interface, which is essential for the [assert.Ceremony] in executing the device authentication process. It handles the extraction and insertion
// of device assertion messages from and into the [accessgraphsecretsv1pb.ReportSecretsRequest] and [accessgraphsecretsv1pb.ReportSecretsResponse] messages.
type reportToAssertStreamAdapter struct {
	stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsClient
}

func (s reportToAssertStreamAdapter) Send(request *devicepb.AssertDeviceRequest) error {
	return trace.Wrap(
		s.stream.Send(
			&accessgraphsecretsv1pb.ReportSecretsRequest{
				Payload: &accessgraphsecretsv1pb.ReportSecretsRequest_DeviceAssertion{
					DeviceAssertion: request,
				},
			},
		),
	)
}

func (s reportToAssertStreamAdapter) Recv() (*devicepb.AssertDeviceResponse, error) {
	in, err := s.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if in.GetDeviceAssertion() == nil {
		return nil, trace.BadParameter("unsupported response type: expected DeviceAssertion, got %T", in.Payload)
	}

	return in.GetDeviceAssertion(), nil
}
