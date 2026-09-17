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

	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
)

// DeviceEnrollToken mirrors teleport.devicetrust.v1.DeviceEnrollToken.
// ExpireTime is dropped because gomobile doesn't support
// timestamppb.Timestamp.
type DeviceEnrollToken struct {
	Token string
}

// CreatePairedDeviceEnrollToken calls CreatePairedDeviceEnrollToken on the public
// Device Trust service. Populated fields of deviceData are forwarded as-is.
func (c *Client) CreatePairedDeviceEnrollToken(pairingToken string, deviceData *DeviceCollectedData) (*DeviceEnrollToken, error) {
	// TODO(ravicious): Integrate Go's context with Swift's task cancellation
	// See https://github.com/gravitational/teleport/pull/61278/changes/8980e91f611264bd760890316d49a842ded2aebb
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pbDeviceData, err := toPBDeviceData(deviceData)
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
			// CreatePairedDeviceEnrollToken blocks until the enrollment is
			// approved in the Web UI, so keep the upgraded connection alive to
			// survive L7 load balancer idle timeouts.
			ALPNConnUpgradePing: true,
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
			DeviceData:         pbDeviceData,
		}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DeviceEnrollToken{
		Token: resp.GetDeviceEnrollToken().GetToken(),
	}, nil
}
