package integration

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/defaults"
	publicdevicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/common"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

// TestCreatePairedDeviceEnrollToken exercises the paired mobile enrollment flow
// end to end, as it plays out between the mobile app and the Web UI:
//
//   - the Web UI, acting for the signed-in user, starts the enrollment and
//     obtains a pairing token (shown to the app as a QR code),
//   - the mobile app claims the pairing over the unauthenticated public gRPC
//     endpoint and blocks waiting for approval,
//   - the user approves the request in the Web UI,
//   - the app receives its device enrollment token.
//
// The authenticated private Device Trust service stands in for the Web UI
// backend (CreateEnrollPairing / ApproveEnrollPairing / DenyEnrollPairing) and
// the public service for the app. See RFD 32e.
func TestCreatePairedDeviceEnrollToken(t *testing.T) {
	t.Parallel()
	sut := common.InitSUT(t)
	ctx := t.Context()

	// Device Trust is enabled by the self-hosted license the SUT boots with.
	// The role grants the mobile enrollment verb and, only for test setup, the
	// verb to register a device in the inventory (in production an admin or MDM
	// sync owns that step, not the enrolling user).
	role, err := types.NewRole("mobile-enroll", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindMobileDevice, []string{types.VerbCreateEnrollToken}),
				types.NewRule(types.KindDevice, []string{types.VerbCreate}),
			},
		},
	})
	require.NoError(t, err)
	_, err = sut.Teleport.Process.GetAuthServer().CreateRole(ctx, role)
	require.NoError(t, err)

	// The app talks to the public endpoint over the Proxy's public gRPC server,
	// with no user identity of its own.
	app := publicdevicepb.NewDeviceTrustServiceClient(dialProxyPublicGRPC(t, sut))

	t.Run("user approves, app gets an enrollment token", func(t *testing.T) {
		t.Parallel()
		webUI, cd := setUpUserAndDevice(t, ctx, sut, "alice", role.GetName())
		token := startEnrollment(t, ctx, webUI)

		tokenResCh := scanQRCodeAndCreateDeviceEnrollToken(ctx, app, token, cd)
		waitForPairingState(t, ctx, webUI, devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)

		_, err := webUI.ApproveEnrollPairing(ctx, devicepb.ApproveEnrollPairingRequest_builder{
			PairingToken: token,
		}.Build())
		require.NoError(t, err)

		res := waitForResult(t, tokenResCh)
		require.NoError(t, res.err)
		assert.NotEmpty(t, res.resp.GetDeviceEnrollToken().GetToken(),
			"app should receive a device enrollment token")
	})

	t.Run("user denies, app is rejected", func(t *testing.T) {
		t.Parallel()
		webUI, cd := setUpUserAndDevice(t, ctx, sut, "bob", role.GetName())
		token := startEnrollment(t, ctx, webUI)

		tokenResCh := scanQRCodeAndCreateDeviceEnrollToken(ctx, app, token, cd)
		waitForPairingState(t, ctx, webUI, devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)

		_, err := webUI.DenyEnrollPairing(ctx, devicepb.DenyEnrollPairingRequest_builder{
			PairingToken: token,
		}.Build())
		require.NoError(t, err)

		res := waitForResult(t, tokenResCh)
		resErr := trail.FromGRPC(res.err)
		require.ErrorAs(t, resErr, new(*trace.AccessDeniedError))
		assert.ErrorContains(t, resErr, "denied or has expired")
	})

	t.Run("unknown pairing token is rejected", func(t *testing.T) {
		t.Parallel()
		// No pairing exists for this token, so the call fails immediately rather
		// than blocking, and the underlying lookup error is obscured.
		_, err := app.CreatePairedDeviceEnrollToken(ctx, publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{
			EnrollPairingToken: "does-not-exist",
			DeviceData:         collectedData("ghost"),
		}.Build())
		require.ErrorIs(t, trail.FromGRPC(err), &trace.AccessDeniedError{Message: "invalid enroll pairing token"})
	})

	t.Run("a different device cannot claim a pending pairing", func(t *testing.T) {
		t.Parallel()
		webUI, cd := setUpUserAndDevice(t, ctx, sut, "carol", role.GetName())
		token := startEnrollment(t, ctx, webUI)

		// The real device claims the pairing and waits for approval.
		tokenResCh := scanQRCodeAndCreateDeviceEnrollToken(ctx, app, token, cd)
		waitForPairingState(t, ctx, webUI, devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)

		// A second device presenting the same token but a different serial is
		// turned away without disturbing the pending claim.
		_, err := app.CreatePairedDeviceEnrollToken(ctx, publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{
			EnrollPairingToken: token,
			DeviceData:         collectedData("carol-attacker"),
		}.Build())
		require.ErrorAs(t, trail.FromGRPC(err), new(*trace.AccessDeniedError))
		assert.ErrorContains(t, trail.FromGRPC(err), "claimed by another device")

		// The original device still completes once the user approves.
		_, err = webUI.ApproveEnrollPairing(ctx, devicepb.ApproveEnrollPairingRequest_builder{
			PairingToken: token,
		}.Build())
		require.NoError(t, err)

		res := waitForResult(t, tokenResCh)
		require.NoError(t, res.err)
		assert.NotEmpty(t, res.resp.GetDeviceEnrollToken().GetToken())
	})
}

// setUpUserAndDevice creates a user with the mobile enrollment role, registers
// the device it will enroll (a stand-in for the admin/MDM inventory step) and
// returns an authenticated private Device Trust client acting as the Web UI
// backend, along with the collected data the app will present.
func setUpUserAndDevice(t *testing.T, ctx context.Context, sut *common.SUT, user, role string) (devicepb.DeviceTrustServiceClient, *devicepb.DeviceCollectedData) {
	t.Helper()
	common.MustCreateUser(t, sut, user, role)
	webUI := devicepb.NewDeviceTrustServiceClient(sut.GetAuthServiceGRPCConn(t, user))

	cd := collectedData(user + "-device")
	_, err := webUI.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
		Device: devicepb.Device_builder{
			OsType:   cd.GetOsType(),
			AssetTag: cd.GetSerialNumber(),
		}.Build(),
	}.Build())
	require.NoError(t, err)
	return webUI, cd
}

// startEnrollment is the Web UI starting mobile enrollment for the signed-in
// user, returning the pairing token the app scans.
func startEnrollment(t *testing.T, ctx context.Context, webUI devicepb.DeviceTrustServiceClient) string {
	t.Helper()
	resp, err := webUI.CreateEnrollPairing(ctx, &devicepb.CreateEnrollPairingRequest{})
	require.NoError(t, err)
	token := resp.GetEnrollPairing().GetStatus().GetToken()
	require.NotEmpty(t, token)
	return token
}

// tokenResult is the outcome of the app's CreatePairedDeviceEnrollToken call.
type tokenResult struct {
	resp *publicdevicepb.CreatePairedDeviceEnrollTokenResponse
	err  error
}

// scanQRCodeAndCreateDeviceEnrollToken runs the mobile app's call to create an
// enrollment token for a device. The RPC blocks until the enrollment request is
// approved, denied or expires, so the test drives the pairing through the Web
// UI client while the call is in flight.
func scanQRCodeAndCreateDeviceEnrollToken(ctx context.Context, app publicdevicepb.DeviceTrustServiceClient, token string, cd *devicepb.DeviceCollectedData) <-chan tokenResult {
	ch := make(chan tokenResult, 1)
	go func() {
		resp, err := app.CreatePairedDeviceEnrollToken(ctx, publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{
			EnrollPairingToken: token,
			DeviceData:         cd,
		}.Build())
		ch <- tokenResult{resp: resp, err: err}
	}()
	return ch
}

// waitForPairingState waits for the signed-in user's pairing to reach state,
// which is how the test knows the backgrounded app call got far enough to act
// on before it drives the next step.
func waitForPairingState(t *testing.T, ctx context.Context, webUI devicepb.DeviceTrustServiceClient, state devicepb.EnrollPairingState) {
	t.Helper()
	require.Eventually(t, func() bool {
		resp, err := webUI.GetCurrentEnrollPairing(ctx, &devicepb.GetCurrentEnrollPairingRequest{})
		return err == nil && resp.GetEnrollPairing().GetStatus().GetState() == state
	}, 15*time.Second, 200*time.Millisecond, "pairing did not reach state %v", state)
}

func waitForResult(t *testing.T, ch <-chan tokenResult) tokenResult {
	t.Helper()
	select {
	case res := <-ch:
		return res
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for CreatePairedDeviceEnrollToken")
		return tokenResult{}
	}
}

func collectedData(serial string) *devicepb.DeviceCollectedData {
	return devicepb.DeviceCollectedData_builder{
		OsType:       devicepb.OSType_OS_TYPE_IOS,
		SerialNumber: serial,
		OsVersion:    "26.3.1",
		CollectTime:  timestamppb.Now(),
	}.Build()
}

// dialProxyPublicGRPC dials the Proxy Service's public gRPC server, over which
// the unauthenticated public Device Trust service is exposed.
func dialProxyPublicGRPC(t *testing.T, sut *common.SUT) *grpc.ClientConn {
	t.Helper()
	dialer := apiclient.NewDialer(
		t.Context(),
		defaults.DefaultIdleTimeout,
		defaults.DefaultIOTimeout,
	)
	tlsConfig := utils.TLSConfig(nil)
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.NextProtos = []string{string(alpncommon.ProtocolProxyGRPCInsecure)}
	conn, err := grpc.NewClient(
		sut.Teleport.ReverseTunnel,
		grpc.WithContextDialer(apiclient.GRPCContextDialer(dialer)),
		grpc.WithUnaryInterceptor(metadata.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(metadata.StreamClientInterceptor),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, conn.Close()) })
	return conn
}
