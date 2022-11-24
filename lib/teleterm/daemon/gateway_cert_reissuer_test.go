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
	"testing"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/tlsca"
)

var log = logrus.WithField(trace.Component, "reissuer")

func TestReissueCert_CallsDBCertReissuerOnceIfItReturnsSuccessfully(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tshdEventsClient := &mockTSHDEventsClient{
		callCounts: make(map[string]int),
	}
	reissuer := &GatewayCertReissuer{
		Log:              log,
		TSHDEventsClient: tshdEventsClient,
	}
	gateway := mustCreateGateway(ctx, t)
	dbCertReissuer := &mockDBCertReissuer{}

	err := reissuer.ReissueCert(ctx, gateway, dbCertReissuer)
	require.NoError(t, err)

	require.Equal(t, 1, dbCertReissuer.callCount,
		"Expected DBCertReissuer to have been called exactly one time")
	require.Equal(t, 0, tshdEventsClient.callCounts["Relogin"],
		"Expected TSHDEventsClient.Relogin to have been called exactly zero times")
	require.Equal(t, 0, tshdEventsClient.callCounts["SendNotification"],
		"Expected TSHDEventsClient.SendNotification to have been called exactly zero times")
}

func TestReissueCert_CallsDBCertReissuerOnceIfItReturnsErrorUnresolvableWithRelogin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tshdEventsClient := &mockTSHDEventsClient{
		callCounts: make(map[string]int),
	}
	reissuer := &GatewayCertReissuer{
		Log:              log,
		TSHDEventsClient: tshdEventsClient,
	}
	gateway := mustCreateGateway(ctx, t)
	unresolvableErr := trace.AccessDenied("")
	dbCertReissuer := &mockDBCertReissuer{
		returnValuesForSubsequentCalls: []error{unresolvableErr},
	}

	err := reissuer.ReissueCert(ctx, gateway, dbCertReissuer)
	require.ErrorIs(t, err, unresolvableErr)

	require.Equal(t, 1, dbCertReissuer.callCount,
		"Expected DBCertReissuer to have been called exactly one time")
	require.Equal(t, 0, tshdEventsClient.callCounts["Relogin"],
		"Expected TSHDEventsClient.Relogin to have been called exactly zero times")
	require.Equal(t, 1, tshdEventsClient.callCounts["SendNotification"],
		"Expected TSHDEventsClient.SendNotification to have been called exactly one time")
}

func TestReissueCert_ResolvesErrorWithReloginAndCallsDBCertReissuerTwice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tshdEventsClient := &mockTSHDEventsClient{
		callCounts: make(map[string]int),
	}
	reissuer := &GatewayCertReissuer{
		Log:              log,
		TSHDEventsClient: tshdEventsClient,
	}
	gateway := mustCreateGateway(ctx, t)
	resolvableErr := trace.Errorf("ssh: cert has expired")
	dbCertReissuer := &mockDBCertReissuer{
		returnValuesForSubsequentCalls: []error{resolvableErr},
	}

	err := reissuer.ReissueCert(ctx, gateway, dbCertReissuer)
	require.NoError(t, err)

	// Ideally, we'd also verify that the cert was actually reissued and set on the LocalProxy
	// underneath. However, with how the API is structured, we have no way of doing this here.
	// Instead, we defer that check to integration tests.

	require.Equal(t, 2, dbCertReissuer.callCount,
		"Expected DBCertReissuer to have been called exactly two times")
	require.Equal(t, 1, tshdEventsClient.callCounts["Relogin"],
		"Expected TSHDEventsClient.Relogin to have been called exactly one time")
	require.Equal(t, 0, tshdEventsClient.callCounts["SendNotification"],
		"Expected TSHDEventsClient.SendNotification to have been called exactly zero times")
}

func TestReissueCert_DoesntAllowConcurrentReloginCalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tshdEventsClient := &mockTSHDEventsClient{
		callCounts: make(map[string]int),
	}
	reissuer := &GatewayCertReissuer{
		Log:              log,
		TSHDEventsClient: tshdEventsClient,
	}
	gateway := mustCreateGateway(ctx, t)
	resolvableErr := trace.Errorf("ssh: cert has expired")
	dbCertReissuer := &mockDBCertReissuer{
		returnValuesForSubsequentCalls: []error{resolvableErr},
	}

	if !reissuer.reloginMu.TryLock() {
		t.Fatalf("Couldn't lock reloginMu")
	}
	defer reissuer.reloginMu.Unlock()

	err := reissuer.ReissueCert(ctx, gateway, dbCertReissuer)
	require.True(t, trace.IsAlreadyExists(err), "Expected err to be trace.AlreadyExistsError")

	require.Equal(t, 1, dbCertReissuer.callCount,
		"Expected DBCertReissuer to have been called exactly one time")
	require.Equal(t, 0, tshdEventsClient.callCounts["Relogin"],
		"Expected TSHDEventsClient.Relogin to have been called exactly zero times")
	require.Equal(t, 1, tshdEventsClient.callCounts["SendNotification"],
		"Expected TSHDEventsClient.SendNotification to have been called exactly one time")
}

func TestReissueCert_AddsAdditionalMessageToErrorOnTimeoutDuringRelogin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tshdEventsClient := &mockTSHDEventsClient{
		callCounts: make(map[string]int),
		reloginErr: status.Error(codes.DeadlineExceeded, "foo"),
	}
	reissuer := &GatewayCertReissuer{
		Log:              log,
		TSHDEventsClient: tshdEventsClient,
	}
	gateway := mustCreateGateway(ctx, t)
	resolvableErr := trace.Errorf("ssh: cert has expired")
	dbCertReissuer := &mockDBCertReissuer{
		returnValuesForSubsequentCalls: []error{resolvableErr},
	}

	err := reissuer.ReissueCert(ctx, gateway, dbCertReissuer)
	require.ErrorContains(t, err, "the user did not refresh the session within")
	require.ErrorIs(t, err, tshdEventsClient.reloginErr)

	require.Equal(t, 1, dbCertReissuer.callCount,
		"Expected DBCertReissuer to have been called exactly one time")
	require.Equal(t, 1, tshdEventsClient.callCounts["Relogin"],
		"Expected TSHDEventsClient.Relogin to have been called exactly one time")
	require.Equal(t, 1, tshdEventsClient.callCounts["SendNotification"],
		"Expected TSHDEventsClient.SendNotification to have been called exactly one time")
}

func TestReissueCert_AddsAdditionalMessageToUnexpectedErrorDuringRelogin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tshdEventsClient := &mockTSHDEventsClient{
		callCounts: make(map[string]int),
		reloginErr: status.Error(codes.Unknown, "foo"),
	}
	reissuer := &GatewayCertReissuer{
		Log:              log,
		TSHDEventsClient: tshdEventsClient,
	}
	gateway := mustCreateGateway(ctx, t)
	resolvableErr := trace.Errorf("ssh: cert has expired")
	dbCertReissuer := &mockDBCertReissuer{
		returnValuesForSubsequentCalls: []error{resolvableErr},
	}

	err := reissuer.ReissueCert(ctx, gateway, dbCertReissuer)
	require.ErrorContains(t, err, "could not refresh the session")
	require.ErrorIs(t, err, tshdEventsClient.reloginErr)

	require.Equal(t, 1, dbCertReissuer.callCount,
		"Expected DBCertReissuer to have been called exactly one time")
	require.Equal(t, 1, tshdEventsClient.callCounts["Relogin"],
		"Expected TSHDEventsClient.Relogin to have been called exactly one time")
	require.Equal(t, 1, tshdEventsClient.callCounts["SendNotification"],
		"Expected TSHDEventsClient.SendNotification to have been called exactly one time")
}

func TestReissueCert_SendsNotificationIfSecondCallToReissueCertsFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tshdEventsClient := &mockTSHDEventsClient{
		callCounts: make(map[string]int),
	}
	reissuer := &GatewayCertReissuer{
		Log:              log,
		TSHDEventsClient: tshdEventsClient,
	}
	gateway := mustCreateGateway(ctx, t)
	resolvableErr := trace.Errorf("ssh: cert has expired")
	unresolvableErr := trace.AccessDenied("")
	dbCertReissuer := &mockDBCertReissuer{
		returnValuesForSubsequentCalls: []error{resolvableErr, unresolvableErr},
	}

	err := reissuer.ReissueCert(ctx, gateway, dbCertReissuer)
	require.ErrorIs(t, err, unresolvableErr)

	require.Equal(t, 2, dbCertReissuer.callCount,
		"Expected DBCertReissuer to have been called exactly two times")
	require.Equal(t, 1, tshdEventsClient.callCounts["Relogin"],
		"Expected TSHDEventsClient.Relogin to have been called exactly one time")
	require.Equal(t, 1, tshdEventsClient.callCounts["SendNotification"],
		"Expected TSHDEventsClient.SendNotification to have been called exactly one time")
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

func (m mockCLICommandProvider) GetCommand(gateway *gateway.Gateway) (string, error) {
	command := fmt.Sprintf("%s/%s", gateway.TargetName(), gateway.TargetSubresourceName())
	return command, nil
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
