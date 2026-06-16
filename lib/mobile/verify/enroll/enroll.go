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
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
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

// CreateMobileEnrollToken calls CreateMobileDeviceEnrollToken on the public
// Device Trust service. Populated fields of deviceData are forwarded as-is.
func (c *Client) CreateMobileEnrollToken(pairingToken string, deviceData *DeviceCollectedData) (*DeviceEnrollToken, error) {
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

	// TODO(ravicious): Replace this with a call to the public gRPC service.
	client := devicepb.NewDeviceTrustServiceClient(grpcConn)
	resp, err := client.CreateDeviceEnrollToken(ctx,
		&devicepb.CreateDeviceEnrollTokenRequest{
			DeviceData: toPBDeviceData(deviceData),
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DeviceEnrollToken{
		Token: resp.GetToken(),
	}, nil
}

// toPBDeviceData translates DeviceCollectedData into the proto type.
// OsType stays UNSPECIFIED until OS_TYPE_IOS lands in
// teleport.devicetrust.v1.OSType (see RFD 32e).
func toPBDeviceData(d *DeviceCollectedData) *devicepb.DeviceCollectedData {
	if d == nil {
		d = &DeviceCollectedData{}
	}
	return &devicepb.DeviceCollectedData{
		CollectTime:        timestamppb.Now(),
		SerialNumber:       d.SerialNumber,
		ModelIdentifier:    d.ModelIdentifier,
		OsVersion:          d.VersionOS,
		OsBuild:            d.BuildOS,
		SystemSerialNumber: d.SystemSerialNumber,
	}
}
