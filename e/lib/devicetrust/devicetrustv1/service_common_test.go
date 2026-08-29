package devicetrustv1_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
)

func createAndEnroll(
	ctx context.Context,
	devices devicepb.DeviceTrustServiceClient, dev *devicepb.Device) (*devicepb.Device, *fakeEnclaveKey, error) {
	dev, err := devices.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
		Device:            dev,
		CreateEnrollToken: true,
	}.Build())
	if err != nil {
		return nil, nil, fmt.Errorf("method CreateDevice: %w", err)
	}

	return enrollDevice(ctx, devices, dev, defaultCollectData)
}

func enrollDevice(
	ctx context.Context,
	devices devicepb.DeviceTrustServiceClient, dev *devicepb.Device, collectDataFn collectDataFunc,
) (*devicepb.Device, *fakeEnclaveKey, error) {
	if dev.GetEnrollToken().GetToken() == "" {
		token, err := devices.CreateDeviceEnrollToken(ctx, devicepb.CreateDeviceEnrollTokenRequest_builder{
			DeviceId: dev.GetId(),
		}.Build())
		if err != nil {
			return nil, nil, err
		}
		// Clear token after execution, if we assigned it.
		defer func() { dev.ClearEnrollToken() }()
		dev.SetEnrollToken(token)
	}

	key, err := newFakeEnclaveKey()
	if err != nil {
		return nil, nil, err
	}

	if collectDataFn == nil {
		collectDataFn = defaultCollectData
	}
	sim := key.simulator(withCollectFn(dev, collectDataFn))

	enrolledDev, err := enrollSimulator(ctx, devices, sim, dev)
	if err != nil {
		return nil, nil, err
	}
	return enrolledDev, key, err
}

func enrollSimulator(
	ctx context.Context,
	devices devicepb.DeviceTrustServiceClient, sim simulator, dev *devicepb.Device) (*devicepb.Device, error) {
	stream, err := devices.EnrollDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("method EnrollDevice: %w", err)
	}

	req := sim.enrollRequest(dev, dev.GetEnrollToken().GetToken())
	if err := stream.Send(req); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("init Send: %w", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err, "init Recv") // Keep the trace error.
	}

	enrolledDev, err := sim.handleEnrollStream(resp, stream, false /* testBehavior */)
	if err != nil {
		return nil, fmt.Errorf("simulator enroll: %w", err)
	}
	return enrolledDev, nil
}

func authenticateSimulator(
	ctx context.Context,
	devices devicepb.DeviceTrustServiceClient,
	sim simulator,
	dev *devicepb.Device,
	initCerts *devicepb.UserCertificates) (*devicepb.UserCertificates, error) {
	stream, err := devices.AuthenticateDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("method AuthenticateDevice: %w", err)
	}

	if initCerts == nil {
		initCerts = &devicepb.UserCertificates{}
	}
	resp, err := sim.authenticate(ctx, dev, stream, devicepb.AuthenticateDeviceInit_builder{
		UserCertificates: initCerts,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err, "simulator authenticate") // Keep the trace error.
	}

	respCerts := resp.GetUserCertificates()
	if respCerts == nil {
		return nil, fmt.Errorf("challenge Recv: got payload %T, wanted UserCertificates", resp.Payload)
	}
	return respCerts, nil
}

type collectDataFunc func(*devicepb.Device) *devicepb.DeviceCollectedData

// defaultCollectData attempts to create a devicepb.DeviceCollectedData that
// correctly matches the device and its profile.
func defaultCollectData(dev *devicepb.Device) *devicepb.DeviceCollectedData {
	cd := devicepb.DeviceCollectedData_builder{
		CollectTime:  timestamppb.Now(),
		OsType:       dev.GetOsType(),
		SerialNumber: dev.GetAssetTag(),
	}.Build()
	if dev.HasProfile() {
		cd.SetModelIdentifier(dev.GetProfile().GetModelIdentifier())
		cd.SetOsVersion(dev.GetProfile().GetOsVersion())
		cd.SetOsBuild(dev.GetProfile().GetOsBuild())
		if len(dev.GetProfile().GetOsUsernames()) > 0 {
			cd.SetOsUsername(dev.GetProfile().GetOsUsernames()[0])
			cd.SetOsLoginUser(dev.GetProfile().GetOsUsernames()[0])
		}
		cd.SetJamfBinaryVersion(dev.GetProfile().GetJamfBinaryVersion())
	}
	return cd
}

func verifyUserTrustedDevice(t *testing.T, events []apievents.AuditEvent) {
	t.Helper()

	if len(events) == 0 {
		t.Error("Got zero audit events to verify")
		return
	}

	for i, e := range events {
		de, ok := e.(*apievents.DeviceEvent2)
		if !ok {
			t.Errorf("Event #%v is not a DeviceEvent: %T", i, e)
			continue
		}

		// Skip failures, we are not guaranteed a Device or TrustedDevice for those.
		if !de.Success {
			continue
		}

		// Make sure the Device is set and has some data in it.
		if de.Device == nil || de.Device.DeviceId == "" {
			t.Errorf("Event #%v has a nil Device or an empty DeviceId: %v", i, de.Device)
			continue
		}

		if diff := cmp.Diff(de.Device, de.TrustedDevice, protocmp.Transform()); diff != "" {
			t.Errorf("Event #%v: Device vs TrustedDevice mismatch (-want +got)\n%s", i, diff)
		}
	}
}

type fakeEnclaveKey struct {
	id        string
	priv      *ecdsa.PrivateKey
	pubKeyDER []byte
}

func newFakeEnclaveKey() (*fakeEnclaveKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	pubKeyDER, err := x509.MarshalPKIXPublicKey(priv.Public())
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	return &fakeEnclaveKey{
		id:        id[:],
		priv:      priv,
		pubKeyDER: pubKeyDER,
	}, nil
}

func (k *fakeEnclaveKey) signChallenge(c []byte) (sig []byte, err error) {
	h := sha256.Sum256(c)
	return ecdsa.SignASN1(rand.Reader, k.priv, h[:])
}

func (k *fakeEnclaveKey) deviceCredential() *devicepb.DeviceCredential {
	return devicepb.DeviceCredential_builder{
		Id:           k.id,
		PublicKeyDer: k.pubKeyDER,
	}.Build()
}

type fakeEnclaveKeySimOpt func(b *macOSBehavior)

func withCollectFn(dev *devicepb.Device, fn collectDataFunc) fakeEnclaveKeySimOpt {
	return func(b *macOSBehavior) {
		prevEnroll := b.modifyEnrollDeviceInit
		prevAuthn := b.modifyAuthenticateDeviceInit
		b.modifyEnrollDeviceInit = func(init *devicepb.EnrollDeviceInit) {
			if prevEnroll != nil {
				prevEnroll(init)
			}
			init.SetDeviceData(fn(dev))
		}
		b.modifyAuthenticateDeviceInit = func(init *devicepb.AuthenticateDeviceInit) {
			if prevAuthn != nil {
				prevAuthn(init)
			}
			init.SetDeviceData(fn(dev))
		}
	}
}

func (k *fakeEnclaveKey) simulator(opts ...fakeEnclaveKeySimOpt) simulator {
	b := macOSBehavior{}
	for _, opt := range opts {
		opt(&b)
	}

	sim := newMacOSSimulator(b)
	sim.key = k
	return sim
}

type outgoingContextParams struct {
	User       string // User used for contextWithUser.
	SourceIP   string // IP used for testenv.WithOutgoingClientSourceAddr.
	EmitterKey string // Key used for testenv.WithOutgoingEmitterKey.
}

func configureOutgoingContext(parent context.Context, params outgoingContextParams) context.Context {
	outCtx := parent

	if params.User != "" {
		outCtx = contextWithUser(outCtx, params.User)
	}
	if params.SourceIP != "" {
		outCtx = testenv.WithOutgoingClientSourceAddr(
			outCtx,
			&net.TCPAddr{
				IP:   net.ParseIP(params.SourceIP),
				Port: 12345, // Port is discarded.
			},
		)
	}
	if params.EmitterKey != "" {
		outCtx = testenv.WithOutgoingEmitterKey(outCtx, params.EmitterKey)
	}

	return outCtx
}
