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

package enroll

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"log/slog"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
	"github.com/gravitational/teleport/lib/devicetrust/challenge"
)

type Client struct {
	proxyServer string
	insecure    bool
}

// NewClient returns a Client configured to dial proxyServer.
// insecure must be false in release builds of the iOS app; it is intended
// only for development against local clusters.
func NewClient(proxyServer string, insecure bool) *Client {
	return &Client{
		proxyServer: proxyServer,
		insecure:    insecure,
	}
}

// DeviceCollectedData mirrors the iOS-relevant subset of
// teleport.devicetrust.v1.DeviceCollectedData. The wrapper is required
// because gomobile only exports types from the bound package.
//
// Fields don't start with an acronym to dodge a gomobile bug
// (https://github.com/golang/go/issues/32008): VersionOS, not OSVersion.
type DeviceCollectedData struct {
	SerialNumber       string
	ModelIdentifier    string
	VersionOS          string
	BuildOS            string
	SystemSerialNumber string
}

// DeviceEnrollToken mirrors teleport.devicetrust.v1.DeviceEnrollToken.
// ExpireTime is dropped because gomobile doesn't support
// timestamppb.Timestamp.
type DeviceEnrollToken struct {
	Token string
}

// EnrolledDevice is the iOS-relevant subset of the enrolled
// teleport.devicetrust.v1.Device returned by EnrollDevice.
type EnrolledDevice struct {
	DeviceID string
	AssetTag string
}

// CreatePairedDeviceEnrollToken calls CreatePairedDeviceEnrollToken on the
// public Device Trust service. Populated fields of deviceData are forwarded
// as-is.
func (c *Client) CreatePairedDeviceEnrollToken(pairingToken string, deviceData *DeviceCollectedData) (*DeviceEnrollToken, error) {
	// TODO(ravicious): Integrate Go's context with Swift's task cancellation
	// See https://github.com/gravitational/teleport/pull/61278/changes/8980e91f611264bd760890316d49a842ded2aebb
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcConn, err := proxyinsecureclient.NewConnection(
		ctx,
		proxyinsecureclient.ConnectionConfig{
			ProxyServer: c.proxyServer,
			Clock:       clockwork.NewRealClock(),
			Insecure:    c.insecure,
			Log:         slog.Default(),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer grpcConn.Close()

	client := devicetrustpublicv1pb.NewDeviceTrustServiceClient(grpcConn)
	resp, err := client.CreatePairedDeviceEnrollToken(ctx,
		devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenRequest_builder{
			EnrollPairingToken: pairingToken,
			DeviceData:         toPBDeviceData(deviceData),
		}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DeviceEnrollToken{
		Token: resp.GetDeviceEnrollToken().GetToken(),
	}, nil
}

// EnrollDevice runs the enrollment ceremony against the public Device Trust
// service using the enrollment token from CreatePairedDeviceEnrollToken.
//
// This is a simplified stand-in for the real iOS ceremony: instead of a Secure
// Enclave key it generates an ephemeral P-256 key in-process, signs the
// server's challenge with it, and returns the enrolled device.
func (c *Client) EnrollDevice(enrollToken string, deviceData *DeviceCollectedData) (*EnrolledDevice, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	grpcConn, err := proxyinsecureclient.NewConnection(
		ctx,
		proxyinsecureclient.ConnectionConfig{
			ProxyServer: c.proxyServer,
			Clock:       clockwork.NewRealClock(),
			Insecure:    c.insecure,
			Log:         slog.Default(),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer grpcConn.Close()

	client := devicetrustpublicv1pb.NewDeviceTrustServiceClient(grpcConn)
	stream, err := client.EnrollDevice(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 1. Init.
	if err := stream.Send(devicetrustpublicv1pb.EnrollDeviceRequest_builder{
		Init: devicetrustpublicv1pb.EnrollDeviceInit_builder{
			Token:        enrollToken,
			CredentialId: uuid.NewString(),
			DeviceData:   toPBDeviceData(deviceData),
			Ios: devicetrustpublicv1pb.IOSEnrollPayload_builder{
				PublicKeyDer: pubDER,
			}.Build(),
		}.Build(),
	}.Build()); err != nil {
		return nil, trace.Wrap(err)
	}

	// 2. Challenge.
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chal := resp.GetIosChallenge().GetChallenge()
	if len(chal) == 0 {
		return nil, trace.BadParameter("expected iOS enroll challenge")
	}

	// 3. Challenge response.
	sig, err := challenge.Sign(chal, priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := stream.Send(devicetrustpublicv1pb.EnrollDeviceRequest_builder{
		IosChallengeResponse: devicetrustpublicv1pb.IOSEnrollChallengeResponse_builder{
			Signature: sig,
		}.Build(),
	}.Build()); err != nil {
		return nil, trace.Wrap(err)
	}

	// 4. Success.
	resp, err = stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dev := resp.GetSuccess().GetDevice()
	if dev == nil {
		return nil, trace.BadParameter("expected enroll device success")
	}
	return &EnrolledDevice{
		DeviceID: dev.GetId(),
		AssetTag: dev.GetAssetTag(),
	}, nil
}

// toPBDeviceData translates DeviceCollectedData into the proto type. OsType is
// hardcoded to iOS.
func toPBDeviceData(d *DeviceCollectedData) *devicepb.DeviceCollectedData {
	if d == nil {
		d = &DeviceCollectedData{}
	}
	return devicepb.DeviceCollectedData_builder{
		CollectTime:        timestamppb.Now(),
		OsType:             devicepb.OSType_OS_TYPE_IOS,
		SerialNumber:       d.SerialNumber,
		ModelIdentifier:    d.ModelIdentifier,
		OsVersion:          d.VersionOS,
		OsBuild:            d.BuildOS,
		SystemSerialNumber: d.SystemSerialNumber,
	}.Build()
}
