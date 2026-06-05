package devicetrustv1_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
)

func TestAssertCeremony(t *testing.T) {
	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	createAssertCeremony := env.DevicesService.CreateAssertCeremony
	ctx := context.Background()

	dev, key, err := createAndEnroll(ctx, devices, devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "alpaca-dev-1",
	}.Build())
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	t.Run("ok", func(t *testing.T) {
		assertCeremony, err := createAssertCeremony()
		if err != nil {
			t.Fatalf("CreateAssertCeremony failed: %v", err)
		}

		stream := &fakeAssertStream{
			dev: dev,
			key: key,
		}
		got, err := assertCeremony.AssertDevice(ctx, stream)
		if err != nil {
			t.Fatalf("AssertDevice returned err=%q, want nil", err)
		}

		// Assert stream state.
		if !stream.success() {
			t.Error("AssertDevice returned err=nil but the stream was not successful")
		}

		// Assert device.
		if got == nil {
			t.Fatal("AssertDevice returned nil device")
		} else {
			got.SetCollectedData(nil) // CollectedData not relevant for the test.
		}
		if diff := cmp.Diff(dev, got, protocmp.Transform()); diff != "" {
			t.Errorf("AssertDevice device mismatch (-want +got)\n%s", diff)
		}
	})
}

type fakeAssertStreamState int

const (
	assertStateStart fakeAssertStreamState = iota
	assertStateInit
	assertStateChallengeReceived
	assertStateChallengeSolved
	assertStateDeviceAsserted // terminal state
	assertStateFailed         // terminal state
)

// fakeAssertStream drives an assert.Ceremony stream, filling in the parts of
// the client itself.
type fakeAssertStream struct {
	dev *devicepb.Device
	key *fakeEnclaveKey

	state     fakeAssertStreamState
	challenge *devicepb.AuthenticateDeviceChallenge
}

func (s *fakeAssertStream) success() bool {
	return s.state == assertStateDeviceAsserted
}

func (s *fakeAssertStream) Recv() (*devicepb.AssertDeviceRequest, error) {
	switch s.state {
	case assertStateStart:
		s.state = assertStateInit

		return devicepb.AssertDeviceRequest_builder{
			Init: devicepb.AssertDeviceInit_builder{
				CredentialId: s.key.id,
				DeviceData:   defaultCollectData(s.dev),
			}.Build(),
		}.Build(), nil

	case assertStateChallengeReceived:
		s.state = assertStateChallengeSolved

		sig, err := s.key.signChallenge(s.challenge.GetChallenge())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return devicepb.AssertDeviceRequest_builder{
			ChallengeResponse: devicepb.AuthenticateDeviceChallengeResponse_builder{
				Signature: sig,
			}.Build(),
		}.Build(), nil

	default:
		s.state = assertStateFailed
		return nil, fmt.Errorf("unexpected Recv interaction, state %v", s.state)
	}
}

func (s *fakeAssertStream) Send(resp *devicepb.AssertDeviceResponse) error {
	switch s.state {
	case assertStateInit:
		chal := resp.GetChallenge()
		if chal == nil {
			s.state = assertStateFailed
			return fmt.Errorf("unexpected Send flow: expected challenge, got payload %T", resp.Payload)
		}

		s.state = assertStateChallengeReceived
		s.challenge = chal
		return nil

	case assertStateChallengeSolved:
		if resp.GetDeviceAsserted() == nil {
			s.state = assertStateFailed
			return fmt.Errorf("unexpected Send flow: expected DeviceAsserted, got payload %T", resp.Payload)
		}

		s.state = assertStateDeviceAsserted
		return nil

	default:
		s.state = assertStateFailed
		return fmt.Errorf("unexpected Send interaction, state %v", s.state)
	}
}
