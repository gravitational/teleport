package devicetrustv1_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
)

func TestService_AuthenticateDevice(t *testing.T) {
	emitter := &eventstest.MockEmitter{}
	env := testenv.MustNew(
		testenv.WithAugmentCertsFunc(fakeAugmentFunc),
		testenv.WithEmitter(emitter),
	)
	defer env.Close()

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
			wantCerts, _ := fakeAugmentFunc(ctx, initCerts)
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
			assertEvents(t, emitter.Events(), []wantEvent{
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
	env := testenv.MustNew(testenv.WithEmitter(emitter))
	defer env.Close()

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
			assertEvents(t, emitter.Events(), []wantEvent{
				{
					Type:     events.DeviceEvent,
					Code:     events.DeviceAuthenticateCode,
					WantFail: true,
				},
			})

		})
	}
}

func createAndEnroll(ctx context.Context, devices devicepb.DeviceTrustServiceClient, dev *devicepb.Device) (*devicepb.Device, *fakeEnclaveKey, error) {
	dev, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device:            dev,
		CreateEnrollToken: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("method CreateDevice: %v", err)
	}

	key, err := newFakeEnclaveKey()
	if err != nil {
		return nil, nil, err
	}

	stream, err := devices.EnrollDevice(ctx)
	if err != nil {
		return nil, nil, err
	}
	sendAndRecv := func(msg *devicepb.EnrollDeviceRequest) (*devicepb.EnrollDeviceResponse, error) {
		if err := stream.Send(msg); err != nil {
			return nil, err
		}
		return stream.Recv()
	}

	resp, err := sendAndRecv(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_Init{
			Init: &devicepb.EnrollDeviceInit{
				Token:        dev.EnrollToken.Token,
				CredentialId: key.id,
				DeviceData: &devicepb.DeviceCollectedData{
					CollectTime:  timestamppb.Now(),
					OsType:       dev.OsType,
					SerialNumber: dev.AssetTag,
				},
				Macos: &devicepb.MacOSEnrollPayload{
					PublicKeyDer: key.pubKeyDER,
				},
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	chalResp := resp.GetMacosChallenge()
	sig, err := key.signChallenge(chalResp.Challenge)
	if err != nil {
		return nil, nil, err
	}
	resp, err = sendAndRecv(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
			MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
				Signature: sig,
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	return resp.GetSuccess().Device, key, nil
}

func fakeAugmentFunc(ctx context.Context, certs *devicepb.UserCertificates) (*devicepb.UserCertificates, error) {
	sshCert := certs.SshAuthorizedKey
	if sshCert != nil {
		sshCert = append(sshCert, 9)
	}
	return &devicepb.UserCertificates{
		X509Der:          []byte("<augmented mTLS cert goes here>"),
		SshAuthorizedKey: sshCert,
	}, nil
}
