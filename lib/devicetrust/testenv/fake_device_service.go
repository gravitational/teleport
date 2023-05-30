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

package testenv

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

type storedDevice struct {
	pb  *devicepb.Device
	pub *ecdsa.PublicKey
}

type fakeDeviceService struct {
	devicepb.UnimplementedDeviceTrustServiceServer

	mu      sync.Mutex
	devices []storedDevice
}

func newFakeDeviceService() *fakeDeviceService {
	return &fakeDeviceService{}
}

// CreateDeviceEnrollToken implements the creation of fake device enrollment
// tokens.
//
// Only auto-enrollment is supported by the fake.
//
// Neither the device or token are stored, as the fake EnrollDevice doesn't
// verify tokens.
func (s *fakeDeviceService) CreateDeviceEnrollToken(ctx context.Context, req *devicepb.CreateDeviceEnrollTokenRequest) (*devicepb.DeviceEnrollToken, error) {
	if req.DeviceId != "" {
		return nil, trace.AccessDenied("device ID token issuance not supported")
	}
	if err := validateCollectedData(req.DeviceData); err != nil {
		return nil, trace.AccessDenied(err.Error())
	}

	return &devicepb.DeviceEnrollToken{
		Token: "fakedeviceenrolltoken",
	}, nil
}

// EnrollDevice implements a fake, server-side device enrollment ceremony.
//
// As long as all required fields are non-nil and the challenge signature
// matches, the fake server lets any device be enrolled. Unlike a proper
// DeviceTrustService implementation, it's not necessary to call CreateDevice or
// acquire an enrollment token from the server.
func (s *fakeDeviceService) EnrollDevice(stream devicepb.DeviceTrustService_EnrollDeviceServer) error {
	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	initReq := req.GetInit()
	switch {
	case initReq == nil:
		return trace.BadParameter("init required")
	case initReq.Token == "":
		return trace.BadParameter("token required")
	case initReq.CredentialId == "":
		return trace.BadParameter("credential ID required")
	}
	if err := validateCollectedData(initReq.DeviceData); err != nil {
		return trace.Wrap(err)
	}

	// OS-specific enrollment.
	var cred *devicepb.DeviceCredential
	var pub *ecdsa.PublicKey
	switch initReq.DeviceData.OsType {
	case devicepb.OSType_OS_TYPE_MACOS:
		cred, pub, err = enrollMacOS(stream, initReq)
		// err handled below
	case devicepb.OSType_OS_TYPE_WINDOWS:
		cred, err = enrollTPM(stream, initReq)
		// err handled below
	default:
		return trace.BadParameter("os not supported")
	}
	if err != nil {
		return trace.Wrap(err)
	}

	// Prepare device.
	cd := initReq.DeviceData
	now := timestamppb.Now()
	dev := &devicepb.Device{
		ApiVersion:   "v1",
		Id:           uuid.NewString(),
		OsType:       cd.OsType,
		AssetTag:     cd.SerialNumber,
		CreateTime:   now,
		UpdateTime:   now,
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED,
		Credential:   cred,
	}
	s.mu.Lock()
	s.devices = append(s.devices, storedDevice{
		pb:  dev,
		pub: pub,
	})
	s.mu.Unlock()

	// Success.
	err = stream.Send(&devicepb.EnrollDeviceResponse{
		Payload: &devicepb.EnrollDeviceResponse_Success{
			Success: &devicepb.EnrollDeviceSuccess{
				Device: dev,
			},
		},
	})
	return trace.Wrap(err)
}

func randomBytes() ([]byte, error) {
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	return buf, err
}

func enrollTPM(stream devicepb.DeviceTrustService_EnrollDeviceServer, initReq *devicepb.EnrollDeviceInit) (*devicepb.DeviceCredential, error) {
	switch {
	case initReq.Tpm == nil:
		return nil, trace.BadParameter("init req missing tpm message")
	case !bytes.Equal(validEKKey, initReq.Tpm.GetEkKey()):
		return nil, trace.BadParameter("ek key in init req did not match expected")
	case !proto.Equal(initReq.Tpm.AttestationParameters, validAttestationParameters):
		return nil, trace.BadParameter("init req tpm message attestation parameters mismatch")
	}

	secret, err := randomBytes()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credentialBlob, err := randomBytes()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	expectSolution := append(secret, credentialBlob...)
	nonce, err := randomBytes()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := stream.Send(&devicepb.EnrollDeviceResponse{
		Payload: &devicepb.EnrollDeviceResponse_TpmChallenge{
			TpmChallenge: &devicepb.TPMEnrollChallenge{
				EncryptedCredential: &devicepb.TPMEncryptedCredential{
					CredentialBlob: credentialBlob,
					Secret:         secret,
				},
				AttestationNonce: nonce,
			},
		},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chalResp := resp.GetTpmChallengeResponse()
	switch {
	case chalResp == nil:
		return nil, trace.BadParameter("challenge response required")
	case !bytes.Equal(expectSolution, chalResp.Solution):
		return nil, trace.BadParameter("activate credential solution in challenge response did not match expected")
	case chalResp.PlatformParameters == nil:
		return nil, trace.BadParameter("missing platform parameters in challenge response")
	case !bytes.Equal(nonce, chalResp.PlatformParameters.EventLog):
		return nil, trace.BadParameter("nonce in challenge response did not match expected")
	}

	return &devicepb.DeviceCredential{
		Id:                    initReq.CredentialId,
		DeviceAttestationType: devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
	}, nil
}

func enrollMacOS(stream devicepb.DeviceTrustService_EnrollDeviceServer, initReq *devicepb.EnrollDeviceInit) (*devicepb.DeviceCredential, *ecdsa.PublicKey, error) {
	switch {
	case initReq.Macos == nil:
		return nil, nil, trace.BadParameter("device Macos data required")
	case len(initReq.Macos.PublicKeyDer) == 0:
		return nil, nil, trace.BadParameter("device Macos.PublicKeyDer required")
	}
	pubKey, err := x509.ParsePKIXPublicKey(initReq.Macos.PublicKeyDer)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	ecPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, nil, trace.BadParameter("unexpected public key type: %T", pubKey)
	}

	// 2. Challenge.
	chal, err := newChallenge()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if err := stream.Send(&devicepb.EnrollDeviceResponse{
		Payload: &devicepb.EnrollDeviceResponse_MacosChallenge{
			MacosChallenge: &devicepb.MacOSEnrollChallenge{
				Challenge: chal,
			},
		},
	}); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// 3. Challenge response.
	resp, err := stream.Recv()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	chalResp := resp.GetMacosChallengeResponse()
	switch {
	case chalResp == nil:
		return nil, nil, trace.BadParameter("challenge response required")
	case len(chalResp.Signature) == 0:
		return nil, nil, trace.BadParameter("signature required")
	}
	if err := verifyChallenge(chal, chalResp.Signature, ecPubKey); err != nil {
		return nil, nil, trace.BadParameter("signature verification failed")
	}

	return &devicepb.DeviceCredential{
		Id:           initReq.CredentialId,
		PublicKeyDer: initReq.Macos.PublicKeyDer,
	}, ecPubKey, nil
}

// AuthenticateDevice implements a fake, server-side device authentication
// ceremony.
//
// AuthenticateDevice requires an enrolled device, so the challenge signature
// can be verified. It largely ignores received certificates and doesn't reply
// with proper certificates in the response. Certificates are acquired outside
// of devicetrust packages, so it's not essential to check them here.
func (s *fakeDeviceService) AuthenticateDevice(stream devicepb.DeviceTrustService_AuthenticateDeviceServer) error {
	// 1. Init.
	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	initReq := req.GetInit()
	switch {
	case initReq == nil:
		return trace.BadParameter("init required")
	case initReq.CredentialId == "":
		return trace.BadParameter("credential ID required")
	}
	if err := validateCollectedData(initReq.DeviceData); err != nil {
		return trace.Wrap(err)
	}
	dev, err := s.findMatchingDevice(initReq.DeviceData, initReq.CredentialId)
	if err != nil {
		return trace.Wrap(err)
	}

	switch dev.pb.OsType {
	case devicepb.OSType_OS_TYPE_MACOS:
		err = authenticateDeviceMacOS(dev, stream)
	case devicepb.OSType_OS_TYPE_WINDOWS:
		err = authenticateDeviceTPM(stream)
	default:
		err = fmt.Errorf("unrecognized os type %q", dev.pb.OsType)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	err = stream.Send(&devicepb.AuthenticateDeviceResponse{
		Payload: &devicepb.AuthenticateDeviceResponse_UserCertificates{
			UserCertificates: &devicepb.UserCertificates{
				X509Der:          []byte("<insert augmented X.509 cert here"),
				SshAuthorizedKey: []byte("<insert augmented SSH cert here"),
			},
		},
	})
	return trace.Wrap(err)
}

func authenticateDeviceMacOS(dev *storedDevice, stream devicepb.DeviceTrustService_AuthenticateDeviceServer) error {
	// 2. Challenge.
	chal, err := newChallenge()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := stream.Send(&devicepb.AuthenticateDeviceResponse{
		Payload: &devicepb.AuthenticateDeviceResponse_Challenge{
			Challenge: &devicepb.AuthenticateDeviceChallenge{
				Challenge: chal,
			},
		},
	}); err != nil {
		return trace.Wrap(err)
	}
	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	// 3. Challenge response.
	chalResp := req.GetChallengeResponse()
	switch {
	case chalResp == nil:
		return trace.BadParameter("challenge response required")
	case len(chalResp.Signature) == 0:
		return trace.BadParameter("signature required")
	}
	return trace.Wrap(verifyChallenge(chal, chalResp.Signature, dev.pub))
}

func authenticateDeviceTPM(stream devicepb.DeviceTrustService_AuthenticateDeviceServer) error {
	// Produce a nonce we can send in the challenge that we expect to see in
	// the EventLog field of the challenge response.
	nonce, err := randomBytes()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := stream.Send(&devicepb.AuthenticateDeviceResponse{
		Payload: &devicepb.AuthenticateDeviceResponse_TpmChallenge{
			TpmChallenge: &devicepb.TPMAuthenticateDeviceChallenge{
				AttestationNonce: nonce,
			},
		},
	}); err != nil {
		return trace.Wrap(err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	chalResp := resp.GetTpmChallengeResponse()
	switch {
	case chalResp == nil:
		return trace.BadParameter("challenge response required")
	case chalResp.PlatformParameters == nil:
		return trace.BadParameter("missing platform parameters in challenge response")
	case !bytes.Equal(nonce, chalResp.PlatformParameters.EventLog):
		return trace.BadParameter("nonce in challenge response did not match expected")
	}
	return nil
}

func (s *fakeDeviceService) findMatchingDevice(cd *devicepb.DeviceCollectedData, credentialID string) (*storedDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stored := range s.devices {
		if cd.OsType != stored.pb.OsType || cd.SerialNumber != stored.pb.AssetTag {
			continue
		}
		if stored.pb.Credential.Id != credentialID {
			return nil, trace.BadParameter("unknown credential for device")
		}
		return &stored, nil
	}
	return nil, trace.NotFound("device %v/%q not enrolled", cd.OsType, cd.SerialNumber)
}

func validateCollectedData(cd *devicepb.DeviceCollectedData) error {
	switch {
	case cd == nil:
		return trace.BadParameter("device data required")
	case cd.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return trace.BadParameter("device OsType invalid")
	case cd.SerialNumber == "":
		return trace.BadParameter("device SerialNumber required")
	}
	if err := cd.CollectTime.CheckValid(); err != nil {
		return trace.BadParameter("device CollectTime invalid: %v", err)
	}
	return nil
}

func newChallenge() ([]byte, error) {
	chal := make([]byte, 32)
	if _, err := rand.Reader.Read(chal); err != nil {
		return nil, trace.Wrap(err)
	}
	return chal, nil
}

func verifyChallenge(chal, sig []byte, pub *ecdsa.PublicKey) error {
	h := sha256.Sum256(chal)
	if !ecdsa.VerifyASN1(pub, h[:], sig) {
		return trace.BadParameter("signature verification failed")
	}
	return nil
}
