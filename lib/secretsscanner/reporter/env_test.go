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

package reporter_test

import (
	"errors"
	"io"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dttestenv "github.com/gravitational/teleport/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/fixtures"
)

type env struct {
	secretsScannerAddr string
	service            *serviceFake
}

type opts struct {
	device            *device
	preAssertError    error
	preReconcileError error
}

type device struct {
	device dttestenv.FakeDevice
	id     string
}

type option func(*opts)

func withDevice(deviceID string, dev dttestenv.FakeDevice) option {
	return func(o *opts) {
		o.device = &device{
			device: dev,
			id:     deviceID,
		}
	}
}

func withPreReconcileError(err error) option {
	return func(o *opts) {
		o.preReconcileError = err
	}
}

func withPreAssertError(err error) option {
	return func(o *opts) {
		o.preAssertError = err
	}
}

func setup(t *testing.T, ops ...option) env {
	t.Helper()

	o := opts{}
	for _, op := range ops {
		op(&o)
	}

	var opts []dttestenv.Opt
	if o.device != nil {
		dev, pubKey, err := dttestenv.CreateEnrolledDevice(o.device.id, o.device.device)
		require.NoError(t, err)
		opts = append(opts, dttestenv.WithPreEnrolledDevice(dev, pubKey))
	}
	dtFakeSvc, err := dttestenv.New(opts...)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := dtFakeSvc.Close()
		assert.NoError(t, err)
	})

	svc := newServiceFake(dtFakeSvc.Service)
	svc.preReconcileError = o.preReconcileError
	svc.preAssertError = o.preAssertError

	tlsConfig, err := fixtures.LocalTLSConfig()
	require.NoError(t, err)

	grpcServer := grpc.NewServer(
		grpc.Creds(
			credentials.NewTLS(tlsConfig.TLS),
		),
	)
	accessgraphsecretsv1pb.RegisterSecretsScannerServiceServer(grpcServer, svc)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		err := grpcServer.Serve(lis)
		assert.NoError(t, err)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	return env{
		service:            svc,
		secretsScannerAddr: lis.Addr().String(),
	}
}

func newServiceFake(deviceTrustSvc *dttestenv.FakeDeviceService) *serviceFake {
	return &serviceFake{
		deviceTrustSvc: deviceTrustSvc,
	}
}

type serviceFake struct {
	accessgraphsecretsv1pb.UnimplementedSecretsScannerServiceServer
	privateKeysReported []*accessgraphsecretsv1pb.PrivateKey
	deviceTrustSvc      *dttestenv.FakeDeviceService
	preReconcileError   error
	preAssertError      error
}

func (s *serviceFake) ReportSecrets(in accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer) error {
	if s.preAssertError != nil {
		return s.preAssertError
	}
	// Step 1. Assert the device.
	if _, err := s.deviceTrustSvc.AssertDevice(in.Context(), streamAdapter{stream: in}); err != nil {
		return trace.Wrap(err)
	}
	// Step 2. Collect the private keys into a temporary slice.
	var collectedKeys []*accessgraphsecretsv1pb.PrivateKey
	for {
		msg, err := in.Recv()
		// Step 4. When the client closes his side of the stream, we break the loop
		// and collect the private keys.
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return trace.Wrap(err)
		}

		if msg.GetPrivateKeys() == nil {
			return trace.BadParameter("unexpected assert request payload: %T", msg.GetPayload())
		}
		// Step 3. Collect the private keys into a temporary slice.
		collectedKeys = append(collectedKeys, msg.GetPrivateKeys().GetKeys()...)

	}

	if s.preReconcileError != nil {
		return s.preReconcileError
	}

	// Step 5. Store the collected private keys.
	// This only happens when the client closes his side of the stream.
	s.privateKeysReported = collectedKeys
	return nil
}

// streamAdapter is a helper struct that adapts the [accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer]
// stream to the device trust assertion stream [assertserver.AssertDeviceServerStream].
// This is needed because we need to extract the [*devicepb.AssertDeviceRequest] from the stream
// and return the [*devicepb.AssertDeviceResponse] to the stream.
type streamAdapter struct {
	stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer
}

func (s streamAdapter) Send(rsp *devicepb.AssertDeviceResponse) error {
	msg := &accessgraphsecretsv1pb.ReportSecretsResponse{
		Payload: &accessgraphsecretsv1pb.ReportSecretsResponse_DeviceAssertion{
			DeviceAssertion: rsp,
		},
	}
	err := s.stream.Send(msg)
	return trace.Wrap(err)
}

func (s streamAdapter) Recv() (*devicepb.AssertDeviceRequest, error) {
	msg, err := s.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if msg.GetDeviceAssertion() == nil {
		return nil, trace.BadParameter("unexpected assert request payload: %T", msg.GetPayload())
	}

	return msg.GetDeviceAssertion(), nil
}
