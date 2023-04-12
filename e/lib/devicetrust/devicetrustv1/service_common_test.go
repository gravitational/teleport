package devicetrustv1_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

func createAndEnroll(ctx context.Context, devices devicepb.DeviceTrustServiceClient, dev *devicepb.Device) (*devicepb.Device, *fakeEnclaveKey, error) {
	dev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device:            dev,
		CreateEnrollToken: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("method CreateDevice: %v", err)
	}

	return enrollDevice(ctx, devices, dev, defaultCollectData)
}

func enrollDevice(
	ctx context.Context,
	devices devicepb.DeviceTrustServiceClient,
	dev *devicepb.Device, collectDataFn func(*devicepb.Device) *devicepb.DeviceCollectedData) (*devicepb.Device, *fakeEnclaveKey, error) {
	if collectDataFn == nil {
		collectDataFn = defaultCollectData
	}

	token := dev.EnrollToken.GetToken()
	if token == "" {
		devToken, err := devices.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
			DeviceId: dev.Id,
		})
		if err != nil {
			return nil, nil, err
		}
		token = devToken.Token
	}

	key, err := newFakeEnclaveKey()
	if err != nil {
		return nil, nil, err
	}

	stream, err := devices.EnrollDevice(ctx)
	if err != nil {
		return nil, nil, err
	}
	sendAndRecv := func(msg *devicepb.EnrollDeviceRequest) (*devicepb.EnrollDeviceResponse, error) {
		if err := stream.Send(msg); err != nil {
			return nil, err
		}
		return stream.Recv()
	}

	resp, err := sendAndRecv(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_Init{
			Init: &devicepb.EnrollDeviceInit{
				Token:        token,
				CredentialId: key.id,
				DeviceData:   collectDataFn(dev),
				Macos: &devicepb.MacOSEnrollPayload{
					PublicKeyDer: key.pubKeyDER,
				},
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	chalResp := resp.GetMacosChallenge()
	sig, err := key.signChallenge(chalResp.Challenge)
	if err != nil {
		return nil, nil, err
	}
	resp, err = sendAndRecv(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
			MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
				Signature: sig,
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	return resp.GetSuccess().Device, key, nil
}

func authenticateDevice(
	ctx context.Context,
	devices devicepb.DeviceTrustServiceClient,
	dev *devicepb.Device, devKey *fakeEnclaveKey, collectDataFn func(*devicepb.Device) *devicepb.DeviceCollectedData) error {
	if collectDataFn == nil {
		collectDataFn = defaultCollectData
	}

	stream, err := devices.AuthenticateDevice(ctx)
	if err != nil {
		return err
	}

	// 1. Init.
	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_Init{
			Init: &devicepb.AuthenticateDeviceInit{
				CredentialId: devKey.id,
				DeviceData:   collectDataFn(dev),
			},
		},
	}); err != nil {
		return err
	}
	resp, err := stream.Recv()
	if err != nil {
		return err
	}

	// 2. Challenge.
	sig, err := devKey.signChallenge(resp.GetChallenge().Challenge)
	if err != nil {
		return err
	}
	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
			ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{
				Signature: sig,
			},
		},
	}); err != nil {
		return err
	}
	resp, err = stream.Recv()
	if err != nil {
		return err
	}

	// 3. Success.
	if resp.GetUserCertificates() == nil {
		return fmt.Errorf("got payload type %T, wanted UserCertificates", resp.Payload)
	}
	return nil
}

// defaultCollectData attempts to create a devicepb.DeviceCollectedData that
// correctly matches the device and its profile.
func defaultCollectData(dev *devicepb.Device) *devicepb.DeviceCollectedData {
	cd := &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       dev.OsType,
		SerialNumber: dev.AssetTag,
	}
	if dev.Profile != nil {
		cd.ModelIdentifier = dev.Profile.ModelIdentifier
		cd.OsVersion = dev.Profile.OsVersion
		cd.OsBuild = dev.Profile.OsBuild
		if len(dev.Profile.OsUsernames) > 0 {
			cd.OsUsername = dev.Profile.OsUsernames[0]
		}
		cd.JamfBinaryVersion = dev.Profile.JamfBinaryVersion
	}
	return cd
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
	return &devicepb.DeviceCredential{
		Id:           k.id,
		PublicKeyDer: k.pubKeyDER,
	}
}
