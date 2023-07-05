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
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"
)

var log = logrus.WithField(trace.Component, "reissuer")

func TestReissueCert(t *testing.T) {
	t.Parallel()
	resolvableErr := trace.Errorf("ssh: cert has expired")
	unresolvableErr := trace.AccessDenied("")
	concurrentCallErr := trace.AlreadyExists("")
	reloginTimeoutErr := status.Error(codes.DeadlineExceeded, "foo")
	unknownErr := status.Error(codes.Unknown, "foo")
	tests := []struct {
		name             string
		reissueErrs      []error
		reloginErr       error
		reissuerOpt      func(t *testing.T, reissuer *GatewayCertReissuer)
		wantReissueCalls int
		wantReloginCalls int
		wantNotifyCalls  int
		wantErr          error
		wantAddedMessage string
	}{
		{
			name:             "calls DB cert reissuer once if it returns successfully",
			reissueErrs:      []error{nil},
			wantReissueCalls: 1,
		},
		{
			name:             "calls DB cert reissuer once if it returns error unresolvable with relogin",
			reissueErrs:      []error{unresolvableErr},
			wantReissueCalls: 1,
			wantReloginCalls: 0,
			wantNotifyCalls:  1,
			wantErr:          unresolvableErr,
		},
		{
			name:             "resolves error with relogin and calls DB cert reissuer twice",
			reissueErrs:      []error{resolvableErr},
			wantReissueCalls: 2,
			wantReloginCalls: 1,
			wantNotifyCalls:  0,
		},
		{
			name:        "does not allow concurrent relogin calls",
			reissueErrs: []error{concurrentCallErr},
			reissuerOpt: func(t *testing.T, reissuer *GatewayCertReissuer) {
				t.Helper()
				require.True(t, reissuer.reloginMu.TryLock(), "Couldn't lock reloginMu")
			},
			wantReissueCalls: 1,
			wantReloginCalls: 0,
			wantNotifyCalls:  1,
			wantErr:          concurrentCallErr,
		},
		{
			name:             "adds additional message to error on timeout during relogin",
			reissueErrs:      []error{resolvableErr},
			reloginErr:       reloginTimeoutErr,
			wantReissueCalls: 1,
			wantReloginCalls: 1,
			wantNotifyCalls:  1,
			wantErr:          reloginTimeoutErr,
			wantAddedMessage: "the user did not refresh the session within",
		},
		{
			name:             "adds additional message to error on unexpected error during relogin",
			reissueErrs:      []error{resolvableErr},
			reloginErr:       unknownErr,
			wantReissueCalls: 1,
			wantReloginCalls: 1,
			wantNotifyCalls:  1,
			wantErr:          unknownErr,
			wantAddedMessage: "could not refresh the session",
		},
		{
			name:             "sends notification if second call to reissue certs fails",
			reissueErrs:      []error{resolvableErr, unresolvableErr},
			wantReissueCalls: 2,
			wantReloginCalls: 1,
			wantNotifyCalls:  1,
			wantErr:          unresolvableErr,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gateway := mustCreateGateway(ctx, t)
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tshdEventsClient := &mockTSHDEventsClient{
				callCounts: make(map[string]int),
				reloginErr: tt.reloginErr,
			}
			reissuer := &GatewayCertReissuer{
				Log: log,
			}
			dbCertReissuer := &mockDBCertReissuer{
				returnValuesForSubsequentCalls: tt.reissueErrs,
			}
			if tt.reissuerOpt != nil {
				tt.reissuerOpt(t, reissuer)
			}
			err := reissuer.ReissueCert(ctx, gateway, dbCertReissuer, tshdEventsClient)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.ErrorContains(t, err, tt.wantAddedMessage)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantReissueCalls, dbCertReissuer.callCount,
				"Unexpected number of calls to DBCertReissuer.ReissueDBCerts")
			require.Equal(t, tt.wantReloginCalls, tshdEventsClient.callCounts["Relogin"],
				"Unexpected number of calls to TSHDEventsClient.Relogin")
			require.Equal(t, tt.wantNotifyCalls, tshdEventsClient.callCounts["SendNotification"],
				"Unexpected number of calls to TSHDEventsClient.SendNotification")
		})
	}
}

func mustCreateGateway(ctx context.Context, t *testing.T) *gateway.Gateway {
	t.Helper()

	gatewayCreator := &mockGatewayCreator{t: t}
	gateway, err := gatewayCreator.CreateGateway(ctx, clusters.CreateGatewayParams{
		TargetURI:          uri.NewClusterURI("foo").AppendDB("postgres").String(),
		TargetUser:         "alice",
		CLICommandProvider: &mockCLICommandProvider{},
	})
	require.NoError(t, err)
	return gateway
}

type mockDBCertReissuer struct {
	callCount                      int
	returnValuesForSubsequentCalls []error
}

func (r *mockDBCertReissuer) ReissueDBCerts(context.Context, tlsca.RouteToDatabase) error {
	var err error
	if r.callCount < len(r.returnValuesForSubsequentCalls) {
		err = r.returnValuesForSubsequentCalls[r.callCount]
	}

	r.callCount++

	return err
}

type mockCLICommandProvider struct{}

func (m mockCLICommandProvider) GetCommand(gateway *gateway.Gateway) (*exec.Cmd, error) {
	// Use a relative path to make exec.Command avoid unnecessarily calling exec.LookPath.
	path := filepath.Join("foo", gateway.Protocol())
	arg := fmt.Sprintf("%s/%s", gateway.TargetName(), gateway.TargetSubresourceName())
	cmd := exec.Command(path, arg)
	return cmd, nil
}

type mockTSHDEventsClient struct {
	callCounts map[string]int
	reloginErr error
}

func (c *mockTSHDEventsClient) Relogin(context.Context, *api.ReloginRequest, ...grpc.CallOption) (*api.ReloginResponse, error) {
	c.callCounts["Relogin"]++

	if c.reloginErr != nil {
		return nil, c.reloginErr
	}

	return &api.ReloginResponse{}, nil
}

func (c *mockTSHDEventsClient) SendNotification(context.Context, *api.SendNotificationRequest, ...grpc.CallOption) (*api.SendNotificationResponse, error) {
	c.callCounts["SendNotification"]++
	return &api.SendNotificationResponse{}, nil
}

func (c *mockTSHDEventsClient) HeadlessAuthentication(context.Context, *api.HeadlessAuthenticationRequest, ...grpc.CallOption) (*api.HeadlessAuthenticationResponse, error) {
	c.callCounts["HeadlessAuthentication"]++
	return &api.HeadlessAuthenticationResponse{}, nil
}
