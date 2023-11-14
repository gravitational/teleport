//go:build tpmsimulator

package devicetrustv1_test

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/x509"
	"errors"
	"fmt"
	"io"

	"github.com/google/go-attestation/attest"
	tpmsimulator "github.com/google/go-tpm-tools/simulator"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	dtoss "github.com/gravitational/teleport/lib/devicetrust"
)

// To include tests based on this simulator, use the `tpmsimulator` build tag.
// This requires openssl libraries to be installed on the machine and findable
// by the compiler. On macOS:
// brew install openssl
// export C_INCLUDE_PATH="$(brew --prefix openssl)/include"
// export LIBRARY_PATH="$(brew --prefix openssl)/lib"
// go test ./e/lib/devicetrust/devicetrustv1 -run TestService_EnrollDevice -tags tpmsimulator

// inform tests they do not need to skip
var tpmSkip = ""

// fakeCmdChannel is used to inject the TPM simulator into `go-attestation`'s
// TPM wrapper.
type fakeCmdChannel struct {
	io.ReadWriteCloser
}

// MeasurementLog implements CommandChannelTPM20.
func (cc *fakeCmdChannel) MeasurementLog() ([]byte, error) {
	// Return nil, we inject an event log in handleEnrollStream
	return nil, nil
}

// PCR on Index 16 is the "debug" PCR we should use in tests.
// See:
// https://trustedcomputinggroup.org/wp-content/uploads/TCG_PCClient_PFP_r1p05_v23_pub.pdf#page=55 (3.3.4.9: PCR[16] - DEBUG)
const debugPCR uint32 = 16

// eventLogHeader is a SHA1 Event Log Entry that indicates to a verifier to
// switch to parsing Crypto Agile Event Log Entries.
// This following data indicates support for SHA-1 and SHA-256 PCR banks.
// See:
// - https://trustedcomputinggroup.org/wp-content/uploads/EFI-Protocol-Specification-rev13-160330final.pdf#page=15 (5.1: SHA1 Event Log Entry Format)
// - https://trustedcomputinggroup.org/wp-content/uploads/EFI-Protocol-Specification-rev13-160330final.pdf#page=18 (5.3: Event Log Header)
var eventLogHeader = []byte{
	// PCRIndex: 0
	// This value is zero for the event log header.
	0x0, 0x0, 0x0, 0x0,
	// EventType: 0x3 (EV_NO_ACTION)
	0x3, 0x0, 0x0, 0x0,
	// Digest (20 bytes): 0
	// This value is zero for the event log header.
	0x0, 0x0, 0x0, 0x0,
	0x0, 0x0, 0x0, 0x0,
	0x0, 0x0, 0x0, 0x0,
	0x0, 0x0, 0x0, 0x0,
	0x0, 0x0, 0x0, 0x0,
	// EventSize: 37
	0x25, 0x0, 0x0, 0x0,
	// Event (37 bytes):
	//   tdTCG_EfiSpecIdEventStruct
	//   Signature: “Spec ID Event03”
	0x53, 0x70, 0x65, 0x63,
	0x20, 0x49, 0x44, 0x20,
	0x45, 0x76, 0x65, 0x6e,
	0x74, 0x30, 0x33, 0x0,
	//   PlatformClass: 0
	0x0, 0x0, 0x0, 0x0,
	//   SpecVersionMinor: 0
	0x0,
	//   SpecVersionMajor: 2
	0x2,
	//   SpecErrata: 0
	0x0,
	//   UIntnSize: 2 (UINT64)
	0x2,
	//   DigestSizes:
	//   Count: 2
	0x2, 0x0, 0x0, 0x0,
	//   DigestSizes[0].AlgorithmId: 0x4 (SHA1)
	0x4, 0x0,
	//   DigestSizes[0].DigestSize: 20 bytes (160 bits)
	0x14, 0x0,
	//   DigestSizes[1].AlgorithmId: 0xb (SHA256)
	0xb, 0x0,
	//   DigestSizes[1].DigestSize: 32 bytes (256 bits)
	0x20, 0x0,
	// Trailing null
	0x0,
}

// pcrAppendEvent is a Crypto Agile Log Entry Format that appends to PCR 16
// in both SHA-1 and SHA-256. We can inject this event into a log w/o actually
// updating the PCR in order to ensure that the server validates the event log
// against the PCR.
// See:
// - https://trustedcomputinggroup.org/wp-content/uploads/EFI-Protocol-Specification-rev13-160330final.pdf#page=15 (5.2: Crypto Agile Event Log Entry Format)
var pcrAppendEvent = []byte{
	// PCRIndex: 16 (this should match the value of `debugPCR`)
	0x10, 0x0, 0x0, 0x0,
	// EventType: 0x5 (EV_ACTION)
	0x5, 0x0, 0x0, 0x0,
	// Digests:
	// Count: 2
	0x2, 0x0, 0x0, 0x0,
	// Digests[0].AlgorithmId: 0x4 (SHA1)
	0x4, 0x0,
	// Digests[0].Digest: 20 bytes of 0xff
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	// Digests[1].AlgorithmId: 0xb (SHA256)
	0xb, 0x0,
	// Digests[1].Digest: 32 bytes of 0xff
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	// EventSize: 1
	0x1, 0x0, 0x0, 0x0,
	// Event: 0 (this is just some filler data, it doesn't affect anything)
	0x0,
}

type tpmSimulator struct {
	behavior tpmBehavior

	sim *tpmsimulator.Simulator
	tpm *attest.TPM
	ak  *attest.AK

	// ekPub is the ASN.1 DER public part of the TPM EK
	ekPub []byte
	// ekCert is the ASN.1 DER encoded TPM EK Certificate, if one has been
	// generated.
	ekCert []byte

	credentialID string
}

func newTPMSimulator(behavior tpmBehavior) *tpmSimulator {
	return &tpmSimulator{
		behavior: behavior,
	}
}

func (e *tpmSimulator) setup() (closer func(), err error) {
	closeFn := func() {
		if e.tpm != nil {
			if e.ak != nil {
				e.ak.Close(e.tpm)
			}
			e.tpm.Close()
		}
		if e.sim != nil {
			e.sim.Close()
		}
	}
	defer func() {
		// Force closure if returning with an error. This ensures one failing
		// test doesn't affect others (the TPM simulator is not concurrency
		// safe)
		if err != nil {
			closeFn()
		}
	}()

	if e.behavior.incorrectAttestEvent && e.behavior.incorrectAttestPCR {
		return nil, fmt.Errorf("incorrectAttestEvent and incorrectAttestPCR are mutually exclusive")
	}

	e.sim, err = tpmsimulator.Get()
	if err != nil {
		return nil, fmt.Errorf("getting tpm simulator: %w", err)
	}

	e.tpm, err = attest.OpenTPM(&attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
		CommandChannel: &fakeCmdChannel{
			ReadWriteCloser: e.sim,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("opening tpm: %w", err)
	}

	e.ak, err = e.tpm.NewAK(&attest.AKConfig{})
	if err != nil {
		return nil, fmt.Errorf("creating ak: %w", err)
	}

	eks, err := e.tpm.EKs()
	if err != nil {
		return nil, fmt.Errorf("fetching ek: %w", err)
	}
	e.ekPub, err = x509.MarshalPKIXPublicKey(eks[0].Public)
	if err != nil {
		return nil, fmt.Errorf("marshaling ek: %w", err)
	}
	if e.behavior.ekCertGenerator != nil {
		e.ekCert, err = e.behavior.ekCertGenerator(eks[0].Public)
		if err != nil {
			return nil, fmt.Errorf("generating ek cert: %w", err)
		}
	}

	e.credentialID = uuid.NewString()

	return closeFn, nil
}

func (e *tpmSimulator) enrollRequest(
	dev *devicepb.Device,
	enrollToken string,
) *devicepb.EnrollDeviceRequest {
	payload := &devicepb.TPMEnrollPayload{
		Ek: &devicepb.TPMEnrollPayload_EkKey{
			EkKey: e.ekPub,
		},
		AttestationParameters: dtoss.AttestationParametersToProto(e.ak.AttestationParameters()),
	}
	// If EKCert is available, send that instead
	if e.ekCert != nil {
		payload.Ek = &devicepb.TPMEnrollPayload_EkCert{
			EkCert: e.ekCert,
		}
	}

	init := &devicepb.EnrollDeviceInit{
		Token:        enrollToken,
		CredentialId: e.credentialID,
		DeviceData: &devicepb.DeviceCollectedData{
			CollectTime:  timestamppb.Now(),
			OsType:       dev.OsType,
			SerialNumber: dev.AssetTag,
		},
		Tpm: payload,
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

func (e *tpmSimulator) handleEnrollStream(
	resp *devicepb.EnrollDeviceResponse,
	stream devicepb.DeviceTrustService_EnrollDeviceClient,
	testBehavior bool,
) (*devicepb.Device, error) {
	c := resp.GetTpmChallenge()

	platParams, err := e.attest(c.AttestationNonce, testBehavior)
	if err != nil {
		return nil, fmt.Errorf("attesting: %w", err)
	}

	var solution []byte
	if testBehavior && e.behavior.incorrectCredActivateSolution {
		solution = []byte("incorrect-solution")
	} else {
		solution, err = e.ak.ActivateCredential(
			e.tpm, *dtoss.EncryptedCredentialFromProto(c.EncryptedCredential),
		)
		if err != nil {
			return nil, fmt.Errorf("activating credential: %w", err)
		}
	}

	// 2. Challenge.
	if err := stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: &devicepb.TPMEnrollChallengeResponse{
				PlatformParameters: dtoss.PlatformParametersToProto(platParams),
				Solution:           solution,
			},
		},
	}); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("challenge: send: %w", err)
	}
	resp, err = stream.Recv()
	if err != nil {
		return nil, err // Unaltered, so it can be asserted.
	}

	// 3. Success.
	return resp.GetSuccess().GetDevice(), nil
}

func (e *tpmSimulator) wantCredential() *devicepb.DeviceCredential {
	cred := &devicepb.DeviceCredential{
		Id:          e.credentialID,
		TpmAkPublic: e.ak.AttestationParameters().Public,
	}
	if e.behavior.ekCertGenerator != nil {
		cred.TpmEkcertSerial = ekCertSerial
	}
	return cred
}

func (e *tpmSimulator) attest(chalNonce []byte, testBehavior bool) (*attest.PlatformParameters, error) {
	eventLog := []byte{} // Must be non-nil, even if empty.
	if !e.behavior.emptyEventLog {
		// By default, we just inject the event that signals to the verifier that
		// the rest of the event log is in TPM2.0 format.
		eventLog = append(eventLog, eventLogHeader...)
		if testBehavior && e.behavior.incorrectAttestEvent {
			// Simulate a bad actor injecting an event into the event log which
			// has not been applied to the PCRs. This creates a mismatch between
			// the event log and the PCRs.
			// This should yield an error like:
			// `verifying event log\n\tevent log failed to verify: the following registers failed to replay: [16]`
			eventLog = append(eventLog, pcrAppendEvent...)
		}
	}

	ak := e.ak
	if testBehavior && e.behavior.incorrectAttestAK {
		// Simulate a bad actor signing the platform attestation with an AK
		// they control rather than the AK we expect.
		// This should yield an error like:
		// `verifying pcrs\n\tquote 0: invalid quote signature: crypto/rsa: verification error`
		newAK, err := e.tpm.NewAK(&attest.AKConfig{})
		if err != nil {
			return nil, fmt.Errorf("creating incorrect ak: %w", err)
		}
		ak = newAK
	}
	nonce := chalNonce
	if testBehavior && e.behavior.incorrectAttestNonce {
		// Simulate a bad actor replaying an old platform attestation with
		// an old nonce.
		// This should yield an error like:
		// `"verifying pcrs\n\tquote 0: nonce`
		nonce = []byte("some-previous-nonce")
	}
	platParams, err := e.tpm.AttestPlatform(
		ak, nonce, &attest.PlatformAttestConfig{
			EventLog: eventLog,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("performing platform attestation: %w", err)
	}

	if testBehavior && e.behavior.incorrectAttestPCR {
		// Simulate a bad actor interfering with the PCR values sent in the
		// platform attestation after a quote has been taken. This ensures the
		// backend checks the PCR values against the Quote.
		//
		// This should yield an error like:
		// `verifying pcrs\n\tquote 0: quote digest didn't match pcrs provided`
		//
		// The first PCR bank in the msft simulator is SHA-1.
		platParams.PCRs[debugPCR].Digest = bytes.Repeat([]byte{0xab}, sha1.Size)
	}

	return platParams, nil
}

func (e *tpmSimulator) authenticate(
	ctx context.Context,
	dev *devicepb.Device,
	stream devicepb.DeviceTrustService_AuthenticateDeviceClient,
	certs *devicepb.UserCertificates,
) (*devicepb.AuthenticateDeviceResponse, error) {
	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_Init{
			Init: &devicepb.AuthenticateDeviceInit{
				UserCertificates: certs,
				CredentialId:     e.credentialID,
				DeviceData: &devicepb.DeviceCollectedData{
					CollectTime:  timestamppb.Now(),
					OsType:       dev.OsType,
					SerialNumber: dev.AssetTag,
				},
			},
		},
	}); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("sending AuthenticateDeviceRequest_Init: %w", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, err // Unaltered, so it can be asserted.
	}

	challenge := resp.GetTpmChallenge()
	if challenge == nil {
		return nil, fmt.Errorf("unexpected payload=%T, want TPMAuthenticateDeviceChallenge ", resp.Payload)
	}

	platParams, err := e.attest(challenge.AttestationNonce, true)
	if err != nil {
		return nil, fmt.Errorf("attesting: %w", err)
	}

	if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
		Payload: &devicepb.AuthenticateDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: &devicepb.TPMAuthenticateDeviceChallengeResponse{
				PlatformParameters: dtoss.PlatformParametersToProto(platParams),
			},
		},
	}); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("sending AuthenticateDeviceRequest_ChallengeResponse: %w", err)
	}
	resp, err = stream.Recv()
	if err != nil {
		return nil, err // Unaltered, so it can be asserted.
	}
	return resp, nil
}
