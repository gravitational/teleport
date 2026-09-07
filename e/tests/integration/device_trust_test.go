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

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/defaults"
	publicdevicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	dterrors "github.com/gravitational/teleport/e/lib/devicetrust/errors"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/lib/devicetrust/testenv"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

// TestPublicEnrollFlow exercises mobile enrollment end to end, as it plays out
// between the mobile app and the Web UI over both public RPCs:
//
//   - the Web UI, acting for the signed-in user, starts the enrollment and
//     obtains a pairing token (shown to the app as a QR code),
//   - the mobile app claims the pairing over the unauthenticated public gRPC
//     endpoint and blocks waiting for approval,
//   - the user approves the request in the Web UI,
//   - the app receives its device enrollment token,
//   - the app enrolls the device with that token, answering the enrollment
//     challenge with the key in its Secure Enclave.
//
// The authenticated private Device Trust service stands in for the Web UI
// backend (CreateEnrollPairing / ApproveEnrollPairing) and the public service
// for the app. See RFD 32e.
func TestPublicEnrollFlow(t *testing.T) {
	t.Parallel()
	sut := common.InitSUT(t)
	ctx := t.Context()
	role := createMobileEnrollRole(t, ctx, sut, "mobile-enroll")
	admin := newDeviceAdmin(t, sut)

	// The app talks to the public endpoints over the Proxy's public gRPC server,
	// with no user identity of its own.
	app := publicdevicepb.NewDeviceTrustServiceClient(dialProxyPublicGRPC(t, sut))
	webUI, dev, _ := setUpUserAndFakeDevice(t, ctx, sut, admin, "alice", role.GetName())

	token := pairAndCreateDeviceEnrollToken(t, ctx, webUI, app, dev.CollectDeviceData())

	resp, err := enrollDevice(ctx, app, dev.EnrollDeviceInit(token), dev.SignChallenge)
	require.NoError(t, err)
	enrolled := resp.GetSuccess().GetDevice()
	require.NotNil(t, enrolled, "expected EnrollDeviceSuccess, got %v", resp)
	assert.Equal(t, devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED, enrolled.GetEnrollStatus())
	assert.Equal(t, "alice", enrolled.GetOwner())
	assert.Equal(t, dev.ID, enrolled.GetCredential().GetId())

	// The inventory, read back as the admin, agrees with what the app was told.
	got, err := admin.GetDevice(ctx, devicepb.GetDeviceRequest_builder{
		DeviceId: enrolled.GetId(),
	}.Build())
	require.NoError(t, err)
	assert.Equal(t, devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED, got.GetEnrollStatus())
	assert.Equal(t, dev.ID, got.GetCredential().GetId())

	// The enrollment spent the token so it cannot enroll the device again.
	_, err = enrollDevice(ctx, app, dev.EnrollDeviceInit(token), dev.SignChallenge)
	require.ErrorIs(t, trail.FromGRPC(err), dterrors.ErrInvalidDeviceEnrollToken)
}

// TestCreatePairedDeviceEnrollToken_errors covers the ways the pairing can end
// without an enrollment token.
func TestCreatePairedDeviceEnrollToken_errors(t *testing.T) {
	t.Parallel()
	sut := common.InitSUT(t)
	ctx := t.Context()
	role := createMobileEnrollRole(t, ctx, sut, "mobile-enroll")
	admin := newDeviceAdmin(t, sut)
	app := publicdevicepb.NewDeviceTrustServiceClient(dialProxyPublicGRPC(t, sut))

	t.Run("user denies, app is rejected", func(t *testing.T) {
		t.Parallel()
		webUI, dev, _ := setUpUserAndFakeDevice(t, ctx, sut, admin, "bob", role.GetName())
		token := startEnrollment(t, ctx, webUI)

		tokenResCh := scanQRCodeAndCreateDeviceEnrollToken(ctx, app, token, dev.CollectDeviceData())
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
		webUI, dev, _ := setUpUserAndFakeDevice(t, ctx, sut, admin, "carol", role.GetName())
		token := startEnrollment(t, ctx, webUI)

		// The real device claims the pairing and waits for approval.
		tokenResCh := scanQRCodeAndCreateDeviceEnrollToken(ctx, app, token, dev.CollectDeviceData())
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

// TestEnrollDevice_errors covers the ways enrollment over the public endpoint
// is rejected, and what each rejection leaves behind: whether the enrollment
// token survives it and whether the device ends up enrolled.
func TestEnrollDevice_errors(t *testing.T) {
	t.Parallel()
	sut := common.InitSUT(t)
	ctx := t.Context()
	role := createMobileEnrollRole(t, ctx, sut, "mobile-enroll")
	admin := newDeviceAdmin(t, sut)
	app := publicdevicepb.NewDeviceTrustServiceClient(dialProxyPublicGRPC(t, sut))

	t.Run("rejects a token the pairing flow did not issue", func(t *testing.T) {
		t.Parallel()
		_, dev, _ := setUpUserAndFakeDevice(t, ctx, sut, admin, "bob", role.GetName())

		_, err := enrollDevice(ctx, app, dev.EnrollDeviceInit("not-a-token"), dev.SignChallenge)
		require.ErrorIs(t, trail.FromGRPC(err), dterrors.ErrInvalidDeviceEnrollToken)
	})

	t.Run("rejects a locked user and keeps the token", func(t *testing.T) {
		t.Parallel()
		webUI, dev, _ := setUpUserAndFakeDevice(t, ctx, sut, admin, "carol", role.GetName())
		token := pairAndCreateDeviceEnrollToken(t, ctx, webUI, app, dev.CollectDeviceData())

		// RFD 32e requires the public path to turn away locked users. The unit
		// tests inject a fakeAuthorizer, so this is the only layer where the real
		// Authorizer runs and consults its lock watcher.
		lock, err := types.NewLock("carol", types.LockSpecV2{
			Target: types.LockTarget{User: "carol"},
		})
		require.NoError(t, err)
		authServer := sut.Teleport.Process.GetAuthServer()
		require.NoError(t, authServer.UpsertLock(ctx, lock))
		waitForUserAuthz(t, ctx, webUI, trace.IsAccessDenied)

		// The Authorizer reports a lock as a generic denial, so that message is all
		// there is to assert on.
		_, err = enrollDevice(ctx, app, dev.EnrollDeviceInit(token), dev.SignChallenge)
		require.ErrorAs(t, trail.FromGRPC(err), new(*trace.AccessDeniedError))
		assert.ErrorContains(t, trail.FromGRPC(err), "access denied")

		// The rejection came before the ceremony, so the token is still stored and
		// enrolls the device once the lock is lifted.
		require.NoError(t, authServer.DeleteLock(ctx, lock.GetName()))
		waitForUserAuthz(t, ctx, webUI, func(err error) bool { return !trace.IsAccessDenied(err) })
		resp, err := enrollDevice(ctx, app, dev.EnrollDeviceInit(token), dev.SignChallenge)
		require.NoError(t, err)
		assert.NotNil(t, resp.GetSuccess())
	})

	t.Run("rejects a user whose permission was revoked and keeps the token", func(t *testing.T) {
		t.Parallel()
		// This subtest edits its user's role, so it gets a role of its own.
		revocable := createMobileEnrollRole(t, ctx, sut, "mobile-enroll-revocable")
		webUI, dev, _ := setUpUserAndFakeDevice(t, ctx, sut, admin, "dave", revocable.GetName())
		token := pairAndCreateDeviceEnrollToken(t, ctx, webUI, app, dev.CollectDeviceData())

		authServer := sut.Teleport.Process.GetAuthServer()
		granted := revocable.GetRules(types.Allow)
		revocable.SetRules(types.Allow, nil)
		revocable, err := authServer.UpsertRole(ctx, revocable)
		require.NoError(t, err)
		waitForUserAuthz(t, ctx, webUI, trace.IsAccessDenied)

		_, err = enrollDevice(ctx, app, dev.EnrollDeviceInit(token), dev.SignChallenge)
		require.ErrorAs(t, trail.FromGRPC(err), new(*trace.AccessDeniedError))
		assert.ErrorContains(t, trail.FromGRPC(err), "access denied")

		// The rejection came before the ceremony, so the token is still stored and
		// enrolls the device once the permission is back.
		revocable.SetRules(types.Allow, granted)
		_, err = authServer.UpsertRole(ctx, revocable)
		require.NoError(t, err)
		waitForUserAuthz(t, ctx, webUI, func(err error) bool { return !trace.IsAccessDenied(err) })
		resp, err := enrollDevice(ctx, app, dev.EnrollDeviceInit(token), dev.SignChallenge)
		require.NoError(t, err)
		assert.NotNil(t, resp.GetSuccess())
	})

	t.Run("a failed ceremony spends the token but doesn't enroll the device", func(t *testing.T) {
		t.Parallel()
		webUI, dev, _ := setUpUserAndFakeDevice(t, ctx, sut, admin, "erin", role.GetName())
		token := pairAndCreateDeviceEnrollToken(t, ctx, webUI, app, dev.CollectDeviceData())

		// A bad signature is only caught after the challenge round trip, so this
		// also covers the proxy relaying an error raised once messages have flowed
		// both ways.
		badSign := func([]byte) ([]byte, error) { return []byte("not a signature"), nil }
		_, err := enrollDevice(ctx, app, dev.EnrollDeviceInit(token), badSign)
		require.ErrorAs(t, trail.FromGRPC(err), new(*trace.BadParameterError))
		assert.ErrorContains(t, trail.FromGRPC(err), "signature verification failed")

		// The ceremony spends the token before the challenge, so the failed
		// attempt consumed it.
		_, err = enrollDevice(ctx, app, dev.EnrollDeviceInit(token), dev.SignChallenge)
		require.ErrorIs(t, trail.FromGRPC(err), dterrors.ErrInvalidDeviceEnrollToken)

		// The device is not enrolled: a fresh token and a correct signature enroll
		// it all the way.
		token = pairAndCreateDeviceEnrollToken(t, ctx, webUI, app, dev.CollectDeviceData())
		resp, err := enrollDevice(ctx, app, dev.EnrollDeviceInit(token), dev.SignChallenge)
		require.NoError(t, err)
		assert.NotNil(t, resp.GetSuccess())
	})

	t.Run("rejects an admin-issued token", func(t *testing.T) {
		t.Parallel()
		_, dev, registered := setUpUserAndFakeDevice(t, ctx, sut, admin, "frank", role.GetName())

		// The admin path mints tokens by device ID and binds no user to them, so
		// the public path has nobody to authorize and treats them like any bad
		// token.
		adminToken, err := admin.CreateDeviceEnrollToken(ctx, devicepb.CreateDeviceEnrollTokenRequest_builder{
			DeviceId: registered.GetId(),
		}.Build())
		require.NoError(t, err)

		_, err = enrollDevice(ctx, app, dev.EnrollDeviceInit(adminToken.GetToken()), dev.SignChallenge)
		require.ErrorIs(t, trail.FromGRPC(err), dterrors.ErrInvalidDeviceEnrollToken)
	})

	t.Run("rejects a desktop OS type", func(t *testing.T) {
		t.Parallel()
		// No user or inventory record is set up: the OS type gate runs before the
		// token lookup.
		dev, err := testenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
		require.NoError(t, err)
		init := dev.EnrollDeviceInit("irrelevant")
		init.GetDeviceData().SetOsType(devicepb.OSType_OS_TYPE_MACOS)

		_, err = enrollDevice(ctx, app, init, dev.SignChallenge)
		require.ErrorAs(t, trail.FromGRPC(err), new(*trace.BadParameterError))
		assert.ErrorContains(t, trail.FromGRPC(err), "unsupported OS type")
	})

	t.Run("returns NotFound when the token user no longer exists", func(t *testing.T) {
		t.Parallel()
		webUI, dev, _ := setUpUserAndFakeDevice(t, ctx, sut, admin, "henry", role.GetName())
		token := pairAndCreateDeviceEnrollToken(t, ctx, webUI, app, dev.CollectDeviceData())

		authServer := sut.Teleport.Process.GetAuthServer()
		require.NoError(t, authServer.DeleteUser(ctx, "henry"))
		// The handler reads users through the auth cache, which trails the
		// deletion. Probing with EnrollDevice too early would enroll instead.
		require.Eventually(t, func() bool {
			_, err := authServer.Cache.GetUser(ctx, "henry", false)
			return trace.IsNotFound(err)
		}, 15*time.Second, 200*time.Millisecond)

		_, err := enrollDevice(ctx, app, dev.EnrollDeviceInit(token), dev.SignChallenge)
		require.ErrorAs(t, trail.FromGRPC(err), new(*trace.NotFoundError))
		assert.ErrorContains(t, trail.FromGRPC(err), "user not found")
	})
}

// createMobileEnrollRole creates a role for the enrolling users under the given
// name. It grants the mobile enrollment verb and nothing else, which is all a
// real enrolling user holds: registering the device, reading it back and
// minting admin-issued tokens are the device admin's job.
func createMobileEnrollRole(t *testing.T, ctx context.Context, sut *common.SUT, name string) types.Role {
	t.Helper()
	role, err := types.NewRole(name, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindMobileDevice, []string{types.VerbCreateEnrollToken}),
			},
		},
	})
	require.NoError(t, err)
	_, err = sut.Teleport.Process.GetAuthServer().CreateRole(ctx, role)
	require.NoError(t, err)
	return role
}

// newDeviceAdmin creates the user who registers devices and mints admin-issued
// tokens, a stand-in for the admin or MDM sync that owns those steps in
// production, and returns a private Device Trust client acting as them.
func newDeviceAdmin(t *testing.T, sut *common.SUT) devicepb.DeviceTrustServiceClient {
	t.Helper()
	common.MustCreateUser(t, sut, "device-admin", teleport.PresetDeviceAdminRoleName)
	return devicepb.NewDeviceTrustServiceClient(sut.GetAuthServiceGRPCConn(t, "device-admin"))
}

// setUpUserAndFakeDevice creates a user with the mobile enrollment role, has
// admin register the device it will enroll and returns an authenticated private
// Device Trust client acting as the Web UI backend, along with the fake device
// and the inventory record it was registered as. The fake produces the
// collected data the app presents and holds the Secure Enclave key the
// enrollment ceremony challenges.
func setUpUserAndFakeDevice(t *testing.T, ctx context.Context, sut *common.SUT, admin devicepb.DeviceTrustServiceClient, user, role string) (devicepb.DeviceTrustServiceClient, *testenv.FakeIOSDevice, *devicepb.Device) {
	t.Helper()
	common.MustCreateUser(t, sut, user, role)
	webUI := devicepb.NewDeviceTrustServiceClient(sut.GetAuthServiceGRPCConn(t, user))

	dev, err := testenv.NewFakeIOSDevice(devicepb.OSType_OS_TYPE_IOS)
	require.NoError(t, err)
	registered := registerDevice(t, ctx, admin, dev.CollectDeviceData())
	return webUI, dev, registered
}

// registerDevice adds the device described by cd to the inventory as admin.
func registerDevice(t *testing.T, ctx context.Context, admin devicepb.DeviceTrustServiceClient, cd *devicepb.DeviceCollectedData) *devicepb.Device {
	t.Helper()
	dev, err := admin.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
		Device: devicepb.Device_builder{
			OsType:   cd.GetOsType(),
			AssetTag: cd.GetSerialNumber(),
		}.Build(),
	}.Build())
	require.NoError(t, err)
	return dev
}

// waitForUserAuthz polls GetCurrentEnrollPairing as the Web UI user until its
// authorization outcome satisfies want. That RPC reruns the same
// mobile_device.create_enroll_token check as EnrollDevice, so its outcome
// tracks the handler's, and it doesn't spend any tokens.
func waitForUserAuthz(t *testing.T, ctx context.Context, webUI devicepb.DeviceTrustServiceClient, want func(err error) bool) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := webUI.GetCurrentEnrollPairing(ctx, &devicepb.GetCurrentEnrollPairingRequest{})
		return want(trail.FromGRPC(err))
	}, 15*time.Second, 200*time.Millisecond, "authorization outcome for the Web UI user did not change")
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

// pairAndCreateDeviceEnrollToken runs the whole pairing flow for the device
// described by cd and returns the enrollment token it ends with: the Web UI
// starts the enrollment, the app claims the pairing, the user approves it.
//
// It has no t.Helper() on purpose: in TestPublicEnrollFlow the pairing is the
// subject under test, not setup, so a failure should point at a line in here.
func pairAndCreateDeviceEnrollToken(t *testing.T, ctx context.Context, webUI devicepb.DeviceTrustServiceClient, app publicdevicepb.DeviceTrustServiceClient, cd *devicepb.DeviceCollectedData) string {
	pairingToken := startEnrollment(t, ctx, webUI)
	tokenResCh := scanQRCodeAndCreateDeviceEnrollToken(ctx, app, pairingToken, cd)
	waitForPairingState(t, ctx, webUI, devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)

	_, err := webUI.ApproveEnrollPairing(ctx, devicepb.ApproveEnrollPairingRequest_builder{
		PairingToken: pairingToken,
	}.Build())
	require.NoError(t, err)

	res := waitForResult(t, tokenResCh)
	require.NoError(t, res.err)
	token := res.resp.GetDeviceEnrollToken().GetToken()
	require.NotEmpty(t, token, "app should receive a device enrollment token")
	return token
}

// enrollDevice runs the app's side of the enrollment ceremony: init, challenge,
// signed response. sign is called with the server challenge and its result
// rides in the IOSEnrollChallengeResponse.
//
// Stream errors are returned as they arrive. They're not trace.Wrap'd on
// purpose so that trail.FromGRPC (used later by the callers) does not mess up
// the error text.
func enrollDevice(ctx context.Context, app publicdevicepb.DeviceTrustServiceClient, init *publicdevicepb.EnrollDeviceInit, sign func(chal []byte) ([]byte, error)) (*publicdevicepb.EnrollDeviceResponse, error) {
	stream, err := app.EnrollDevice(ctx)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(publicdevicepb.EnrollDeviceRequest_builder{
		Init: init,
	}.Build()); err != nil {
		return nil, err
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	chal := resp.GetIosChallenge()
	if chal == nil {
		return resp, trace.BadParameter("expected IOSEnrollChallenge, got %v", resp)
	}

	sig, err := sign(chal.GetChallenge())
	if err != nil {
		return nil, err
	}
	if err := stream.Send(publicdevicepb.EnrollDeviceRequest_builder{
		IosChallengeResponse: publicdevicepb.IOSEnrollChallengeResponse_builder{
			Signature: sig,
		}.Build(),
	}.Build()); err != nil {
		return nil, err
	}

	resp, err = stream.Recv()
	return resp, err
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
