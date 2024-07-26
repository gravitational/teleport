package devicetrustv1_test

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"

	clientpb "github.com/gravitational/teleport/api/client/proto"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
)

func TestService_AuthenticateDevice(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(
		t,
		testenv.WithAugmentCertsFunc(fakeAugmentFunc),
		testenv.WithEmitter(emitter),
	)

	devices := env.DevicesClient
	ctx := context.Background()

	tests := []struct {
		name                          string
		shouldSkip                    string
		deviceTemplate                *devicepb.Device
		simulator                     simulator
		wantErr                       string
		wantDCDTPMPlatformAttestation bool
	}{
		{
			name: "macOS: success",
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "macos-success",
			},
			simulator: newMacOSSimulator(macOSBehavior{}),
		},
		{
			name:       "windows: success",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "windows-success",
			},
			simulator:                     newTPMSimulator(tpmBehavior{}),
			wantDCDTPMPlatformAttestation: true,
		},
		{
			name:       "linux: success",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_LINUX,
				AssetTag: "linux-success",
			},
			simulator: newTPMSimulator(tpmBehavior{
				emptyEventLog: true,
			}),
			wantDCDTPMPlatformAttestation: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Allowing skipping of tests on non-supported platforms.
			if test.shouldSkip != "" {
				t.Skip(test.shouldSkip)
			}

			// Allow underlying device mock to be set up
			cleanup, err := test.simulator.setup()
			if err != nil {
				t.Fatalf("Failed to setup device simulator: %v", err)
			}
			defer cleanup()

			// Create and then enroll the device to use for auth
			dev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
				Device:            test.deviceTemplate,
				CreateEnrollToken: true,
			})
			if err != nil {
				t.Fatalf("CreateDevice failed: %v", err)
			}
			enrolledDev, err := enrollSimulator(ctx, devices, test.simulator, dev)
			if err != nil {
				t.Fatalf("enrollSimulator failed: %v", err)
			}

			// Fetch the device before the ceremony so we can compare collected data
			// entries at the end.
			devBefore, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: enrolledDev.Id,
			})
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}

			emitter.Reset()

			initCerts := &devicepb.UserCertificates{
				X509Der:          []byte("ignored"), // mTLS cert takes its place.
				SshAuthorizedKey: []byte{1, 2, 3, 4, 6},
			}
			gotCerts, err := authenticateSimulator(ctx, devices, test.simulator, enrolledDev, initCerts)
			if err != nil {
				t.Fatalf("authenticateSimulator failed: %v", err)
			}

			// UserCertificates.
			if len(gotCerts.X509Der) == 0 {
				t.Error("Got empty X509Der, want non-empty")
			}
			if len(gotCerts.SshAuthorizedKey) == 0 {
				t.Error("Got empty SshAuthorizedKey, want non-empty")
			}

			// Extract the wanted certs from fakeAugmentFunc.
			certsProto, _ := fakeAugmentFunc(ctx, &authz.Context{}, &auth.AugmentUserCertificateOpts{
				SSHAuthorizedKey: initCerts.SshAuthorizedKey,
				DeviceExtensions: &auth.DeviceExtensions{
					DeviceID:     enrolledDev.Id,
					AssetTag:     enrolledDev.AssetTag,
					CredentialID: enrolledDev.Credential.Id,
				},
			})
			block, _ := pem.Decode(certsProto.TLS)
			if block == nil {
				t.Fatal("Failed to decode fakeAugmentFunc X.509 PEM")
				return // Make staticcheck happy.
			}
			wantCerts := &devicepb.UserCertificates{
				X509Der:          block.Bytes,
				SshAuthorizedKey: certsProto.SSH,
			}

			if diff := cmp.Diff(wantCerts, gotCerts, protocmp.Transform()); diff != "" {
				t.Errorf("AuthenticateDevice certificates mismatch (-want +got)\n%s", diff)
			}

			// Verify collected data recording.
			devAfter, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: enrolledDev.Id,
			})
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}
			if gotCD, wantCD := len(devAfter.CollectedData), len(devBefore.CollectedData)+1; gotCD != wantCD {
				t.Errorf("Got %v collected data instances, want %v", gotCD, wantCD)
			}
			// TODO(noah): Assert collected data more thoroughly in tests.
			authnCollectedData := devAfter.CollectedData[len(devAfter.CollectedData)-1]
			if test.wantDCDTPMPlatformAttestation && authnCollectedData.TpmPlatformAttestation == nil {
				t.Errorf("authnCollectedData.TpmPlatformAttestation=nil, want non-nil (authnCollectedData=%+v", authnCollectedData)
			} else if !test.wantDCDTPMPlatformAttestation && authnCollectedData.TpmPlatformAttestation != nil {
				t.Errorf("authnCollectedData.TpmPlatformAttestation=%v, want nil", authnCollectedData.TpmPlatformAttestation)
			}

			// Verify audit log.
			assertEvents(t, emitter.Events(), []wantEvent{
				{
					Type: events.DeviceAuthenticateEvent,
					Code: events.DeviceAuthenticateCode,
				},
			})
			verifyUserTrustedDevice(t, emitter.Events())
		})
	}
}

func TestService_AuthenticateDevice_errors(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(
		t,
		testenv.WithEmitter(emitter),
	)

	devices := env.DevicesClient
	ctx := context.Background()

	const invalidPayloadMessage = "initial payload"
	const deviceAuthnFailedMessage = "device authentication failed"

	tests := []struct {
		name       string
		shouldSkip string

		noEnroll       bool
		deviceTemplate *devicepb.Device
		simulator      simulator

		assertErr            func(error) bool
		wantErr              string
		wantAuditUserMessage string
	}{
		// Init step errors.
		{
			name: "init: CredentialId empty",
			simulator: newMacOSSimulator(macOSBehavior{
				modifyAuthenticateDeviceInit: func(r *devicepb.AuthenticateDeviceInit) {
					r.CredentialId = ""
				},
			}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "credential-id-empty",
			},
			assertErr:            trace.IsBadParameter,
			wantErr:              "credential ID",
			wantAuditUserMessage: invalidPayloadMessage,
		},
		{
			name: "init: CredentialId mismatch",
			simulator: newMacOSSimulator(macOSBehavior{
				modifyAuthenticateDeviceInit: func(r *devicepb.AuthenticateDeviceInit) {
					r.CredentialId = "different-credential-id"
				},
			}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "credential-id-unknown",
			},
			assertErr:            trace.IsBadParameter,
			wantErr:              "unknown device credential",
			wantAuditUserMessage: "unknown device credential",
		},
		{
			name: "init: DeviceData nil",
			simulator: newMacOSSimulator(macOSBehavior{
				modifyAuthenticateDeviceInit: func(r *devicepb.AuthenticateDeviceInit) {
					r.DeviceData = nil
				},
			}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "device-data-nil",
			},
			assertErr:            trace.IsBadParameter,
			wantErr:              "device data required",
			wantAuditUserMessage: invalidPayloadMessage,
		},
		{
			name: "init: DeviceData mismatch",
			simulator: newMacOSSimulator(macOSBehavior{
				modifyAuthenticateDeviceInit: func(r *devicepb.AuthenticateDeviceInit) {
					r.DeviceData.SerialNumber = "ceni n'est pas une serial number"
				},
			}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "device-data-mismatch",
			},
			assertErr:            trace.IsNotFound,
			wantErr:              "not registered",
			wantAuditUserMessage: "device not found",
		},
		{
			name: "init: device sends platform attestation in dcd",
			simulator: newMacOSSimulator(macOSBehavior{
				modifyAuthenticateDeviceInit: func(r *devicepb.AuthenticateDeviceInit) {
					r.DeviceData.TpmPlatformAttestation = &devicepb.TPMPlatformAttestation{
						Nonce: []byte("a-nonce"),
					}
				},
			}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "device-data-sends-platform-attestation",
			},
			assertErr:            trace.IsBadParameter,
			wantErr:              "tpm_platform_attestation is a read only field and cannot be submitted in device collected data",
			wantAuditUserMessage: invalidPayloadMessage,
		},

		{
			name:      "unenrolled device",
			noEnroll:  true,
			simulator: newMacOSSimulator(macOSBehavior{}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "unenrolled",
			},
			assertErr:            trace.IsBadParameter,
			wantErr:              "device not enrolled",
			wantAuditUserMessage: "device not enrolled",
		},

		// macOS specific errors.
		{
			name: "macos: Signature nil",
			simulator: newMacOSSimulator(macOSBehavior{
				nilSignature: true,
			}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "macos-nil-signature",
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "signature required",
		},
		{
			name: "macos: Signature invalid",
			simulator: newMacOSSimulator(macOSBehavior{
				incorrectSignature: true,
			}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "macos-invalid-signature",
			},
			assertErr:            trace.IsBadParameter,
			wantErr:              "verification failed",
			wantAuditUserMessage: deviceAuthnFailedMessage,
		},
		{
			name: "macos: wrong key signs the challenge",
			simulator: newMacOSSimulator(macOSBehavior{
				incorrectSigningKey: true,
			}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "macos-wrong-signing-key",
			},
			assertErr:            trace.IsBadParameter,
			wantErr:              "verification failed",
			wantAuditUserMessage: deviceAuthnFailedMessage,
		},
		// tpm specific errors.
		{
			name:       "tpm: incorrect platform attestation AK",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-incorrect-attest-ak",
			},
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestAK: true,
			}),
			assertErr:            trace.IsBadParameter,
			wantErr:              "platform attestation verification failed",
			wantAuditUserMessage: deviceAuthnFailedMessage,
		},
		{
			name:       "tpm: incorrect platform attestation nonce",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-incorrect-attest-nonce",
			},
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestNonce: true,
			}),
			assertErr:            trace.IsBadParameter,
			wantErr:              "platform attestation verification failed",
			wantAuditUserMessage: deviceAuthnFailedMessage,
		},
		{
			name:       "tpm: incorrect platform attestation pcr",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-incorrect-attest-pcr",
			},
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestPCR: true,
			}),
			assertErr:            trace.IsBadParameter,
			wantErr:              "platform attestation verification failed",
			wantAuditUserMessage: deviceAuthnFailedMessage,
		},
		{
			name:       "tpm: incorrect platform attestation event",
			shouldSkip: tpmSkip,
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "tpm-incorrect-attest-event",
			},
			simulator: newTPMSimulator(tpmBehavior{
				incorrectAttestEvent: true,
			}),
			assertErr:            trace.IsBadParameter,
			wantErr:              "platform attestation verification failed",
			wantAuditUserMessage: "event log verification",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Allowing skipping of tests on non-supported platforms.
			if test.shouldSkip != "" {
				t.Skip(test.shouldSkip)
			}

			// Allow underlying device mock to be set up
			cleanup, err := test.simulator.setup()
			if err != nil {
				t.Fatalf("Failed to setup device simulator: %v", err)
			}
			defer cleanup()

			// Create and then enroll the device to use for auth
			dev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
				Device:            test.deviceTemplate,
				CreateEnrollToken: true,
			})
			if err != nil {
				t.Fatalf("CreateDevice failed: %v", err)
			}
			if !test.noEnroll {
				dev, err = enrollSimulator(ctx, devices, test.simulator, dev)
				if err != nil {
					t.Fatalf("enrollSimulator failed: %v", err)
				}
			}
			emitter.Reset()

			_, err = authenticateSimulator(ctx, devices, test.simulator, dev, nil /* initCerts */)
			if !test.assertErr(err) {
				t.Errorf("AuthenticateDevice: assertErr failed, err=%v", err)
			}
			assert.ErrorContains(t, err, test.wantErr, "AuthenticateDevice error mismatch")

			// Verify audit log.
			assertEvents(t, emitter.Events(), []wantEvent{
				{
					Type:     events.DeviceAuthenticateEvent,
					Code:     events.DeviceAuthenticateCode,
					WantFail: true,
				},
			})

			// Verify audit Status.UserMessage.
			if test.wantAuditUserMessage == "" {
				return
			}
			var gotMessage string
			if devEvent, ok := emitter.LastEvent().(*apievents.DeviceEvent2); ok {
				gotMessage = devEvent.Status.UserMessage
			}
			if !strings.Contains(gotMessage, test.wantAuditUserMessage) {
				t.Errorf("Audit Status.UserMessage=%q, want %q", gotMessage, test.wantAuditUserMessage)
			}
		})
	}
}

func TestService_AuthenticateDevice_backfillOwner(t *testing.T) {
	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	identity := env.IdentityService
	ctx := context.Background()

	user, _ := types.NewUser(testenv.DefaultUser)
	if _, err := identity.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// legacyDev has no user assigned.
	legacyKey, err := newFakeEnclaveKey()
	if err != nil {
		t.Fatalf("newFakeEnclaveKey failed: %v", err)
	}
	legacyDev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			AssetTag:     "legacyNoOwner",
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED,
			Credential: &devicepb.DeviceCredential{
				Id:           legacyKey.id,
				PublicKeyDer: legacyKey.pubKeyDER,
			},
		},
		CreateAsResource: true,
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// enrolledDev has an owner, but the user is missing its TrustedDeviceIDs.
	enrolledDev, enrolledKey, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "enrolled1",
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	user, err = identity.UpdateAndSwapUser(ctx, user.GetName(), false /* withSecrets */, func(u types.User) (changed bool, err error) {
		u.SetTrustedDeviceIDs(nil)
		return true, nil
	})
	if err != nil {
		t.Fatalf("UpdateAndSwapUser failed: %v", err)
	}

	// missingIndex is missing the devicesByUser index.
	missingIndexKey, err := newFakeEnclaveKey()
	if err != nil {
		t.Fatalf("newFakeEnclaveKey failed: %v", err)
	}
	missingIndex, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:       devicepb.OSType_OS_TYPE_MACOS,
			AssetTag:     "missingIndex",
			EnrollStatus: devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED,
			Credential: &devicepb.DeviceCredential{
				Id:           missingIndexKey.id,
				PublicKeyDer: missingIndexKey.pubKeyDER,
			},
			Owner: user.GetName(),
		},
		CreateAsResource: true,
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	if _, err := identity.UpdateAndSwapUser(ctx, user.GetName(), false /* withSecrets */, func(u types.User) (changed bool, err error) {
		u.SetTrustedDeviceIDs(append(u.GetTrustedDeviceIDs(), missingIndex.Id))
		return true, nil
	}); err != nil {
		t.Fatalf("UpdateAndSwapUser failed: %v", err)
	}

	wantOwner := user.GetName()

	tests := []struct {
		name string
		dev  *devicepb.Device
		key  *fakeEnclaveKey
	}{
		{
			name: "legacyDev without owner",
			dev:  legacyDev,
			key:  legacyKey,
		},
		{
			name: "user missing trusted_device_ids",
			dev:  enrolledDev,
			key:  enrolledKey,
		},
		{
			name: "missing devicesByUser index",
			dev:  missingIndex,
			key:  missingIndexKey,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := authenticateSimulator(ctx, devices, test.key.simulator(), test.dev, nil /* initCerts */); err != nil {
				t.Fatalf("AuthenticateDevice failed: %v", err)
			}

			// Verify Device.Owner.
			deviceID := test.dev.Id
			storedDev, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: deviceID,
			})
			switch {
			case err != nil:
				t.Fatalf("GetDevice failed: %v", err)
			case storedDev.Owner != wantOwner:
				t.Errorf("AuthenticateDevice: Device owner not backfilled, got=%q, want %q", storedDev.Owner, wantOwner)
			}

			// Verify User.TrustedDeviceIDs.
			storedUser, err := identity.GetUser(ctx, wantOwner, false /* withSecrets */)
			switch {
			case err != nil:
				t.Fatalf("GetUser failed: %v", err)
			case !slices.Contains(storedUser.GetTrustedDeviceIDs(), deviceID):
				t.Errorf(
					"AuthenticateDevice: User %q trusted_device_ids not backfilled, got=%q, want %q",
					storedUser.GetName(), storedUser.GetTrustedDeviceIDs(), deviceID)
			}
		})
	}
}

func TestService_AuthenticateDevice_webAuthn(t *testing.T) {
	if tpmSkip != "" {
		t.Skip(tpmSkip) // tpmsimulator required for this test.
	}

	const userLlama = "llama"
	const userAlpaca = "alpaca"
	allUsers := []string{userLlama, userAlpaca}
	emitter := &keyedEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(&userAwareAuthorizer{
			knownUsers:      allUsers,
			authorizedUsers: allUsers,
		}),
		testenv.WithEmitter(emitter),
	)

	devicesClient := env.DevicesClient
	service := env.DevicesService
	ctx := context.Background()

	llamaData := setupUserForDeviceWebAuthn(t, env, setupUserWebAuthnOpts{
		user: userLlama,
		devices: []*devicepb.Device{
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama-1",
			},
			{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "llama-2",
			},
		},
	})
	t.Cleanup(llamaData.Close) // close simulators

	alpacaData := setupUserForDeviceWebAuthn(t, env, setupUserWebAuthnOpts{
		user: userAlpaca,
		devices: []*devicepb.Device{
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "alpaca-1",
			},
			// Switches owner to "llama" later.
			{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "alpaca-2",
			},
		},
	})
	t.Cleanup(alpacaData.Close) // close simulators

	// Change the Owner of the "alpaca-2" device by enrolling it to someone else.
	devAlpaca2 := &(alpacaData.devices[1])
	enrollToken, err := devicesClient.CreateDeviceEnrollToken(
		contextWithUser(ctx, llamaData.user),
		&devicepb.CreateDeviceEnrollTokenRequest{
			DeviceId: devAlpaca2.dev.Id,
		})
	if err != nil {
		t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
	}
	devAlpaca2.dev.EnrollToken = enrollToken
	devAlpaca2.dev, err = enrollSimulator(
		contextWithUser(ctx, llamaData.user), // takes ownership of the device
		devicesClient,
		devAlpaca2.sim,
		devAlpaca2.dev,
	)
	if err != nil {
		t.Fatalf("EnrollDevice failed: %v", err)
	}
	// Sanity check that the ownership change did work.
	if devAlpaca2.dev.Owner != llamaData.user {
		t.Fatalf("Device %q has an unexpected owner: %q", devAlpaca2.dev.AssetTag, devAlpaca2.dev.Owner)
	}

	type createTokenData struct {
		userAgent, clientIP, user string
	}

	type deviceAuthnData struct {
		dev *devicepb.Device
		sim simulator
	}

	type contextData struct {
		clientIP, user string
	}

	validTokenOpts := createTokenData{
		userAgent: sampleUserAgentMacOS,
		clientIP:  sampleIP,
		user:      llamaData.user,
	}
	validCtxData := contextData{
		clientIP: sampleIP,
		user:     llamaData.user,
	}
	validAuthnOpts := deviceAuthnData{
		dev: llamaData.device.dev,
		sim: llamaData.device.sim,
	}

	winTokenOpts := createTokenData{
		userAgent: sampleUserAgentWindows,
		clientIP:  sampleIP,
		user:      llamaData.user,
	}
	winAuthnData := deviceAuthnData{
		dev: llamaData.devices[1].dev,
		sim: llamaData.devices[1].sim,
	}

	const invalidTokenMessage = "invalid device web token"
	const invalidPayloadMessage = "initial payload"
	tests := []struct {
		name string

		token       createTokenData
		modifyToken func(*devicepb.DeviceWebToken) // may be nil

		ctx   contextData // used for the AuthenticateDevice ctx
		authn deviceAuthnData

		wantErr              string           // err returned to client
		wantAuditUserMessage string           // Status.UserMessage written to audit
		assertErr            func(error) bool // defaults to trace.IsAccessDenied on errors.
	}{
		{
			name:  "ok",
			token: validTokenOpts,
			ctx:   validCtxData,
			authn: validAuthnOpts,
		},
		{
			name:  "ok (TPM)",
			token: winTokenOpts,
			ctx:   validCtxData,
			authn: winAuthnData,
		},

		// BadParameter variations, likely a programmer error.
		{
			name:  "token has empty ID",
			token: validTokenOpts,
			modifyToken: func(token *devicepb.DeviceWebToken) {
				token.Id = ""
			},
			ctx:                  validCtxData,
			authn:                validAuthnOpts,
			wantErr:              "token ID required",
			wantAuditUserMessage: invalidPayloadMessage,
			assertErr:            trace.IsBadParameter,
		},
		{
			name:  "token has empty Token",
			token: validTokenOpts,
			modifyToken: func(token *devicepb.DeviceWebToken) {
				token.Token = ""
			},
			ctx:                  validCtxData,
			authn:                validAuthnOpts,
			wantErr:              "token required",
			wantAuditUserMessage: invalidPayloadMessage,
			assertErr:            trace.IsBadParameter,
		},

		// "invalid token" errors, aka various failed token checks.
		{
			name:  "invalid plaintext token",
			token: validTokenOpts,
			modifyToken: func(token *devicepb.DeviceWebToken) {
				token.Token = base64.RawURLEncoding.EncodeToString([]byte(`not a valid plaintext token`))
			},
			ctx:                  validCtxData,
			authn:                validAuthnOpts,
			wantErr:              invalidTokenMessage,
			wantAuditUserMessage: invalidTokenMessage,
		},
		{
			name:                 "fails expected device check (Windows vs macOS)",
			token:                winTokenOpts, // Windows
			ctx:                  validCtxData,
			authn:                validAuthnOpts, // macOS
			wantErr:              invalidTokenMessage,
			wantAuditUserMessage: "expected device mismatch",
		},
		{
			name:  "client has wrong IP",
			token: validTokenOpts,
			ctx: func() contextData {
				d := validCtxData
				d.clientIP = "142.251.129.206" // wrong!
				return d
			}(),
			authn:                validAuthnOpts,
			wantErr:              invalidTokenMessage,
			wantAuditUserMessage: "IP mismatch",
		},
		{
			name: "token issued for another user",
			token: createTokenData{
				userAgent: sampleUserAgentMacOS, // matches llama's device
				clientIP:  sampleIP,             // matches ctx IP
				user:      userAlpaca,
			},
			ctx:                  validCtxData,
			authn:                validAuthnOpts,
			wantErr:              "logged in user",
			wantAuditUserMessage: "user mismatch",
		},
		{
			name: "device has incorrect owner",
			token: createTokenData{
				userAgent: sampleUserAgentMacOS, // matches device
				clientIP:  sampleIP,
				user:      userAlpaca,
			},
			ctx: contextData{
				clientIP: sampleIP,
				user:     userAlpaca,
			},
			authn: deviceAuthnData{
				dev: alpacaData.devices[1].dev, // Owner changed to "llama".
				sim: alpacaData.devices[1].sim,
			},
			wantErr:              invalidTokenMessage,
			wantAuditUserMessage: "owner mismatch",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// We don't need an actual session here, only the Auth server reads it.
			webSessionID := uuid.NewString()

			// Create the DeviceWebToken, usually done by
			// auth.Server.AuthenticateWebUser.
			webToken, err := service.CreateDeviceWebToken(ctx, &devicepb.DeviceWebToken{
				WebSessionId:     webSessionID,
				BrowserUserAgent: test.token.userAgent,
				BrowserIp:        test.token.clientIP,
				User:             test.token.user,
			})
			if err != nil {
				t.Fatalf("CreateDeviceWebToken failed: %v", err)
			}
			if webToken == nil {
				t.Fatal("CreateDeviceWebToken returned a nil token")
			}

			if test.modifyToken != nil {
				test.modifyToken(webToken)
			}

			outCtx := configureOutgoingContext(context.Background(), outgoingContextParams{
				User:       test.ctx.user,
				SourceIP:   test.ctx.clientIP,
				EmitterKey: webSessionID,
			})

			// Authenticate.
			confirmToken, err := authenticateDeviceWeb(
				outCtx,
				devicesClient,
				test.authn.dev, test.authn.sim,
				webToken,
			)

			// Assert error type and message.
			assertErr := test.assertErr
			if err != nil && assertErr == nil {
				assertErr = trace.IsAccessDenied
			}
			if assertErr != nil && !assertErr(err) {
				t.Errorf("AuthenticateDevice: assertErr failed: err=%v (%T)", err, trace.Unwrap(err))
			}
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "AuthenticateDevice error mismatch")
			}
			if err != nil {
				// Assert audit failure.
				assertEvents(t, emitter.Events(webSessionID), []wantEvent{
					{
						Type:     events.DeviceAuthenticateEvent,
						Code:     events.DeviceAuthenticateCode,
						WantFail: true,
					},
				})

				// Assert audit UserMessage.
				lastEvent := emitter.LastEvent(webSessionID)
				var userMessage string
				if deviceEvent, ok := lastEvent.(*apievents.DeviceEvent2); ok {
					userMessage = deviceEvent.UserMessage
				}
				assert.Contains(t, userMessage, test.wantAuditUserMessage, "AuthenticateDevice: audit Status.UserMessage mismatch")

				return
			}

			// Assert non-empty token.
			if confirmToken.GetId() == "" || confirmToken.GetToken() == "" {
				t.Errorf("AuthenticateDevice returned invalid DeviceConfirmationToken: %v", confirmToken)
			}

			// Assert audit success.
			assertEvents(t, emitter.Events(webSessionID), []wantEvent{
				{
					Type: events.DeviceAuthenticateEvent,
					Code: events.DeviceAuthenticateCode,
				},
			})
		})
	}
}

type setupUserWebAuthnOpts struct {
	user    string
	devices []*devicepb.Device // Device templates. Creates a single macOS device if nil.
}

type deviceWithSim struct {
	dev    *devicepb.Device
	sim    simulator
	closer func()
}

type userWebAuthnData struct {
	user    string
	device  deviceWithSim   // first device in the list, for convenience
	devices []deviceWithSim // complete list of devices
}

func (u *userWebAuthnData) Close() {
	for _, d := range u.devices {
		if d.closer != nil {
			d.closer()
		}
	}
}

func setupUserForDeviceWebAuthn(t *testing.T, env *testenv.E, opts setupUserWebAuthnOpts) *userWebAuthnData {
	devicesClient := env.DevicesClient
	identity := env.IdentityService
	ctx := context.Background()

	// Create user.
	user := opts.user
	u, err := types.NewUser(user)
	if err != nil {
		t.Fatalf("NewUser(%q) failed: %v", user, err)
	}
	if _, err := identity.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser(%q) failed: %v", user, err)
	}
	userCtx := contextWithUser(ctx, user)

	var createdDevs []deviceWithSim
	for _, template := range opts.devices {
		created, err := devicesClient.CreateDevice(userCtx, &devicepb.CreateDeviceRequest{
			Device:            template,
			CreateEnrollToken: true,
		})
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}

		var sim simulator
		if created.OsType == devicepb.OSType_OS_TYPE_MACOS {
			sim = newMacOSSimulator(macOSBehavior{})
		} else {
			sim = newTPMSimulator(tpmBehavior{})
		}
		closer, err := sim.setup()
		if err != nil {
			t.Fatalf("sim.setup() failed: %v", err)
		}

		enrolled, err := enrollSimulator(userCtx, devicesClient, sim, created)
		if err != nil {
			t.Fatalf("EnrollDevice failed: %v", err)
		}

		createdDevs = append(createdDevs, deviceWithSim{
			dev:    enrolled,
			sim:    sim,
			closer: closer,
		})
	}

	var dev deviceWithSim
	if len(createdDevs) > 0 {
		dev = createdDevs[0]
	}

	return &userWebAuthnData{
		user:    user,
		device:  dev,
		devices: createdDevs,
	}
}

func authenticateDeviceWeb(
	ctx context.Context,
	devicesClient devicepb.DeviceTrustServiceClient,
	dev *devicepb.Device,
	sim simulator,
	webToken *devicepb.DeviceWebToken,
) (*devicepb.DeviceConfirmationToken, error) {
	stream, err := devicesClient.AuthenticateDevice(ctx)
	if err != nil {
		return nil, nil
	}

	resp, err := sim.authenticate(ctx, dev, stream, &devicepb.AuthenticateDeviceInit{
		UserCertificates: &devicepb.UserCertificates{
			X509Der:          []byte("ignored input"),
			SshAuthorizedKey: []byte("other ignored input"),
		},
		DeviceWebToken: webToken,
	})

	if err != nil {
		return nil, err
	}

	// Assert payload type. We want a DeviceConfirmationToken payload as reply.
	confirmToken := resp.GetConfirmationToken()
	if confirmToken == nil {
		return nil, fmt.Errorf("unexpected AuthenticateDevice response payload, got=%T, want DeviceConfirmationToken", resp.GetPayload())
	}

	return resp.GetConfirmationToken(), nil
}

func fakeAugmentFunc(_ context.Context, authCtx *authz.Context, opts *auth.AugmentUserCertificateOpts) (*clientpb.Certs, error) {
	// Sanity checks.
	switch {
	case authCtx == nil:
		return nil, errors.New("authCtx required")
	case opts == nil:
		return nil, errors.New("opts required")
	case opts.DeviceExtensions == nil:
		return nil, errors.New("opts.DeviceExtensions required")
	case opts.DeviceExtensions.DeviceID == "":
		return nil, errors.New("opts.DeviceExtensions.DeviceID required")
	case opts.DeviceExtensions.AssetTag == "":
		return nil, errors.New("opts.DeviceExtensions.AssetTag required")
	case opts.DeviceExtensions.CredentialID == "":
		return nil, errors.New("opts.DeviceExtensions.CredentialID required")
	}

	sshCert := opts.SSHAuthorizedKey
	if sshCert != nil {
		sshCert = append(sshCert, 9)
	}

	// "Build" a fake TLS cert that includes device extension data.
	// This is a roundabout way to make sure the server is passing in the correct
	// information.
	ext := opts.DeviceExtensions
	tlsCert := fmt.Sprintf(""+
		"<stand in for TLS cert:"+
		"\n\tid=%v"+
		"\n\tasset=%v"+
		"\n\tcredential=%v>",
		ext.DeviceID, ext.AssetTag, ext.CredentialID)

	return &clientpb.Certs{
		SSH: sshCert,
		TLS: pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: []byte(tlsCert),
		}),
	}, nil
}
