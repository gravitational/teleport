package devicetrustv1_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestService_EnrollDevice(t *testing.T) {
	deviceTrustConfig := &types.DeviceTrust{}
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithEmitter(emitter),
		testenv.WithAuthPreferenceSpec(types.AuthPreferenceSpecV2{
			DeviceTrust: deviceTrustConfig,
		}),
	)

	devices := env.DevicesClient
	ctx := context.Background()

	// macOSFailDev is used for failure test scenarios.
	macOSFailDev, err := devices.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
		Device: devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "fail",
		}.Build(),
		CreateEnrollToken: true,
	}.Build())
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	ekCertCA, ekCertCAPEM, err := newFakeEKCertCA()
	if err != nil {
		t.Fatalf("newFakeEKCertCA failed: %v", err)
	}
	unrecognizedEKCertCA, _, err := newFakeEKCertCA()
	if err != nil {
		t.Fatalf("newFakeEKCertCA failed: %v", err)
	}

	// Create a few "wrong" keys to use in MacOS tests:
	// * P-224 (too small for an Enclave key)
	// * RSA-2048 (unexpected for macOS)
	p224Key, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	p224KeyDER, err := x509.MarshalPKIXPublicKey(p224Key.Public())
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey failed: %v", err)
	}
	rsaKey, err := keys.ParsePrivateKey(fixtures.PEMBytes["rsa"])
	if err != nil {
		t.Fatalf("ParsePrivateKey failed: %v", err)
	}
	rsaKeyDER, err := x509.MarshalPKIXPublicKey(rsaKey.Public())
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey failed: %v", err)
	}

	wantEnrollFailure := []wantEvent{
		{
			Type:     events.DeviceEnrollEvent,
			Code:     events.DeviceEnrollCode,
			WantFail: true,
		},
	}
	wantEnrollSuccess := []wantEvent{
		{
			Type: events.DeviceEnrollEvent,
			Code: events.DeviceEnrollCode,
		},
	}

	tests := []struct {
		name       string
		shouldSkip string
		// deviceTemplate is the device to create prior to enrollment.
		// If the device has an Id the test will refresh its enrollment token,
		// otherwise a new device is created.
		deviceTemplate                *devicepb.Device
		deviceTrustConfig             *types.DeviceTrust
		simulator                     simulator
		assertInitErr                 func(err error) bool
		assertHandleErr               func(err error) bool
		wantAuditEvents               []wantEvent
		wantAttestationType           devicepb.DeviceAttestationType
		wantDCDTPMPlatformAttestation bool
	}{
		// General "Init" step validation errors.
		// These are failures regardless of the OsType.
		{
			name:           "init: invalid Token",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.SetToken("invalid")
				},
			}),
			assertInitErr:   trace.IsAccessDenied,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: empty CredentialId",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.SetCredentialId("")
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: nil DeviceData",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.ClearDeviceData()
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: nil DeviceData.CollectTime",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetDeviceData().ClearCollectTime()
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: unspecified DeviceData.OsType",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetDeviceData().SetOsType(devicepb.OSType_OS_TYPE_UNSPECIFIED)
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: mismatched DeviceData.OsType",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetDeviceData().SetOsType(devicepb.OSType_OS_TYPE_LINUX)
				},
			}),
			assertInitErr:   trace.IsNotFound,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			// Mobile devices enroll through the public Device Trust service.
			name:           "init: iOS DeviceData.OsType",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetDeviceData().SetOsType(devicepb.OSType_OS_TYPE_IOS)
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: iPadOS DeviceData.OsType",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetDeviceData().SetOsType(devicepb.OSType_OS_TYPE_IPADOS)
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: empty DeviceData.SerialNumber",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetDeviceData().SetSerialNumber("")
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: unknown DeviceData.SerialNumber",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetDeviceData().SetSerialNumber("unknown")
				},
			}),
			assertInitErr:   trace.IsNotFound,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: device sends platform attestation in dcd",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetDeviceData().SetTpmPlatformAttestation(devicepb.TPMPlatformAttestation_builder{
						Nonce: []byte("a-nonce"),
					}.Build())
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		// Windows
		{
			name:       "windows: success with EKPub",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "llama",
			}.Build(),
			simulator:                     newTPMSimulator(tpmBehavior{}),
			wantAuditEvents:               wantEnrollSuccess,
			wantDCDTPMPlatformAttestation: true,
			wantAttestationType:           devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
		},
		{
			name:       "windows: success with EKCert",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "llama-ekcert",
			}.Build(),
			simulator: newTPMSimulator(tpmBehavior{
				ekCertGenerator: ekCertCA,
			}),
			wantAuditEvents:               wantEnrollSuccess,
			wantDCDTPMPlatformAttestation: true,
			wantAttestationType:           devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKCERT,
		},
		{
			name:       "windows: success with EKCert trusted",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "llama-ekcert-trusted",
			}.Build(),
			deviceTrustConfig: &types.DeviceTrust{
				EKCertAllowedCAs: []string{
					string(ekCertCAPEM),
				},
			},
			simulator: newTPMSimulator(tpmBehavior{
				ekCertGenerator: ekCertCA,
			}),
			wantAuditEvents:               wantEnrollSuccess,
			wantDCDTPMPlatformAttestation: true,
			wantAttestationType:           devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKCERT_TRUSTED,
		},
		// Linux
		{
			name:       "linux: success with EKPub",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_LINUX,
				AssetTag: "linux-tpm-success",
			}.Build(),
			simulator: newTPMSimulator(tpmBehavior{
				emptyEventLog: true,
			}),
			wantAuditEvents:               wantEnrollSuccess,
			wantAttestationType:           devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
			wantDCDTPMPlatformAttestation: true,
		},
		// General TPM failure cases
		{
			name:       "tpm: missing TPM payload in init",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-no-payload",
			}.Build(),
			simulator: newTPMSimulator(tpmBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.ClearTpm()
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: no attest params in TPM payload",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-no-attest-params",
			}.Build(),
			simulator: newTPMSimulator(tpmBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetTpm().ClearAttestationParameters()
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: no EK in TPM payload",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-no-ek",
			}.Build(),
			simulator: newTPMSimulator(tpmBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetTpm().ClearEk()
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: incorrect platform attestation AK",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-incorrect-attest-ak",
			}.Build(),
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestAK: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: incorrect platform attestation nonce",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-incorrect-attest-nonce",
			}.Build(),
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestNonce: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: incorrect credential activation solution",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-incorrect-activation-solution",
			}.Build(),
			simulator: newTPMSimulator(tpmBehavior{
				incorrectCredActivateSolution: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: incorrect platform attestation pcr",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-incorrect-attest-pcr",
			}.Build(),
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestPCR: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: incorrect platform attestation event",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-incorrect-attest-event",
			}.Build(),
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestEvent: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: ekpub provided when allowed_ekcert_cas configured",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-ekpub-allowed-ekcert-cas",
			}.Build(),
			deviceTrustConfig: &types.DeviceTrust{
				EKCertAllowedCAs: []string{
					string(ekCertCAPEM),
				},
			},
			simulator:       newTPMSimulator(tpmBehavior{}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: ekcert from unrecognized CA",
			shouldSkip: tpmSkip,
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-ekcert-unrecognized-ca",
			}.Build(),
			deviceTrustConfig: &types.DeviceTrust{
				EKCertAllowedCAs: []string{
					string(ekCertCAPEM),
				},
			},
			simulator: newTPMSimulator(tpmBehavior{
				ekCertGenerator: unrecognizedEKCertCA,
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		// macOS enrollment and edge cases.
		{
			name: "macOS success",
			deviceTemplate: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama",
			}.Build(),
			simulator:       newMacOSSimulator(macOSBehavior{}),
			wantAuditEvents: wantEnrollSuccess,
		},
		{
			name:           "macOS: nil Macos payload",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.ClearMacos()
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "macOS: empty Macos.PubKeyDER",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetMacos().SetPublicKeyDer([]byte{})
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "macOS: invalid Macos.PubKeyDER",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetMacos().SetPublicKeyDer([]byte("not a public key"))
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "macOS: unexpected Macos.PubKeyDER type",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetMacos().SetPublicKeyDer(rsaKeyDER) // Enclave keys are ECDSA.
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "macOS: unexpected Macos.PubKeyDER curve",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.GetMacos().SetPublicKeyDer(p224KeyDER) // Enclave keys are P-256.
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "macOS: invalid challenge signature",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				incorrectSignature: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "macOS: wrong key signs the challenge",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				incorrectSigningKey: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Allowing skipping of tests on non-supported platforms.
			if test.shouldSkip != "" {
				t.Skip(test.shouldSkip)
			}
			if test.deviceTrustConfig != nil {
				*deviceTrustConfig = *test.deviceTrustConfig
				defer func() {
					*deviceTrustConfig = types.DeviceTrust{}
				}()
			}
			// Allow underlying device mock to be set up
			cleanup, err := test.simulator.setup()
			if err != nil {
				t.Fatalf("Failed to setup device simulator: %v", err)
			}
			defer cleanup()

			// Create device and enrollment token to use below.
			var created *devicepb.Device
			enrollToken := ""
			switch dev := test.deviceTemplate; {
			case dev == nil:
				t.Fatal("No device template provided. This is likely a test setup mistake.")
			case dev.GetId() == "": // Create new device
				var err error
				created, err = devices.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
					Device:            dev,
					CreateEnrollToken: true,
				}.Build())
				if err != nil {
					t.Fatalf("CreateDevice failed: %v", err)
				}
				enrollToken = created.GetEnrollToken().GetToken()
			default: // Create new enrollment token
				token, err := devices.CreateDeviceEnrollToken(ctx, devicepb.CreateDeviceEnrollTokenRequest_builder{
					DeviceId: dev.GetId(),
				}.Build())
				if err != nil {
					t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
				}
				created = dev
				enrollToken = token.GetToken()
			}
			emitter.Reset()

			assertAudit := false
			defer func() {
				if assertAudit {
					assertEvents(t, emitter.Events(), test.wantAuditEvents)
					verifyUserTrustedDevice(t, emitter.Events())
				}
			}()

			// 1. Init.
			stream, err := devices.EnrollDevice(ctx)
			if err != nil {
				t.Fatalf("EnrollDevice failed: %v", err)
			}
			req := test.simulator.enrollRequest(created, enrollToken)
			if err := stream.Send(req); err != nil && !errors.Is(err, io.EOF) {
				t.Fatalf("init: Send failed: %v", err)
			}
			// If we got this far we should have audit logs in the end.
			assertAudit = true
			resp, err := stream.Recv()
			switch {
			case test.assertInitErr == nil && err == nil: // OK!
			case test.assertInitErr == nil && err != nil:
				t.Fatalf("init: Recv returned an unexpected error: %v", err)
			case !test.assertInitErr(err):
				t.Fatalf("init: assertInitErr failed, err=%v", err)
			}
			if err != nil {
				return // Stop here if init failed.
			}

			// Flow varies per OS after init.
			gotDev, err := test.simulator.handleEnrollStream(resp, stream, true /* testBehavior */)
			switch {
			case test.assertHandleErr == nil && err == nil: // OK!
			case test.assertHandleErr == nil && err != nil:
				t.Fatalf("handleOSStream returned an unexpected error: %v", err)
			case !test.assertHandleErr(err):
				t.Fatalf("handleOSStream: assertHandleErr failed, err=%v", err)
			}
			if err != nil {
				return // Stop here if handleOSStream failed.
			}
			if gotDev == nil {
				t.Fatal("EnrollDevice returned nil device on success")
				return // Make staticcheck happy.
			}

			// Verify that basic device fields are updated.
			if got, want := gotDev.GetUpdateTime().AsTime(), created.GetUpdateTime().AsTime(); !got.After(want) {
				t.Errorf("gotDev.UpdateTime=%v, want >%v", got, want)
			}

			wantDev := created
			wantDev.SetUpdateTime(gotDev.GetUpdateTime())
			wantDev.ClearEnrollToken() // token spent, also not expected here
			wantDev.SetEnrollStatus(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED)
			wantDev.SetCredential(test.simulator.wantCredential())
			wantDev.GetCredential().SetDeviceAttestationType(test.wantAttestationType)
			wantDev.SetCollectedData(nil) // not expected here
			wantDev.SetOwner(testenv.DefaultUser)
			if diff := cmp.Diff(wantDev, gotDev, protocmp.Transform()); diff != "" {
				t.Errorf("EnrollDevice mismatch (-want +got):\n%s", diff)
			}

			storedDev, err := devices.GetDevice(ctx, devicepb.GetDeviceRequest_builder{
				DeviceId: gotDev.GetId(),
			}.Build())
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}
			// TODO(noah): Assert collected data more thoroughly in tests.
			if collectedDataLen := len(storedDev.GetCollectedData()); collectedDataLen != 1 {
				t.Errorf("len(storedDev.CollectedData)=%d, want %d", collectedDataLen, 1)
			}
			storedCollectedData := storedDev.GetCollectedData()[0]
			if test.wantDCDTPMPlatformAttestation && !storedCollectedData.HasTpmPlatformAttestation() {
				t.Error("storedDev.CollectedData.TpmPlatformAttestation=nil, want non-nil")
			} else if !test.wantDCDTPMPlatformAttestation && storedCollectedData.HasTpmPlatformAttestation() {
				t.Errorf("storedDev.CollectedData.TpmPlatformAttestation=%v, want nil", storedCollectedData.GetTpmPlatformAttestation())
			}
			storedDev.SetCollectedData(nil)
			if diff := cmp.Diff(gotDev, storedDev, protocmp.Transform()); diff != "" {
				t.Errorf("GetDevice mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestService_EnrollDevice_rejectsInterceptedAutoEnrollToken(t *testing.T) {
	t.Parallel()

	// Prepare an authorizer and a set of users with the following powers:
	// - adminUser: logged in and has all necessary verbs
	// - autoEnrollUser: logged in but has no device verbs
	const adminUser = "llama"
	const autoEnrollUser = "alpaca"
	authorizer := newUserAwareAuthorizer(
		withKnownUsers(adminUser, autoEnrollUser),
		withAuthorizedUsers(adminUser),
	)

	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(authorizer),
		testenv.WithEmitter(emitter),
		testenv.WithAuthPreferenceSpec(types.AuthPreferenceSpecV2{
			DeviceTrust: &types.DeviceTrust{
				AutoEnroll: true,
			},
		}),
	)
	devices := env.DevicesClient
	ctx := context.Background()

	dev, err := devices.CreateDevice(contextWithUser(ctx, adminUser), devicepb.CreateDeviceRequest_builder{
		Device: devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama-auto",
		}.Build(),
	}.Build())
	require.NoError(t, err)

	// Mint the token the way the auto-enroll path does, recording autoEnrollUser
	// as the enrolling user, then pretend it was intercepted.
	token, err := devices.CreateDeviceEnrollToken(
		contextWithUser(ctx, autoEnrollUser),
		devicepb.CreateDeviceEnrollTokenRequest_builder{
			DeviceData: defaultCollectData(dev),
		}.Build())
	require.NoError(t, err)

	// A different authenticated user, adminUser, spends the token.
	sim := newMacOSSimulator(macOSBehavior{})
	cleanup, err := sim.setup()
	require.NoError(t, err)
	t.Cleanup(cleanup)

	emitter.Reset()
	stream, err := devices.EnrollDevice(contextWithUser(ctx, adminUser))
	require.NoError(t, err)
	req := sim.enrollRequest(dev, token.GetToken())
	if err := stream.Send(req); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("init: Send failed: %v", err)
	}

	_, err = stream.Recv()
	require.ErrorIs(t, err, &trace.AccessDeniedError{Message: "invalid device enrollment token"})

	gotEvents := emitter.Events()
	if assertEvents(t, gotEvents, []wantEvent{
		{
			Type:     events.DeviceEnrollEvent,
			Code:     events.DeviceEnrollCode,
			WantFail: true,
		},
	}) {
		event := gotEvents[0].(*apievents.DeviceEvent2)
		wantMessage := fmt.Sprintf(
			"enrollment token user mismatch (want %s, got %s)", autoEnrollUser, adminUser)
		if got := event.Status.UserMessage; got != wantMessage {
			t.Errorf("event.Status.UserMessage = %q, want %q", got, wantMessage)
		}
	}
}

func TestService_EnrollDevice_spendsAdminTokenAcrossUsers(t *testing.T) {
	t.Parallel()

	// Prepare an authorizer and a set of users with the following powers:
	// - adminUser: logged in and has all necessary verbs
	// - enrollUser: logged in and has all necessary verbs
	const adminUser = "llama"
	const enrollUser = "alpaca"
	authorizer := newUserAwareAuthorizer(
		withKnownUsers(adminUser, enrollUser),
		withAuthorizedUsers(adminUser, enrollUser),
	)

	env := testenv.NewUsingT(t, testenv.WithAuthorizer(authorizer))
	devices := env.DevicesClient
	ctx := context.Background()

	dev, err := devices.CreateDevice(contextWithUser(ctx, adminUser), devicepb.CreateDeviceRequest_builder{
		Device: devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama-admin",
		}.Build(),
	}.Build())
	require.NoError(t, err)

	// Admin-issued tokens (with just DeviceId) record no user, so a caller other
	// than the admin who minted the token can spend it. enrollUser completes the
	// enrollment with a token minted by adminUser.
	adminToken, err := devices.CreateDeviceEnrollToken(
		contextWithUser(ctx, adminUser),
		devicepb.CreateDeviceEnrollTokenRequest_builder{
			DeviceId: dev.GetId(),
		}.Build())
	require.NoError(t, err)

	sim := newMacOSSimulator(macOSBehavior{})
	cleanup, err := sim.setup()
	require.NoError(t, err)
	t.Cleanup(cleanup)

	stream, err := devices.EnrollDevice(contextWithUser(ctx, enrollUser))
	require.NoError(t, err)
	req := sim.enrollRequest(dev, adminToken.GetToken())
	if err := stream.Send(req); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("init: Send failed: %v", err)
	}
	resp, err := stream.Recv()
	require.NoError(t, err)
	enrolled, err := sim.handleEnrollStream(resp, stream, true /* testBehavior */)
	require.NoError(t, err)
	if got := enrolled.GetOwner(); got != enrollUser {
		t.Errorf("enrolled.Owner = %q, want %q", got, enrollUser)
	}
}

// TestService_EnrollDevice_ignoreUsageBasedLimits verifies that legacy
// device trust limits from old licenses no longer apply.
// (see https://github.com/gravitational/teleport.e/issues/7490)
func TestService_EnrollDevice_ignoreUsageBasedLimits(t *testing.T) {
	t.Parallel()

	const devicesLimit = 3
	env := testenv.NewUsingT(t, testenv.WithModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			IsUsageBasedBilling: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust:            {Enabled: true, Limit: devicesLimit},
				entitlements.MobileDeviceManagement: {Enabled: true},
			},
		},
	}))

	devices := env.DevicesClient
	ctx := context.Background()

	// 1. Register limit+2 devices. This is allowed.
	var allDevs []*devicepb.Device
	for i := range devicesLimit + 2 {
		dev, err := devices.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
			Device: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: fmt.Sprintf("dev-%v", i),
			}.Build(),
		}.Build())
		require.NoError(t, err, "CreateDevice failed")
		allDevs = append(allDevs, dev)
	}

	// 2. Attempt to enroll past the limit. This should pass because Enterprise users can now enroll unlimited devices.
	for i, dev := range allDevs {
		_, _, err := enrollDevice(ctx, devices, dev, defaultCollectData)
		require.NoError(t, err, "Device enrollment %d should succeed (limit is %d)", i+1, devicesLimit)
	}

	// 3. Verify all devices are enrolled.
	for _, dev := range allDevs {
		got, err := devices.GetDevice(ctx, devicepb.GetDeviceRequest_builder{DeviceId: dev.GetId()}.Build())
		require.NoError(t, err)
		require.Equal(t, devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED, got.GetEnrollStatus())
	}
}
