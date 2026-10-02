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
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/grpc"

	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicetrustv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	proxyinsecureclient "github.com/gravitational/teleport/lib/client/proxy/insecure"
)

type AuthenticateDeviceOperation struct {
	cancel      context.CancelFunc
	conn        *grpc.ClientConn
	stream      devicetrustpublicv1pb.DeviceTrustService_AuthenticateDeviceClient
	initRequest *devicetrustpublicv1pb.AuthenticateDeviceRequest
	closeOnce   sync.Once
	closeErr    error
}

type DeviceConfirmationToken struct {
	Identifier string
	Token      string
}

func (c *Client) NewAuthenticateDeviceOperation(
	webTokenIdentifier string,
	webToken string,
	deviceData *DeviceCollectedData,
	credentialIdentifier string,
) (*AuthenticateDeviceOperation, error) {
	pbDeviceData, err := toPBDeviceData(deviceData)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	conn, err := proxyinsecureclient.NewConnection(
		ctx,
		proxyinsecureclient.ConnectionConfig{
			ProxyServer:         c.proxyServer,
			Clock:               clockwork.NewRealClock(),
			Insecure:            c.insecure,
			Log:                 slog.Default(),
			ALPNConnUpgradePing: true,
		},
	)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}

	stream, err := devicetrustpublicv1pb.NewDeviceTrustServiceClient(conn).AuthenticateDevice(ctx)
	if err != nil {
		cancel()
		_ = conn.Close()
		return nil, trace.Wrap(err)
	}

	initRequest := devicetrustpublicv1pb.AuthenticateDeviceRequest_builder{
		Init: devicetrustpublicv1pb.AuthenticateDeviceInit_builder{
			CredentialId: credentialIdentifier,
			DeviceData:   pbDeviceData,
			DeviceWebToken: devicetrustv1.DeviceWebToken_builder{
				Id:    webTokenIdentifier,
				Token: webToken,
			}.Build(),
		}.Build(),
	}.Build()

	return &AuthenticateDeviceOperation{
		cancel:      cancel,
		conn:        conn,
		stream:      stream,
		initRequest: initRequest,
	}, nil
}

func (o *AuthenticateDeviceOperation) Initialize() ([]byte, error) {
	if err := o.stream.Send(o.initRequest); err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := o.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challenge := response.GetChallenge()
	if challenge == nil {
		return nil, trace.BadParameter("expected AuthenticateDeviceChallenge, got %v", response.WhichPayload())
	}

	return challenge.GetChallenge(), nil
}

func (o *AuthenticateDeviceOperation) SendChallengeResponse(signature []byte) (*DeviceConfirmationToken, error) {
	err := o.stream.Send(
		devicetrustpublicv1pb.AuthenticateDeviceRequest_builder{
			ChallengeResponse: devicetrustpublicv1pb.AuthenticateDeviceChallengeResponse_builder{
				Signature: signature,
			}.Build(),
		}.Build(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := o.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	confirmationToken := response.GetConfirmationToken()
	if confirmationToken == nil {
		return nil, trace.BadParameter("expected DeviceConfirmationToken, got %v", response.WhichPayload())
	}

	return &DeviceConfirmationToken{
		Identifier: confirmationToken.GetId(),
		Token:      confirmationToken.GetToken(),
	}, nil
}

func (o *AuthenticateDeviceOperation) Close() error {
	o.closeOnce.Do(func() {
		o.cancel()
		o.closeErr = o.conn.Close()
	})
	return trace.Wrap(o.closeErr)
}
