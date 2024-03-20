package devicetrustv1_test

import (
	"context"
	"errors"
	"fmt"
	"io"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

type macOSBehavior struct {
	incorrectSigningKey bool
	incorrectSignature  bool
	nilSignature        bool

	modifyEnrollDeviceInit       func(r *devicepb.EnrollDeviceInit)
	modifyAuthenticateDeviceInit func(r *devicepb.AuthenticateDeviceInit)
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
	nopCloser := func() {}
	e.key, err = newFakeEnclaveKey()
	if err != nil {
		return nil, fmt.Errorf("creating key fake: %w", err)
	}
	return nopCloser, nil
}

func (e *macOSSimulator) enrollRequest(
	dev *devicepb.Device,
	enrollToken string,
) *devicepb.EnrollDeviceRequest {
	init := &devicepb.EnrollDeviceInit{
		Token:        enrollToken,
		CredentialId: e.key.id,
		DeviceData:   defaultCollectData(dev),
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
	testBehavior bool,
) (*devicepb.Device, error) {
	sig, err := e.signChallenge(resp.GetMacosChallenge().Challenge, testBehavior)
	if err != nil {
		return nil, fmt.Errorf("signing challenge: %w", err)
	}

	// 2. Challenge.
	if err := stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
			MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
				Signature: sig,
			},
		},
	}); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("challenge Send: %w", err)
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

func (e *macOSSimulator) signChallenge(c []byte, testBehavior bool) ([]byte, error) {
	if testBehavior && e.behavior.nilSignature {
		return nil, nil
	}

	var err error
	signingKey := e.key
	if testBehavior && e.behavior.incorrectSigningKey {
		signingKey, err = newFakeEnclaveKey()
		if err != nil {
			return nil, fmt.Errorf("creating fake key: %w", err)
		}
	}

	if testBehavior && e.behavior.incorrectSignature {
		return []byte("not a signature"), nil
	}

	sig, err := signingKey.signChallenge(c)
	if err != nil {
		return nil, fmt.Errorf("signing challenge: %w", err)
	}

	return sig, nil
}

func (e *macOSSimulator) authenticate(
	ctx context.Context,
	dev *devicepb.Device,
	stream devicepb.DeviceTrustService_AuthenticateDeviceClient,
	initTemplate *devicepb.AuthenticateDeviceInit,
) (*devicepb.AuthenticateDeviceResponse, error) {
	init := &devicepb.AuthenticateDeviceInit{
		UserCertificates: initTemplate.GetUserCertificates(),
		CredentialId:     e.key.id,
		DeviceData:       defaultCollectData(dev),
		DeviceWebToken:   initTemplate.GetDeviceWebToken(),
	}
	if e.behavior.modifyAuthenticateDeviceInit != nil {
		e.behavior.modifyAuthenticateDeviceInit(init)
	}
	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_Init{
			Init: init,
		},
	}); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("init Send: %w", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, err // Unaltered, so it can be asserted.
	}

	chalResp := resp.GetChallenge()
	if chalResp == nil {
		return nil, fmt.Errorf("init Recv: unexpected payload=%T, want AuthenticateDeviceChallenge ", resp.Payload)
	}
	sig, err := e.signChallenge(chalResp.Challenge, true)
	if err != nil {
		return nil, fmt.Errorf("signing challenge: %w", err)
	}
	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
			ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{
				Signature: sig,
			},
		},
	}); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("challend Send: %w", err)
	}
	resp, err = stream.Recv()
	if err != nil {
		return nil, err // Unaltered, so it can be asserted.
	}
	return resp, nil
}
