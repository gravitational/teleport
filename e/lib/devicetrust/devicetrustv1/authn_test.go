package devicetrustv1_test

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	emitter := &eventstest.MockEmitter{}
	env := testenv.NewUsingT(
		t,
		testenv.WithAugmentCertsFunc(fakeAugmentFunc),
		testenv.WithEmitter(emitter),
	)

	devices := env.DevicesClient
	ctx := context.Background()

	macOSDev, macOSKey, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}

	tests := []struct {
		name string
		dev  *devicepb.Device
		key  *fakeEnclaveKey
	}{
		{
			name: "macOS device",
			dev:  macOSDev,
			key:  macOSKey,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emitter.Reset()

			// Fetch the device before the ceremony so we can compare collected data
			// entries at the end.
			devBefore, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: test.dev.Id,
			})
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}

			stream, err := devices.AuthenticateDevice(ctx)
			if err != nil {
				t.Fatalf("AuthenticateDevice failed: %v", err)
			}

			// 1. Init.
			initCerts := &devicepb.UserCertificates{
				X509Der:          []byte("ignored"), // mTLS cert takes its place.
				SshAuthorizedKey: []byte{1, 2, 3, 4, 6},
			}
			if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
				Payload: &devicepb.AuthenticateDeviceRequest_Init{
					Init: &devicepb.AuthenticateDeviceInit{
						UserCertificates: initCerts,
						CredentialId:     test.key.id,
						DeviceData: &devicepb.DeviceCollectedData{
							CollectTime:  timestamppb.Now(),
							OsType:       test.dev.OsType,
							SerialNumber: test.dev.AssetTag,
						},
					},
				},
			}); err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			resp, err := stream.Recv()
			if err != nil {
				t.Fatalf("init: Recv failed: %v", err)
			}

			// 2. Challenge.
			chalResp := resp.GetChallenge()
			if chalResp == nil {
				t.Fatalf("Got unexpected payload=%T, want AuthenticateDeviceChallenge ", resp.Payload)
			}
			sig, err := test.key.signChallenge(chalResp.Challenge)
			if err != nil {
				t.Fatalf("signChallenge failed: %v", err)
			}
			if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
				Payload: &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
					ChallengeResponse: &devicepb.AuthenticateDeviceChallengeResponse{
						Signature: sig,
					},
				},
			}); err != nil {
				t.Fatalf("Send failed: %v", err)
			}
			resp, err = stream.Recv()
			if err != nil {
				t.Fatalf("challenge: Recv failed: %v", err)
			}

			// 3. UserCertificates.
			gotCerts := resp.GetUserCertificates()
			if gotCerts == nil {
				t.Fatalf("Got unexpected payload=%T, want UserCertificates", resp.Payload)
			}
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
					DeviceID:     test.dev.Id,
					AssetTag:     test.dev.AssetTag,
					CredentialID: test.dev.Credential.Id,
				},
			})
			block, _ := pem.Decode(certsProto.TLS)
			if block == nil {
				t.Fatal("Failed to decode fakeAugmentFunc X.509 PEM")
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
				DeviceId: test.dev.Id,
			})
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}
			if gotCD, wantCD := len(devAfter.CollectedData), len(devBefore.CollectedData)+1; gotCD != wantCD {
				t.Errorf("Got %v collected data instances, want %v", gotCD, wantCD)
			}

			// Verify audit log.
			assertEventsEmitter(t, emitter, []wantEvent{
				{
					Type: events.DeviceEvent,
					Code: events.DeviceAuthenticateCode,
				},
			})
		})
	}
}

func TestService_AuthenticateDevice_errors(t *testing.T) {
	emitter := &eventstest.MockEmitter{}
	env := testenv.NewUsingT(
		t,
		testenv.WithEmitter(emitter),
	)

	devices := env.DevicesClient
	ctx := context.Background()

	// Create an enrolled and a plain device for tests.
	dev1, key1, err := createAndEnroll(ctx, devices, &devicepb.Device{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "llama",
	})
	if err != nil {
		t.Fatalf("createAndEnroll failed: %v", err)
	}
	notEnrolled, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "alpaca",
		},
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	// failKey is used for various failure scenarios.
	failKey, err := newFakeEnclaveKey()
	if err != nil {
		t.Fatalf("newFakeEnclaveKey failed: %v", err)
	}

	validInit := func(dev *devicepb.Device, key *fakeEnclaveKey) func() *devicepb.AuthenticateDeviceInit {
		return func() *devicepb.AuthenticateDeviceInit {
			return &devicepb.AuthenticateDeviceInit{
				UserCertificates: nil, // only mTLS cert is augmented.
				CredentialId:     key.id,
				DeviceData: &devicepb.DeviceCollectedData{
					CollectTime:  timestamppb.Now(),
					OsType:       dev.OsType,
					SerialNumber: dev.AssetTag,
				},
			}
		}
	}
	validChallenge := func(sig []byte) *devicepb.AuthenticateDeviceChallengeResponse {
		return &devicepb.AuthenticateDeviceChallengeResponse{
			Signature: sig,
		}
	}

	tests := []struct {
		name                string
		dev                 *devicepb.Device
		key                 *fakeEnclaveKey
		createInitReq       func() *devicepb.AuthenticateDeviceInit
		createChallengeResp func(sig []byte) *devicepb.AuthenticateDeviceChallengeResponse
		assertErr           func(error) bool
		wantErr             string
	}{
		// Init step errors.
		{
			name: "init: CredentialId empty",
			dev:  dev1,
			key:  key1,
			createInitReq: func() *devicepb.AuthenticateDeviceInit {
				init := validInit(dev1, key1)()
				init.CredentialId = ""
				return init
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "credential ID",
		},
		{
			name: "init: CredentialId mismatch",
			dev:  dev1,
			key:  key1,
			createInitReq: func() *devicepb.AuthenticateDeviceInit {
				init := validInit(dev1, key1)()
				init.CredentialId = failKey.id
				return init
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "unknown device credential",
		},
		{
			name: "init: DeviceData nil",
			dev:  dev1,
			key:  key1,
			createInitReq: func() *devicepb.AuthenticateDeviceInit {
				init := validInit(dev1, key1)()
				init.DeviceData = nil
				return init
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "device data required",
		},
		{
			name: "init: DeviceData mismatch",
			dev:  dev1,
			key:  key1,
			createInitReq: func() *devicepb.AuthenticateDeviceInit {
				init := validInit(dev1, key1)()
				init.DeviceData.SerialNumber = "ceni n'est pas une serial number"
				return init
			},
			assertErr: trace.IsNotFound,
			wantErr:   "not registered",
		},

		// Challenge step errors.
		{
			name:          "challenge: Signature nil",
			dev:           dev1,
			key:           key1,
			createInitReq: validInit(dev1, key1),
			createChallengeResp: func(sig []byte) *devicepb.AuthenticateDeviceChallengeResponse {
				return &devicepb.AuthenticateDeviceChallengeResponse{
					Signature: nil,
				}
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "signature required",
		},
		{
			name:          "challenge: Signature invalid",
			dev:           dev1,
			key:           key1,
			createInitReq: validInit(dev1, key1),
			createChallengeResp: func(sig []byte) *devicepb.AuthenticateDeviceChallengeResponse {
				return &devicepb.AuthenticateDeviceChallengeResponse{
					Signature: []byte("not a signature"),
				}
			},
			assertErr: trace.IsBadParameter,
			wantErr:   "verification failed",
		},

		{
			name:          "unenrolled device",
			dev:           notEnrolled,
			key:           failKey,
			createInitReq: validInit(notEnrolled, failKey),
			assertErr:     trace.IsBadParameter,
			wantErr:       "device not enrolled",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			emitter.Reset()

			if test.createChallengeResp == nil {
				test.createChallengeResp = validChallenge
			}

			authenticate := func() error {
				stream, err := devices.AuthenticateDevice(ctx)
				if err != nil {
					return err
				}

				// 1. Init.
				if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
					Payload: &devicepb.AuthenticateDeviceRequest_Init{
						Init: test.createInitReq(),
					},
				}); err != nil {
					return err
				}
				resp, err := stream.Recv()
				if err != nil {
					return err
				}

				chalResp := resp.GetChallenge()
				sig, err := test.key.signChallenge(chalResp.Challenge)
				if err != nil {
					return err
				}
				if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
					Payload: &devicepb.AuthenticateDeviceRequest_ChallengeResponse{
						ChallengeResponse: test.createChallengeResp(sig),
					},
				}); err != nil {
					return err
				}
				_, err = stream.Recv()
				return err
			}

			err := authenticate()
			if !test.assertErr(err) {
				t.Errorf("AuthenticateDevice: assertErr failed, err=%v", err)
			}
			assert.ErrorContains(t, err, test.wantErr, "AuthenticateDevice error mismatch")

			// Verify audit log.
			assertEventsEmitter(t, emitter, []wantEvent{
				{
					Type:     events.DeviceEvent,
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
	emitter := &eventstest.MockEmitter{}
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
	// Wait for the Create, CreateToken and Enroll audit events.
	require.Eventually(
		t,
		func() bool { return len(emitter.Events()) >= 3 },
		2*time.Second, 10*time.Millisecond,
		"Failed to drain createAndEnroll audit events")
	emitter.Reset()

	// authenticate wraps the AuthenticateDevice logic so error handling is
	// simpler below.
	// It specifically relies on dev1, key1 and stops at the "init" step (which is
	// expected to fail).
	authenticate := func() error {
		stream, err := devices.AuthenticateDevice(ctx)
		if err != nil {
			return err
		}

		// 1. Init.
		if err := stream.Send(&devicepb.AuthenticateDeviceRequest{
			Payload: &devicepb.AuthenticateDeviceRequest_Init{
				Init: &devicepb.AuthenticateDeviceInit{
					UserCertificates: nil,
					CredentialId:     key1.id,
					DeviceData: &devicepb.DeviceCollectedData{
						CollectTime:  timestamppb.Now(),
						OsType:       dev1.OsType,
						SerialNumber: dev1.AssetTag,
					},
				},
			},
		}); err != nil {
			return err
		}

		// 2. Challenge.
		// Authn fails here for mode="off".
		resp, err := stream.Recv()
		if err != nil {
			return err
		}
		chalResp := resp.GetChallenge()
		sig, err := key1.signChallenge(chalResp.GetChallenge())
		if err != nil {
			t.Errorf("signChallenge returned err=%v, this is likely unexpected", err)
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

		// 3. Success.
		_, err = stream.Recv()
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
			defer emitter.Reset()

			// Test!
			err := authenticate()

			// Success scenario assertions.
			if test.wantSuccess {
				if err != nil {
					t.Errorf("AuthenticateDevice returned err=%v, want nil", err)
				}
				// See if issued audit events, but don't test the specifics here - these
				// are tested elsewhere.
				assert.Eventually(
					t,
					func() bool { return len(emitter.Events()) > 0 },
					2*time.Second, 10*time.Millisecond,
					"AuthenticateDevice issued 0 audit events, want >0")
				return
			}

			// Failure assertions.
			if !trace.IsBadParameter(err) {
				t.Fatalf("AuthenticateDevice returned err = %v (%T), want trace.BadParameterError", err, err)
			}
			assert.ErrorContains(t, err, "device trust disabled", "AuthenticateDevice error mismatch")

			// Assert no audit noise.
			if events := emitter.Events(); len(events) > 0 {
				t.Errorf("AuthenticateDevice issued unexpected audit events: %v, want no events", events)
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
