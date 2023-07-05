// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"context"
	"time"

	"google.golang.org/grpc"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

const (
	// tshdEventsTimeout is the maximum amount of time the gRPC client managed by the tshd daemon will
	// wait for a response from the tshd events server managed by the Electron app. This timeout
	// should be used for quick one-off calls where the client doesn't need the server or the user to
	// perform any additional work, such as the SendNotification RPC.
	tshdEventsTimeout = time.Second
)

// TSHDEventsClient takes only those methods from api.TshdEventsServiceClient that
// GatewayCertReissuer actually needs. It makes mocking the client in tests easier and future-proof.
//
// Refer to [api.TshdEventsServiceClient] for a more detailed documentation.
type TSHDEventsClient interface {
	// Relogin makes the Electron app display a login modal. Please refer to
	// [api.TshdEventsServiceClient.Relogin] for more details.
	Relogin(ctx context.Context, in *api.ReloginRequest, opts ...grpc.CallOption) (*api.ReloginResponse, error)
	// SendNotification causes the Electron app to display a notification. Please refer to
	// [api.TshdEventsServiceClient.SendNotification] for more details.
	SendNotification(ctx context.Context, in *api.SendNotificationRequest, opts ...grpc.CallOption) (*api.SendNotificationResponse, error)
	// HeadlessAuthentication
	HeadlessAuthentication(ctx context.Context, in *api.HeadlessAuthenticationRequest, opts ...grpc.CallOption) (*api.HeadlessAuthenticationResponse, error)
}
