package devicetrustv1_test

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
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

	tests := []struct {
		name       string
		shouldSkip string

		noEnroll       bool
		deviceTemplate *devicepb.Device
		simulator      simulator

		assertErr func(error) bool
		wantErr   string
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
			assertErr: trace.IsBadParameter,
			wantErr:   "credential ID",
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
			assertErr: trace.IsBadParameter,
			wantErr:   "unknown device credential",
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
			assertErr: trace.IsBadParameter,
			wantErr:   "device data required",
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
			assertErr: trace.IsNotFound,
			wantErr:   "not registered",
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
			assertErr: trace.IsBadParameter,
			wantErr:   "tpm_platform_attestation is a read only field and cannot be submitted in device collected data",
		},

		{
			name:      "unenrolled device",
			noEnroll:  true,
			simulator: newMacOSSimulator(macOSBehavior{}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "unenrolled",
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "device not enrolled",
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
			assertErr: trace.IsBadParameter,
			wantErr:   "verification failed",
		},
		{
			name: "macOS: wrong key signs the challenge",
			simulator: newMacOSSimulator(macOSBehavior{
				incorrectSigningKey: true,
			}),
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "macos-wrong-signing-key",
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "verification failed",
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
			assertErr: trace.IsBadParameter,
			wantErr:   "platform attestation verification failed",
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
			assertErr: trace.IsBadParameter,
			wantErr:   "platform attestation verification failed",
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
			assertErr: trace.IsBadParameter,
			wantErr:   "platform attestation verification failed",
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
			assertErr: trace.IsBadParameter,
			wantErr:   "platform attestation verification failed",
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
		})
	}
}

type fakeRolesChecker struct {
	testenv.NoopChecker
	roles []types.Role
}

func (c *fakeRolesChecker) HasRole(name string) bool {
	for _, role := range c.roles {
		if role.GetName() == name {
			return true
		}
	}
	return false
}

func (c *fakeRolesChecker) RoleNames() []string {
	names := make([]string, len(c.roles))
	for i, role := range c.roles {
		names[i] = role.GetName()
	}
	return names
}

func (c *fakeRolesChecker) Roles() []types.Role {
	return c.roles
}

func TestService_AuthenticateDevice_deviceModeOff(t *testing.T) {
	checker := &fakeRolesChecker{}
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(
		t,
		testenv.WithAuthPreferenceSpec(types.AuthPreferenceSpecV2{
			DeviceTrust: &types.DeviceTrust{
				Mode: constants.DeviceTrustModeOff,
			},
		}),
		testenv.WithAuthorizer(&fakeAuthorizer{
			Checker: checker,
		}),
		testenv.WithEmitter(emitter),
	)

	devices := env.DevicesClient
	ctx := context.Background()

	// Create an enrolled device for testing.
	dev1, key1, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	authenticate := func() error {
		_, err := authenticateSimulator(ctx, devices, key1.simulator(), dev1, nil /* initCerts */)
		return err
	}

	// Define a few roles with reasonable-looking allow rules for the following
	// tests.
	// Only the DeviceTrustMode matters for the tests.
	allowRule := types.RoleConditions{
		Logins: []string{"llama"},
		NodeLabels: map[string]utils.Strings{
			"env": {"dev"},
		},
	}
	var allRoles []types.Role
	for _, mode := range []string{
		"", // Empty means "off" for roles.
		constants.DeviceTrustModeOff,
		constants.DeviceTrustModeOptional,
		constants.DeviceTrustModeRequired,
	} {
		role, err := types.NewRole(fmt.Sprintf("mode=%v", mode), types.RoleSpecV6{
			Options: types.RoleOptions{
				DeviceTrustMode: mode,
			},
			Allow: allowRule,
		})
		if err != nil {
			t.Fatalf("NewRole failed: %v", err)
		}
		allRoles = append(allRoles, role)
	}
	modeEmptyRole := allRoles[0]
	modeOffRole := allRoles[1]
	modeOptionalRole := allRoles[2]
	modeRequiredRole := allRoles[3]

	tests := []struct {
		name        string
		roles       []types.Role
		wantSuccess bool
	}{
		{
			name:        "authn not allowed by cluster, empty roles",
			wantSuccess: false,
		},
		{
			name:        "authn not allowed by cluster or roles",
			roles:       []types.Role{modeEmptyRole, modeOffRole},
			wantSuccess: false,
		},
		{
			name:        "authn allowed by mode=optional role",
			roles:       []types.Role{modeEmptyRole, modeOffRole, modeOptionalRole},
			wantSuccess: true,
		},
		{
			name:        "authn allowed by mode=required role",
			roles:       []types.Role{modeEmptyRole, modeOffRole, modeRequiredRole},
			wantSuccess: true,
		},
		{
			name:        "authn allowed by multiple roles",
			roles:       allRoles,
			wantSuccess: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checker.roles = test.roles
			emitter.Reset()

			// Test!
			const maxAttempts = 3
			var err error
			for i := 0; i < maxAttempts; i++ {
				err = authenticate()
				// Sometimes authenticate fails with a mysterious `io.EOF`, retry if
				// that's the case.
				if errors.Is(err, io.EOF) {
					t.Logf("Got EOF from authenticate, retrying: %q", err)
					continue
				}
				break
			}

			// Success scenario assertions.
			if test.wantSuccess {
				if err != nil {
					t.Errorf("AuthenticateDevice returned err=%v, want nil", err)
				}
				// See if issued audit events, but don't test the specifics here - these
				// are tested elsewhere.
				if len(emitter.Events()) < 1 {
					t.Error("AuthenticateDevice issued 0 audit events, want >0")
				}
				return
			}

			// Failure assertions.
			if !trace.IsBadParameter(err) {
				t.Fatalf("AuthenticateDevice returned err = %q (%T), want trace.BadParameterError", err, err)
			}
			assert.ErrorContains(t, err, "device trust disabled", "AuthenticateDevice error mismatch")

			// Assert no audit noise.
			if events := emitter.Events(); len(events) > 0 {
				t.Errorf("AuthenticateDevice issued unexpected audit events: %v, want no events", events)
			}
		})
	}
}

func TestService_AuthenticateDevice_backfillOwner(t *testing.T) {
	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	identity := env.IdentityService
	ctx := context.Background()

	legacyKey, err := newFakeEnclaveKey()
	if err != nil {
		t.Fatalf("newFakeEnclaveKey failed: %v", err)
	}

	user, _ := types.NewUser(testenv.DefaultUser)
	if _, err := identity.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// legacyDev has no user assigned.
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

	// enrolledDev has an owner, but we'll update the user and remove its trusted
	// device ID
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := authenticateSimulator(ctx, devices, test.key.simulator(), test.dev, nil /* initCerts */); err != nil {
				t.Fatalf("AuthenticateDevice failed: %v", err)
			}

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

func fakeAugmentFunc(ctx context.Context, authCtx *authz.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error) {
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

	return &proto.Certs{
		SSH: sshCert,
		TLS: pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: []byte(tlsCert),
		}),
	}, nil
}
