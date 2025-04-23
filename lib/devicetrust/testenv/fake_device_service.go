/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package testenv

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/devicetrust/assertserver"
	"github.com/gravitational/teleport/lib/devicetrust/challenge"
)

// FakeEnrollmentToken is a "free", never spent enrollment token.
const FakeEnrollmentToken = "29d73573-1682-42a1-b28f-c0e42a29942f"

type storedDevice struct {
	pb          *devicepb.Device
	pub         *ecdsa.PublicKey
	enrollToken string // stored separately from the device
}

// storedDeviceAuthnAttempt is the underlying entity behind
// [devicepb.DeviceWebToken] and [devicepb.DeviceConfirmationToken].
type storedDeviceAuthnAttempt struct {
	id                     string
	webSessionID           string
	expectedDeviceID       string
	webToken, confirmToken string
}

type FakeDeviceService struct {
	devicepb.UnimplementedDeviceTrustServiceServer

	autoCreateDevice bool

	// mu guards the fields below it.
	// As a rule of thumb we lock entire methods, so we can work with pointers to
	// the contents of devices without worry.
	mu                  sync.Mutex
	devices             []storedDevice
	devicesLimitReached bool
	deviceAuthnAttempts []*storedDeviceAuthnAttempt
}

func newFakeDeviceService() *FakeDeviceService {
	return &FakeDeviceService{}
}

// CreateDeviceWebTokenParams are the parameters for
// [CreateDeviceWebTokenForTesting].
type CreateDeviceWebTokenParams struct {
	ExpectedDeviceID string
	WebSessionID     string
}

// CreateDeviceWebTokenForTesting creates a fake [devicepb.DeviceWebToken] for
// testing.
// The returned token can be used for a successful [AuthenticateDevice] call.
func (s *FakeDeviceService) CreateDeviceWebTokenForTesting(params CreateDeviceWebTokenParams) (*devicepb.DeviceWebToken, error) {
	// "True" device web token creation requires quite a bit more and calculates
	// the expected device itself.
	// For the purposes of this fake this is good enough to give us confidence
	// that the client-side ceremony is passing all the right inputs.
	switch {
	case params.ExpectedDeviceID == "":
		return nil, trace.BadParameter("param ExpectedDeviceID required")
	case params.WebSessionID == "":
		return nil, trace.BadParameter("param WebSessionID required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.NewString()
	webToken := uuid.NewString()

	s.deviceAuthnAttempts = append(s.deviceAuthnAttempts, &storedDeviceAuthnAttempt{
		id:               id,
		expectedDeviceID: params.ExpectedDeviceID,
		webSessionID:     params.WebSessionID,
		webToken:         webToken,
	})

	return &devicepb.DeviceWebToken{
		Id:    id,
		Token: webToken,
	}, nil
}

// VerifyConfirmationToken verifies that the token is valid within this
// FakeDeviceService.
//
// This is a test support method, it doesn't spend the token.
func (s *FakeDeviceService) VerifyConfirmationToken(token *devicepb.DeviceConfirmationToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, attempt := range s.deviceAuthnAttempts {
		if attempt.id == token.Id && attempt.confirmToken == token.Token {
			return nil
		}
	}

	return errors.New("token not issued by FakeDeviceService")
}

// SetDevicesLimitReached simulates a server where the devices limit was already
// reached.
func (s *FakeDeviceService) SetDevicesLimitReached(limitReached bool) {
	s.mu.Lock()
	s.devicesLimitReached = limitReached
	s.mu.Unlock()
}

func (s *FakeDeviceService) CreateDevice(ctx context.Context, req *devicepb.CreateDeviceRequest) (*devicepb.Device, error) {
	dev := req.Device
	switch {
	case dev == nil:
		return nil, trace.BadParameter("device required")
	case dev.OsType == devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return nil, trace.BadParameter("device OS type required")
	case dev.AssetTag == "":
		return nil, trace.BadParameter("device asset tag required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Do some superficial checks.
	// We don't deeply validate devices or check for ID collisions for brevity.
	for _, sd := range s.devices {
		if sd.pb.OsType == dev.OsType && sd.pb.AssetTag == dev.AssetTag {
			return nil, trace.AlreadyExists("device already registered")
		}
	}

	// Take a copy and ignore most fields, except what we need for testing.
	now := timestamppb.Now()
	created := &devicepb.Device{
		ApiVersion:   "v1",
		Id:           uuid.NewString(),
		OsType:       dev.OsType,
		AssetTag:     dev.AssetTag,
		CreateTime:   now,
		UpdateTime:   now,
		EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
	}

	// Prepare enroll token, if requested.
	var enrollToken string
	if req.CreateEnrollToken {
		enrollToken = uuid.NewString()
	}

	// "Store" device.
	s.devices = append(s.devices, storedDevice{
		pb:          created,
		enrollToken: enrollToken,
	})

	resp := created
	if enrollToken != "" {
		resp = proto.Clone(created).(*devicepb.Device)
		resp.EnrollToken = &devicepb.DeviceEnrollToken{
			Token: enrollToken,
		}
	}
	return resp, nil
}

func (s *FakeDeviceService) FindDevices(ctx context.Context, req *devicepb.FindDevicesRequest) (*devicepb.FindDevicesResponse, error) {
	if req.IdOrTag == "" {
		return nil, trace.BadParameter("param id_or_tag required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var devs []*devicepb.Device
	for _, sd := range s.devices {
		if sd.pb.Id == req.IdOrTag || sd.pb.AssetTag == req.IdOrTag {
			devs = append(devs, sd.pb)
		}
	}

	return &devicepb.FindDevicesResponse{
		Devices: devs,
	}, nil
}

// CreateDeviceEnrollToken implements the creation of fake device enrollment
// tokens.
//
// ID-based creation requires a previously-created device and stores the new
// token.
//
// Auto-enrollment is completely fake, it doesn't require the device to exist.
// Always returns [FakeEnrollmentToken].
func (s *FakeDeviceService) CreateDeviceEnrollToken(ctx context.Context, req *devicepb.CreateDeviceEnrollTokenRequest) (*devicepb.DeviceEnrollToken, error) {
	if req.DeviceId != "" {
		return s.createEnrollTokenID(ctx, req.DeviceId)
	}

	// Auto-enrollment path.
	if err := validateCollectedData(req.DeviceData); err != nil {
		return nil, trace.AccessDenied("%s", err)
	}

	return &devicepb.DeviceEnrollToken{
		Token: FakeEnrollmentToken,
	}, nil
}

func (s *FakeDeviceService) createEnrollTokenID(ctx context.Context, deviceID string) (*devicepb.DeviceEnrollToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sd, err := s.findDeviceByID(deviceID)
	if err != nil {
		return nil, err
	}

	// Create and store token for posterior verification.
	enrollToken := uuid.NewString()
	sd.enrollToken = enrollToken

	return &devicepb.DeviceEnrollToken{
		Token: enrollToken,
	}, nil
}

// EnrollDevice implements a fake, server-side device enrollment ceremony.
//
// If the service was created using [WithAutoCreateDevice], the device is
// automatically created. The enrollment token must either match
// [FakeEnrollmentToken] or be created via a successful
// [CreateDeviceEnrollToken] call.
func (s *FakeDeviceService) EnrollDevice(stream devicepb.DeviceTrustService_EnrollDeviceServer) error {
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
	cd := initReq.DeviceData

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.devicesLimitReached {
		return trace.AccessDenied("cluster has reached its enrolled trusted device limit")
	}

	// Find or auto-create device.
	sd, err := s.findDeviceByOSTag(cd.OsType, cd.SerialNumber)
	switch {
	case s.autoCreateDevice && trace.IsNotFound(err):
		// Auto-created device.
		now := timestamppb.Now()
		dev := &devicepb.Device{
			ApiVersion:   "v1",
			Id:           uuid.NewString(),
			OsType:       cd.OsType,
			AssetTag:     cd.SerialNumber,
			CreateTime:   now,
			UpdateTime:   now,
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
		}
		s.devices = append(s.devices, storedDevice{
			pb: dev,
		})
		sd = &s.devices[len(s.devices)-1]
	case err != nil:
		return err
	}

	// Spend enrollment token.
	if err := s.spendEnrollmentToken(sd, initReq.Token); err != nil {
		return err
	}

	// OS-specific enrollment.
	var cred *devicepb.DeviceCredential
	var pub *ecdsa.PublicKey
	switch initReq.DeviceData.OsType {
	case devicepb.OSType_OS_TYPE_MACOS:
		cred, pub, err = enrollMacOS(stream, initReq)
		// err handled below
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		cred, err = enrollTPM(stream, initReq)
		// err handled below
	default:
		return trace.BadParameter("os not supported")
	}
	if err != nil {
		return trace.Wrap(err)
	}

	// Save enrollment information.
	sd.pb.UpdateTime = timestamppb.Now()
	sd.pb.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED
	sd.pb.Credential = cred
	sd.pub = pub

	// Success.
	err = stream.Send(&devicepb.EnrollDeviceResponse{
		Payload: &devicepb.EnrollDeviceResponse_Success{
			Success: &devicepb.EnrollDeviceSuccess{
				Device: sd.pb,
			},
		},
	})
	return trace.Wrap(err)
}

func (s *FakeDeviceService) spendEnrollmentToken(sd *storedDevice, token string) error {
	if token == FakeEnrollmentToken {
		sd.enrollToken = "" // Clear just in case.
		return nil
	}

	if sd.enrollToken != token {
		return trace.AccessDenied("invalid device enrollment token")
	}

	// "Spend" token.
	sd.enrollToken = ""
	return nil
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

// CreateAssertCeremony creates a fake, server-side device assertion ceremony.
func (s *FakeDeviceService) CreateAssertCeremony() (assertserver.Ceremony, error) {
	return s, nil
}

// AssertDevice implements a fake, server-side device assertion ceremony.
//
// AssertDevice requires an enrolled device, so the challenge signature
// can be verified.
func (s *FakeDeviceService) AssertDevice(ctx context.Context, stream assertserver.AssertDeviceServerStream) (*devicepb.Device, error) {
	// 1. Init.
	req, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	initReq := req.GetInit()
	switch {
	case initReq == nil:
		return nil, trace.BadParameter("init required")
	case initReq.CredentialId == "":
		return nil, trace.BadParameter("credential ID required")
	}
	if err := validateCollectedData(initReq.DeviceData); err != nil {
		return nil, trace.Wrap(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dev, err := s.findDeviceByCredential(initReq.DeviceData, initReq.CredentialId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch dev.pb.OsType {
	case devicepb.OSType_OS_TYPE_MACOS:
		err = authenticateDeviceMacOS(dev, assertStreamAdapter{stream: stream}, nil /*sshCert*/)
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		err = authenticateDeviceTPM(assertStreamAdapter{stream: stream}, nil /*sshCert*/)
	default:
		err = fmt.Errorf("unrecognized os type %q", dev.pb.OsType)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Success.
	return dev.pb, trace.Wrap(stream.Send(&devicepb.AssertDeviceResponse{
		Payload: &devicepb.AssertDeviceResponse_DeviceAsserted{},
	}))
}

// AuthenticateDevice implements a fake, server-side device authentication
// ceremony.
//
// AuthenticateDevice requires an enrolled device, so the challenge signature
// can be verified. It largely ignores received certificates and doesn't reply
// with proper certificates in the response. Certificates are acquired outside
// of devicetrust packages, so it's not essential to check them here.
func (s *FakeDeviceService) AuthenticateDevice(stream devicepb.DeviceTrustService_AuthenticateDeviceServer) error {
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

	s.mu.Lock()
	defer s.mu.Unlock()

	dev, err := s.findDeviceByCredential(initReq.DeviceData, initReq.CredentialId)
	if err != nil {
		return trace.Wrap(err)
	}

	// Validate/spent the device web token, if present.
	var confirmToken *devicepb.DeviceConfirmationToken
	if webToken := initReq.DeviceWebToken; webToken != nil {
		var err error
		confirmToken, err = s.spendDeviceWebToken(webToken, dev)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	switch dev.pb.OsType {
	case devicepb.OSType_OS_TYPE_MACOS:
		err = authenticateDeviceMacOS(dev, stream, initReq.GetUserCertificates().GetSshAuthorizedKey())
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		err = authenticateDeviceTPM(stream, initReq.GetUserCertificates().GetSshAuthorizedKey())
	default:
		err = fmt.Errorf("unrecognized os type %q", dev.pb.OsType)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	// Standalone device authentication.
	if confirmToken == nil {
		return trace.Wrap(stream.Send(&devicepb.AuthenticateDeviceResponse{
			Payload: &devicepb.AuthenticateDeviceResponse_UserCertificates{
				UserCertificates: &devicepb.UserCertificates{
					X509Der:          []byte("<insert augmented X.509 cert here"),
					SshAuthorizedKey: []byte("<insert augmented SSH cert here"),
				},
			},
		}))
	}

	// Web authentication.
	return trace.Wrap(stream.Send(&devicepb.AuthenticateDeviceResponse{
		Payload: &devicepb.AuthenticateDeviceResponse_ConfirmationToken{
			ConfirmationToken: confirmToken,
		},
	}))
}

func (s *FakeDeviceService) spendDeviceWebToken(webToken *devicepb.DeviceWebToken, dev *storedDevice) (*devicepb.DeviceConfirmationToken, error) {
	const invalidWebTokenMessage = "invalid device web token"

	for _, attempt := range s.deviceAuthnAttempts {
		if attempt.id != webToken.Id {
			continue
		}

		storedToken := attempt.webToken

		// Spend token regardless of outcome.
		attempt.webToken = ""

		switch {
		case storedToken == "": // Invalid attempt state or token already spent.
			return nil, trace.AccessDenied("%s", invalidWebTokenMessage)
		case storedToken != webToken.Token: // Bad token
			return nil, trace.AccessDenied("%s", invalidWebTokenMessage)
		case attempt.expectedDeviceID != dev.pb.Id: // Failed expected device check.
			return nil, trace.AccessDenied("%s", invalidWebTokenMessage)
		}

		// Issue a new confirmation token.
		attempt.confirmToken = uuid.NewString()

		return &devicepb.DeviceConfirmationToken{
			Id:    attempt.id,
			Token: attempt.confirmToken,
		}, nil
	}

	// Token ID not found.
	return nil, trace.AccessDenied("%s", invalidWebTokenMessage)
}

func authenticateDeviceMacOS(
	dev *storedDevice,
	stream authenticateDeviceStream,
	sshCert []byte,
) error {
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
	if err := challenge.Verify(chal, chalResp.Signature, dev.pub); err != nil {
		return trace.Wrap(err)
	}

	// Verify SSH challenge signature if augmented SSH cert was requested.
	if len(sshCert) != 0 {
		if err := verifySSHChallenge(sshCert, chal, chalResp.SshSignature); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func authenticateDeviceTPM(stream authenticateDeviceStream, sshCert []byte) error {
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

	// Verify SSH challenge signature if augmented SSH cert was requested.
	if len(sshCert) != 0 {
		if err := verifySSHChallenge(sshCert, nonce, chalResp.SshSignature); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func verifySSHChallenge(sshAuthorizedKey, chal, signature []byte) error {
	switch {
	case len(sshAuthorizedKey) == 0:
		return trace.BadParameter("sshAuthorizedKey required")
	case len(chal) == 0:
		return trace.BadParameter("chal required")
	case len(signature) == 0:
		return trace.BadParameter("signature required")
	}
	sshCert, err := sshutils.ParseCertificate(sshAuthorizedKey)
	if err != nil {
		return trace.Wrap(err, "parsing SSH certificate")
	}
	var pubKey crypto.PublicKey
	if cryptoKey, ok := sshCert.Key.(ssh.CryptoPublicKey); ok {
		pubKey = cryptoKey.CryptoPublicKey()
	} else {
		return trace.BadParameter("unsupported SSH public key type %T", sshCert.Key)
	}
	if err := challenge.Verify(chal, signature, pubKey); err != nil {
		return trace.BadParameter("SSH key verification failed: %v", err)
	}
	return nil
}

func (s *FakeDeviceService) findDeviceByID(deviceID string) (*storedDevice, error) {
	return s.findDeviceByPredicate(func(sd *storedDevice) bool {
		return sd.pb.Id == deviceID
	})
}

func (s *FakeDeviceService) findDeviceByOSTag(osType devicepb.OSType, assetTag string) (*storedDevice, error) {
	return s.findDeviceByPredicate(func(sd *storedDevice) bool {
		return sd.pb.OsType == osType && sd.pb.AssetTag == assetTag
	})
}

func (s *FakeDeviceService) findDeviceByCredential(cd *devicepb.DeviceCollectedData, credentialID string) (*storedDevice, error) {
	sd, err := s.findDeviceByOSTag(cd.OsType, cd.SerialNumber)
	if err != nil {
		return nil, err
	}
	if sd.pb.Credential.Id != credentialID {
		return nil, trace.BadParameter("unknown credential for device")
	}
	return sd, nil
}

func (s *FakeDeviceService) findDeviceByPredicate(fn func(*storedDevice) bool) (*storedDevice, error) {
	for i, stored := range s.devices {
		if fn(&stored) {
			return &s.devices[i], nil
		}
	}
	return nil, trace.NotFound("device not found")
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

type authenticateDeviceStream interface {
	Recv() (*devicepb.AuthenticateDeviceRequest, error)
	Send(*devicepb.AuthenticateDeviceResponse) error
}

// assertStreamAdapter adapts an [assertserver.AssertDeviceServerStream] to an
// [authenticateDeviceStream].
type assertStreamAdapter struct {
	stream assertserver.AssertDeviceServerStream
}

func (s assertStreamAdapter) Recv() (*devicepb.AuthenticateDeviceRequest, error) {
	req, err := s.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert AssertDeviceRequest to AuthenticateDeviceRequest.
	if req == nil || req.Payload == nil {
		return nil, trace.BadParameter("assert request payload required")
	}
	authnReq := &devicepb.AuthenticateDeviceRequest{}
	switch req.Payload.(type) {
	case *devicepb.AssertDeviceRequest_ChallengeResponse:
		authnReq.Payload = &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
			ChallengeResponse: req.GetChallengeResponse(),
		}
	case *devicepb.AssertDeviceRequest_TpmChallengeResponse:
		authnReq.Payload = &devicepb.AuthenticateDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: req.GetTpmChallengeResponse(),
		}
	default:
		return nil, trace.BadParameter("unexpected assert request payload: %T", req.Payload)
	}

	return authnReq, nil
}

func (s assertStreamAdapter) Send(authnResp *devicepb.AuthenticateDeviceResponse) error {
	if authnResp == nil || authnResp.Payload == nil {
		return trace.BadParameter("authenticate response payload required")
	}

	// Convert AuthenticateDeviceResponse to AssertDeviceResponse.
	resp := &devicepb.AssertDeviceResponse{}
	switch authnResp.Payload.(type) {
	case *devicepb.AuthenticateDeviceResponse_Challenge:
		resp.Payload = &devicepb.AssertDeviceResponse_Challenge{
			Challenge: authnResp.GetChallenge(),
		}
	case *devicepb.AuthenticateDeviceResponse_TpmChallenge:
		resp.Payload = &devicepb.AssertDeviceResponse_TpmChallenge{
			TpmChallenge: authnResp.GetTpmChallenge(),
		}
	default:
		return trace.BadParameter("unexpected authentication response payload: %T", authnResp.Payload)
	}

	return trace.Wrap(s.stream.Send(resp))
}
