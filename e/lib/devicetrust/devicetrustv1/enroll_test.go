package devicetrustv1_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
)

func TestService_EnrollDevice(t *testing.T) {
	emitter := &eventstest.MockEmitter{}
	env := testenv.MustNew(testenv.WithEmitter(emitter))
	defer env.Close()

	devices := env.DevicesClient
	ctx := context.Background()

	// failDev is used for failure test scenarios.
	failDev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "fail",
		},
		CreateEnrollToken: true,
	})
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}

	macOSKey1, err := newFakeEnclaveKey()
	if err != nil {
		t.Fatalf("newFakeEnclaveKey failed: %v", err)
	}
	// macOSKeyFail is used for failure test scenarios.
	macOSKeyFail, err := newFakeEnclaveKey()
	if err != nil {
		t.Fatalf("newFakeEnclaveKey failed: %v", err)
	}

	// Create a few "wrong" keys:
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
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048 /* bits */)
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	rsaKeyDER, err := x509.MarshalPKIXPublicKey(rsaKey.Public())
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey failed: %v", err)
	}

	// macOSInitFunc creates the initial enrollment request for a macOS device.
	newMacOSInitFunc := func(key *fakeEnclaveKey) func(dev *devicepb.Device) *devicepb.EnrollDeviceRequest {
		return func(dev *devicepb.Device) *devicepb.EnrollDeviceRequest {
			return &devicepb.EnrollDeviceRequest{
				Payload: &devicepb.EnrollDeviceRequest_Init{
					Init: &devicepb.EnrollDeviceInit{
						Token:        dev.EnrollToken.GetToken(),
						CredentialId: key.id,
						DeviceData: &devicepb.DeviceCollectedData{
							CollectTime:  timestamppb.Now(),
							OsType:       devicepb.OSType_OS_TYPE_MACOS,
							SerialNumber: dev.AssetTag,
						},
						Macos: &devicepb.MacOSEnrollPayload{
							PublicKeyDer: key.pubKeyDER,
						},
					},
				},
			}
		}
	}
	// newMacOSHandleFunc handles successful macOS registrations, starting from
	// the 2nd step.
	newMacOSHandleFunc := func(key *fakeEnclaveKey) func(resp *devicepb.EnrollDeviceResponse, stream devicepb.DeviceTrustService_EnrollDeviceClient) (*devicepb.Device, error) {
		return func(resp *devicepb.EnrollDeviceResponse, stream devicepb.DeviceTrustService_EnrollDeviceClient) (*devicepb.Device, error) {
			c := resp.GetMacosChallenge().GetChallenge()
			sig, err := key.signChallenge(c)
			if err != nil {
				return nil, fmt.Errorf("signChallenge failed: %w", err)
			}

			// 2. Challenge.
			if err := stream.Send(&devicepb.EnrollDeviceRequest{
				Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
					MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
						Signature: sig,
					},
				},
			}); err != nil {
				return nil, fmt.Errorf("challenge: Send failed: %w", err)
			}
			resp, err = stream.Recv()
			if err != nil {
				return nil, err // Unaltered, so it can be asserted.
			}

			// 3. Success.
			return resp.GetSuccess().GetDevice(), nil
		}
	}

	// newUnsupportedOSInit creates an init request for currently unsupported
	// OSes.
	newUnsupportedOSInit := func(dev *devicepb.Device) *devicepb.EnrollDeviceRequest {
		return &devicepb.EnrollDeviceRequest{
			Payload: &devicepb.EnrollDeviceRequest_Init{
				Init: &devicepb.EnrollDeviceInit{
					Token:        dev.EnrollToken.GetToken(),
					CredentialId: dev.AssetTag + "-credential",
					DeviceData: &devicepb.DeviceCollectedData{
						CollectTime:  timestamppb.Now(),
						OsType:       dev.OsType,
						SerialNumber: dev.AssetTag,
					},
				},
			},
		}
	}

	wantEnrollFailure := []wantEvent{
		{
			Type:     events.DeviceEvent,
			Code:     events.DeviceEnrollCode,
			WantFail: true,
		},
	}
	wantEnrollSuccess := []wantEvent{
		{
			Type: events.DeviceEvent,
			Code: events.DeviceEnrollCode,
		},
	}

	tests := []struct {
		name string
		// deviceTemplate is the device to create prior to enrollment.
		// If the device has an Id the test will refresh its enrollment token,
		// otherwise a new device is created.
		deviceTemplate       *devicepb.Device
		createInitRequest    func(dev *devicepb.Device) *devicepb.EnrollDeviceRequest
		assertInitErr        func(err error) bool
		handleOSStream       func(resp *devicepb.EnrollDeviceResponse, stream devicepb.DeviceTrustService_EnrollDeviceClient) (*devicepb.Device, error)
		createWantCredential func() *devicepb.DeviceCredential
		assertHandleErr      func(err error) bool
		wantAuditEvents      []wantEvent
	}{
		// General "Init" step validation errors.
		// These are failures regardless of the OsType.
		{
			name:           "init: invalid Token",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().Token = "invalid"
				return req
			},
			assertInitErr:   trace.IsAccessDenied,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: empty CredentialId",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().CredentialId = ""
				return req
			},
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: nil DeviceData",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().DeviceData = nil
				return req
			},
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: nil DeviceData.CollectTime",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().DeviceData.CollectTime = nil
				return req
			},
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: unspecified DeviceData.OsType",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().DeviceData.OsType = devicepb.OSType_OS_TYPE_UNSPECIFIED
				return req
			},
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: mismatched DeviceData.OsType",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().DeviceData.OsType = devicepb.OSType_OS_TYPE_LINUX
				return req
			},
			assertInitErr:   trace.IsNotFound,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: empty DeviceData.SerialNumber",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().DeviceData.SerialNumber = ""
				return req
			},
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "init: unknown DeviceData.SerialNumber",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().DeviceData.SerialNumber = "unknown"
				return req
			},
			assertInitErr:   trace.IsNotFound,
			wantAuditEvents: wantEnrollFailure,
		},

		// macOS enrollment and edge cases.
		{
			name: "macOS success",
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "llama",
			},
			createInitRequest:    newMacOSInitFunc(macOSKey1),
			handleOSStream:       newMacOSHandleFunc(macOSKey1),
			createWantCredential: macOSKey1.deviceCredential,
			wantAuditEvents:      wantEnrollSuccess,
		},
		{
			name:           "macOS: nil Macos payload",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().Macos = nil
				return req
			},
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "macOS: empty Macos.PubKeyDER",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().Macos.PublicKeyDer = []byte{}
				return req
			},
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "macOS: invalid Macos.PubKeyDER",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().Macos.PublicKeyDer = []byte("not a public key")
				return req
			},
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "macOS: unexpected Macos.PubKeyDER type",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().Macos.PublicKeyDer = rsaKeyDER // Enclave keys are ECDSA.
				return req
			},
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:           "macOS: unexpected Macos.PubKeyDER curve",
			deviceTemplate: failDev,
			createInitRequest: func(_ *devicepb.Device) *devicepb.EnrollDeviceRequest {
				req := newMacOSInitFunc(macOSKeyFail)(failDev)
				req.GetInit().Macos.PublicKeyDer = p224KeyDER // Enclave keys are P-256.
				return req
			},
			assertInitErr:   trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:              "macOS: invalid challenge signature",
			deviceTemplate:    failDev,
			createInitRequest: newMacOSInitFunc(macOSKeyFail),
			handleOSStream: func(_ *devicepb.EnrollDeviceResponse, stream devicepb.DeviceTrustService_EnrollDeviceClient) (*devicepb.Device, error) {
				// 2. Challenge.
				if err := stream.Send(&devicepb.EnrollDeviceRequest{
					Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
						MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
							Signature: []byte("not a signature"),
						},
					},
				}); err != nil {
					return nil, fmt.Errorf("challenge: Send failed: %w", err)
				}
				_, err := stream.Recv()
				return nil, err // Should be a non-nil error.
			},
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name:              "macOS: wrong key signs the challenge",
			deviceTemplate:    failDev,
			createInitRequest: newMacOSInitFunc(macOSKeyFail),
			handleOSStream: func(resp *devicepb.EnrollDeviceResponse, stream devicepb.DeviceTrustService_EnrollDeviceClient) (*devicepb.Device, error) {
				wrongKey, err := newFakeEnclaveKey()
				if err != nil {
					return nil, fmt.Errorf("failed to create fake key: %w", err)
				}
				// Use the wrong key to sign the challenge.
				return newMacOSHandleFunc(wrongKey)(resp, stream)
			},
			assertHandleErr: trace.IsBadParameter,
			wantAuditEvents: wantEnrollFailure,
		},

		// Unsupported (for now) OsTypes.
		{
			name: "Linux not supported",
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_LINUX,
				AssetTag: "linux1",
			},
			createInitRequest: newUnsupportedOSInit,
			assertInitErr: func(err error) bool {
				return trace.IsBadParameter(err) && strings.Contains(err.Error(), "Linux")
			},
			wantAuditEvents: wantEnrollFailure,
		},
		{
			name: "Windows not supported",
			deviceTemplate: &devicepb.Device{
				OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
				AssetTag: "windows1",
			},
			createInitRequest: newUnsupportedOSInit,
			assertInitErr: func(err error) bool {
				return trace.IsBadParameter(err) && strings.Contains(err.Error(), "Windows")
			},
			wantAuditEvents: wantEnrollFailure,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create device and enrollment token to use below.
			var created *devicepb.Device
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
			default: // Create new enrollment token
				token, err := devices.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
					DeviceId: dev.Id,
				})
				if err != nil {
					t.Fatalf("CreateDeviceEnrollToken failed: %v", err)
				}
				created = dev
				created.EnrollToken = token
			}
			emitter.Reset()

			assertAudit := false
			defer func() {
				if assertAudit {
					assertEvents(t, emitter.Events(), test.wantAuditEvents)
				}
			}()

			// 1. Init.
			stream, err := devices.EnrollDevice(ctx)
			if err != nil {
				t.Fatalf("EnrollDevice failed: %v", err)
			}
			if err := stream.Send(test.createInitRequest(created)); err != nil {
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
			gotDev, err := test.handleOSStream(resp, stream)
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
				t.Fatalf("EnrollDevice returned nil device on success")
			}

			// Verify that basic device fields are updated.
			if got, want := gotDev.UpdateTime.AsTime(), created.UpdateTime.AsTime(); !got.After(want) {
				t.Errorf("gotDev.UpdateTime=%v, want >%v", got, want)
			}

			wantDev := created
			wantDev.UpdateTime = gotDev.UpdateTime
			wantDev.EnrollToken = nil // token spent, also not expected here
			wantDev.EnrollStatus = devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED
			wantDev.Credential = test.createWantCredential()
			wantDev.CollectedData = nil // not expected here
			if diff := cmp.Diff(wantDev, gotDev, protocmp.Transform()); diff != "" {
				t.Errorf("EnrollDevice mismatch (-want +got):\n%s", diff)
			}

			storedDev, err := devices.GetDevice(ctx, &devicepb.GetDeviceRequest{
				DeviceId: gotDev.Id,
			})
			if err != nil {
				t.Fatalf("GetDevice failed: %v", err)
			}
			// TODO(codingllama): Assert collected data in tests.
			storedDev.CollectedData = nil
			if diff := cmp.Diff(gotDev, storedDev, protocmp.Transform()); diff != "" {
				t.Errorf("GetDevice mismatch (-want +got):\n%s", diff)
			}
		})
	}
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
