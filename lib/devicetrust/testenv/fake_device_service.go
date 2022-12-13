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
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

type fakeDeviceService struct {
	devicepb.UnimplementedDeviceTrustServiceServer
}

func newFakeDeviceService() *fakeDeviceService {
	return &fakeDeviceService{}
}

func (s *fakeDeviceService) EnrollDevice(stream devicepb.DeviceTrustService_EnrollDeviceServer) error {
	// As long as all required fields are non-nil and the challenge signature
	// matches, the fake server lets any device be enrolled.
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
	case initReq.DeviceData == nil:
		return trace.BadParameter("device data required")
	case initReq.DeviceData.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return trace.BadParameter("device OsType required")
	case initReq.DeviceData.SerialNumber == "":
		return trace.BadParameter("device SerialNumber required")
	}

	// OS-specific enrollment.
	if initReq.DeviceData.OsType != devicepb.OSType_OS_TYPE_MACOS {
		return trace.BadParameter("os not supported")
	}
	cred, err := enrollMacOS(stream, initReq)
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

func enrollMacOS(stream devicepb.DeviceTrustService_EnrollDeviceServer, initReq *devicepb.EnrollDeviceInit) (*devicepb.DeviceCredential, error) {
	switch {
	case initReq.Macos == nil:
		return nil, trace.BadParameter("device Macos data required")
	case len(initReq.Macos.PublicKeyDer) == 0:
		return nil, trace.BadParameter("device Macos.PublicKeyDer required")
	}
	pubKey, err := x509.ParsePKIXPublicKey(initReq.Macos.PublicKeyDer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ecPubKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, trace.BadParameter("unexpected public key type: %T", pubKey)
	}

	// 2. Challenge.
	chal := make([]byte, 32)
	if _, err := rand.Reader.Read(chal); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := stream.Send(&devicepb.EnrollDeviceResponse{
		Payload: &devicepb.EnrollDeviceResponse_MacosChallenge{
			MacosChallenge: &devicepb.MacOSEnrollChallenge{
				Challenge: chal,
			},
		},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// 3. Challenge response.
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	chalResp := resp.GetMacosChallengeResponse()
	switch {
	case chalResp == nil:
		return nil, trace.BadParameter("challenge response required")
	case len(chalResp.Signature) == 0:
		return nil, trace.BadParameter("signature required")
	}
	h := sha256.Sum256(chal)
	if !ecdsa.VerifyASN1(ecPubKey, h[:], chalResp.Signature) {
		return nil, trace.BadParameter("signature verification failed")
	}

	return &devicepb.DeviceCredential{
		Id:           initReq.CredentialId,
		PublicKeyDer: initReq.Macos.PublicKeyDer,
	}, nil
}
