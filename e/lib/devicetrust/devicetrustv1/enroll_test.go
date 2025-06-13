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
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/modules"
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
	macOSFailDev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "fail",
		},
		CreateEnrollToken: true,
	})
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
					r.Token = "invalid"
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
					r.CredentialId = ""
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
					r.DeviceData = nil
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
					r.DeviceData.CollectTime = nil
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
					r.DeviceData.OsType = devicepb.OSType_OS_TYPE_UNSPECIFIED
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
					r.DeviceData.OsType = devicepb.OSType_OS_TYPE_LINUX
				},
			}),
			assertInitErr:   trace.IsNotFound,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: empty DeviceData.SerialNumber",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.DeviceData.SerialNumber = ""
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
					r.DeviceData.SerialNumber = "unknown"
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
					r.DeviceData.TpmPlatformAttestation = &devicepb.TPMPlatformAttestation{
						Nonce: []byte("a-nonce"),
					}
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		// Windows
		{
			name:       "windows: success with EKPub",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "llama",
			},
			simulator:                     newTPMSimulator(tpmBehavior{}),
			wantAuditEvents:               wantEnrollSuccess,
			wantDCDTPMPlatformAttestation: true,
			wantAttestationType:           devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB,
		},
		{
			name:       "windows: success with EKCert",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "llama-ekcert",
			},
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
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "llama-ekcert-trusted",
			},
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
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_LINUX,
				AssetTag: "linux-tpm-success",
			},
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
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-no-payload",
			},
			simulator: newTPMSimulator(tpmBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.Tpm = nil
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: no attest params in TPM payload",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-no-attest-params",
			},
			simulator: newTPMSimulator(tpmBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.Tpm.AttestationParameters = nil
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: no EK in TPM payload",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-no-ek",
			},
			simulator: newTPMSimulator(tpmBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.Tpm.Ek = nil
				},
			}),
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: incorrect platform attestation AK",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-incorrect-attest-ak",
			},
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestAK: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: incorrect platform attestation nonce",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-incorrect-attest-nonce",
			},
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestNonce: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: incorrect credential activation solution",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-incorrect-activation-solution",
			},
			simulator: newTPMSimulator(tpmBehavior{
				incorrectCredActivateSolution: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: incorrect platform attestation pcr",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-incorrect-attest-pcr",
			},
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestPCR: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: incorrect platform attestation event",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-incorrect-attest-event",
			},
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestEvent: true,
			}),
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:       "tpm: ekpub provided when allowed_ekcert_cas configured",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-ekpub-allowed-ekcert-cas",
			},
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
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-fail-ekcert-unrecognized-ca",
			},
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
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama",
			},
			simulator:       newMacOSSimulator(macOSBehavior{}),
			wantAuditEvents: wantEnrollSuccess,
		},
		{
			name:           "macOS: nil Macos payload",
			deviceTemplate: macOSFailDev,
			simulator: newMacOSSimulator(macOSBehavior{
				modifyEnrollDeviceInit: func(r *devicepb.EnrollDeviceInit) {
					r.Macos = nil
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
					r.Macos.PublicKeyDer = []byte{}
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
					r.Macos.PublicKeyDer = []byte("not a public key")
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
					r.Macos.PublicKeyDer = rsaKeyDER // Enclave keys are ECDSA.
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
					r.Macos.PublicKeyDer = p224KeyDER // Enclave keys are P-256.
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
			case dev.Id == "": // Create new device
				var err error
				created, err = devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
					Device:            dev,
					CreateEnrollToken: true,
				})
				if err != nil {
					t.Fatalf("CreateDevice failed: %v", err)
				}
				enrollToken = created.EnrollToken.Token
			default: // Create new enrollment token
				token, err := devices.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
					DeviceId: dev.Id,
				})
				if err != nil {
					t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
				}
				created = dev
				enrollToken = token.Token
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
			if got, want := gotDev.UpdateTime.AsTime(), created.UpdateTime.AsTime(); !got.After(want) {
				t.Errorf("gotDev.UpdateTime=%v, want >%v", got, want)
			}

			wantDev := created
			wantDev.UpdateTime = gotDev.UpdateTime
			wantDev.EnrollToken = nil // token spent, also not expected here
			wantDev.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED
			wantDev.Credential = test.simulator.wantCredential()
			wantDev.Credential.DeviceAttestationType = test.wantAttestationType
			wantDev.CollectedData = nil // not expected here
			wantDev.Owner = testenv.DefaultUser
			if diff := cmp.Diff(wantDev, gotDev, protocmp.Transform()); diff != "" {
				t.Errorf("EnrollDevice mismatch (-want +got):\n%s", diff)
			}

			storedDev, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: gotDev.Id,
			})
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}
			// TODO(noah): Assert collected data more thoroughly in tests.
			if collectedDataLen := len(storedDev.CollectedData); collectedDataLen != 1 {
				t.Errorf("len(storedDev.CollectedData)=%d, want %d", collectedDataLen, 1)
			}
			storedCollectedData := storedDev.CollectedData[0]
			if test.wantDCDTPMPlatformAttestation && storedCollectedData.TpmPlatformAttestation == nil {
				t.Error("storedDev.CollectedData.TpmPlatformAttestation=nil, want non-nil")
			} else if !test.wantDCDTPMPlatformAttestation && storedCollectedData.TpmPlatformAttestation != nil {
				t.Errorf("storedDev.CollectedData.TpmPlatformAttestation=%v, want nil", storedCollectedData.TpmPlatformAttestation)
			}
			storedDev.CollectedData = nil
			if diff := cmp.Diff(gotDev, storedDev, protocmp.Transform()); diff != "" {
				t.Errorf("GetDevice mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestService_EnrollDevice_usageBasedLimits(t *testing.T) {
	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	ctx := context.Background()

	// Set usage-based and device limits.
	// This is safe to do because NewUsingT sets modules.TestModules when called.
	// We'll also rely on the already-registered cleanup.
	m := modules.GetModules().(*modules.TestModules)
	m.TestFeatures.IsUsageBasedBilling = true
	const devicesLimit = 3
	m.TestFeatures.Entitlements[entitlements.DeviceTrust] = modules.EntitlementInfo{Enabled: true, Limit: devicesLimit}
	modules.SetModules(m)

	// 1. Register limit+1 devices. This is allowed.
	var allDevs []*devicepb.Device
	for i := range devicesLimit + 1 {
		dev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
			Device: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: fmt.Sprintf("dev-%v", i),
			},
		})
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		allDevs = append(allDevs, dev)
	}

	enrollSuccess := func(t *testing.T, dev *devicepb.Device) {
		t.Helper()
		if _, _, err := enrollDevice(ctx, devices, dev, defaultCollectData); err != nil {
			t.Errorf("enrollDevice returned err=%v, want success", err)
		}
	}
	enrollLimitFailure := func(t *testing.T, dev *devicepb.Device) {
		t.Helper()
		if _, _, err := enrollDevice(ctx, devices, dev, defaultCollectData); !trace.IsAccessDenied(err) {
			t.Errorf("enrollDevice returned err=%v, want AccessDenied/device limit failure", err)
		}
	}

	// 2. Enroll limit devices.
	for _, dev := range allDevs[:devicesLimit] {
		enrollSuccess(t, dev)
	}

	// 3. Attempt to enroll past the limit.
	lastDev := allDevs[devicesLimit]
	enrollLimitFailure(t, lastDev)

	// 4. Going below the limit allows further enrollments.
	firstDev := allDevs[0]
	if _, err := devices.UpdateDevice(ctx, &devicepb.UpdateDeviceRequest{
		Device: &devicepb.Device{
			Id:           firstDev.Id,
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED,
		},
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: []string{"enroll_status"}, // unenroll device
		},
	}); err != nil {
		t.Fatalf("UpdateDevice failed: %v", err)
	}
	enrollSuccess(t, lastDev)       // allowed, below limit
	enrollLimitFailure(t, firstDev) // limits applied
}
