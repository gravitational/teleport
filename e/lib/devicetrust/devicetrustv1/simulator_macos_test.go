package devicetrustv1_test

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

type macOSBehavior struct {
	incorrectSigningKey bool
	incorrectSignature  bool

	modifyEnrollDeviceInit func(r *devicepb.EnrollDeviceInit)
}

type macOSSimulator struct {
	key      *fakeEnclaveKey
	behavior macOSBehavior
}

func newMacOSSimulator(behavior macOSBehavior) *macOSSimulator {
	return &macOSSimulator{
		behavior: behavior,
	}
}

func (e *macOSSimulator) setup() (closer func(), err error) {
	e.key, err = newFakeEnclaveKey()
	if err != nil {
		return nil, fmt.Errorf("creating key fake: %w", err)
	}
	return func() {}, nil
}

func (e *macOSSimulator) enrollRequest(
	dev *devicepb.Device,
	enrollToken string,
) *devicepb.EnrollDeviceRequest {
	init := &devicepb.EnrollDeviceInit{
		Token:        enrollToken,
		CredentialId: e.key.id,
		DeviceData: &devicepb.DeviceCollectedData{
			CollectTime:  timestamppb.Now(),
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			SerialNumber: dev.AssetTag,
		},
		Macos: &devicepb.MacOSEnrollPayload{
			PublicKeyDer: e.key.pubKeyDER,
		},
	}
	if e.behavior.modifyEnrollDeviceInit != nil {
		e.behavior.modifyEnrollDeviceInit(init)
	}
	return &devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_Init{
			Init: init,
		},
	}
}

func (e *macOSSimulator) handleEnrollStream(
	resp *devicepb.EnrollDeviceResponse,
	stream devicepb.DeviceTrustService_EnrollDeviceClient,
) (*devicepb.Device, error) {
	var err error
	signingKey := e.key
	if e.behavior.incorrectSigningKey {
		signingKey, err = newFakeEnclaveKey()
		if err != nil {
			return nil, fmt.Errorf("creating fake key: %w", err)
		}
	}

	var sig = []byte("not a signature")
	if !e.behavior.incorrectSignature {
		c := resp.GetMacosChallenge().GetChallenge()
		sig, err = signingKey.signChallenge(c)
		if err != nil {
			return nil, fmt.Errorf("signing challenge: %w", err)
		}
	}

	// 2. Challenge.
	if err := stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
			MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
				Signature: sig,
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("challenge: Send failed: %w", err)
	}
	resp, err = stream.Recv()
	if err != nil {
		return nil, err // Unaltered, so it can be asserted.
	}

	// 3. Success.
	return resp.GetSuccess().GetDevice(), nil
}

func (e *macOSSimulator) wantCredential() *devicepb.DeviceCredential {
	return e.key.deviceCredential()
}
