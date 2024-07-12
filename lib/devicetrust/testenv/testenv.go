/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package testenv

import (
	"context"
	"crypto/ecdsa"
	"net"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/devicetrust/native"
)

// Opt is a creation option for [E]
type Opt func(*E)

// WithAutoCreateDevice instructs EnrollDevice to automatically create the
// requested device, if it wasn't previously registered.
// See also [FakeEnrollmentToken].
func WithAutoCreateDevice(b bool) Opt {
	return func(e *E) {
		e.Service.autoCreateDevice = b
	}
}

// WithPreEnrolledDevice registers a device with the service without having to enroll it.
// This is useful for testing device authentication flows.
// [pub] is the public key of the macOS device and is used to verify the device. TPM devices
// do not require a public key and should pass nil.
func WithPreEnrolledDevice(dev *devicepb.Device, pub *ecdsa.PublicKey) Opt {
	return func(e *E) {
		e.Service.mu.Lock()
		defer e.Service.mu.Unlock()
		e.Service.devices = append(e.Service.devices,
			storedDevice{
				pb:          dev,
				enrollToken: FakeEnrollmentToken,
				pub:         pub,
			},
		)
	}

}

// E is an integrated test environment for device trust.
type E struct {
	DevicesClient devicepb.DeviceTrustServiceClient
	Service       *FakeDeviceService

	closers []func() error
}

// Close tears down the test environment.
func (e *E) Close() error {
	var errs []error
	for i := len(e.closers) - 1; i >= 0; i-- {
		if err := e.closers[i](); err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

// MustNew creates a new E or panics.
// Callers are required to defer e.Close() to release test resources.
func MustNew(opts ...Opt) *E {
	env, err := New(opts...)
	if err != nil {
		panic(err)
	}
	return env
}

// New creates a new E.
// Callers are required to defer e.Close() to release test resources.
func New(opts ...Opt) (*E, error) {
	e := &E{
		Service: newFakeDeviceService(),
	}

	for _, opt := range opts {
		opt(e)
	}

	ok := false
	defer func() {
		if !ok {
			e.Close()
		}
	}()

	// gRPC Server.
	const bufSize = 100 // arbitrary
	lis := bufconn.Listen(bufSize)
	e.closers = append(e.closers, lis.Close)

	s := grpc.NewServer(
		// Options below are similar to auth.GRPCServer.
		grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
	)
	e.closers = append(e.closers, func() error {
		s.GracefulStop()
		s.Stop()
		return nil
	})

	// Register service.
	devicepb.RegisterDeviceTrustServiceServer(s, e.Service)

	// Start.
	go func() {
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	// gRPC client.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, "unused",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
		grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
	)
	if err != nil {
		return nil, err
	}
	e.closers = append(e.closers, cc.Close)
	e.DevicesClient = devicepb.NewDeviceTrustServiceClient(cc)

	ok = true
	return e, nil
}

// FakeDevice is implemented by the platform-native fakes and is used in tests
// for device authentication and enrollment.
type FakeDevice interface {
	CollectDeviceData(mode native.CollectDataMode) (*devicepb.DeviceCollectedData, error)
	EnrollDeviceInit() (*devicepb.EnrollDeviceInit, error)
	GetDeviceOSType() devicepb.OSType
	SignChallenge(chal []byte) (sig []byte, err error)
	SolveTPMEnrollChallenge(challenge *devicepb.TPMEnrollChallenge, debug bool) (*devicepb.TPMEnrollChallengeResponse, error)
	SolveTPMAuthnDeviceChallenge(challenge *devicepb.TPMAuthenticateDeviceChallenge) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error)
	GetDeviceCredential() *devicepb.DeviceCredential
}
