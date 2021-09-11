/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"net"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestMFADeviceManagement(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)
	clock := srv.Clock().(clockwork.FakeClock)

	// Enable U2F support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		U2F: &types.U2F{
			AppID:  "teleport",
			Facets: []string{"teleport"},
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Create a fake user.
	user, _, err := CreateUserAndRole(srv.Auth(), "mfa-user", []string{"role"})
	require.NoError(t, err)
	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// No MFA devices should exist for a new user.
	resp, err := cl.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.Devices)

	totpSecrets := make(map[string]string)
	u2fDevices := make(map[string]*mocku2f.Key)

	// Add several MFA devices.
	addTests := []struct {
		desc string
		opts mfaAddTestOpts
	}{
		{
			desc: "add initial TOTP device",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "totp-dev",
					Type:       proto.AddMFADeviceRequestInit_TOTP,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// The challenge should be empty for the first device.
					require.Empty(t, cmp.Diff(req, &proto.MFAAuthenticateChallenge{}))
					return &proto.MFAAuthenticateResponse{}
				},
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, req *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					totpRegisterChallenge := req.GetTOTP()
					require.NotEmpty(t, totpRegisterChallenge)
					require.Equal(t, totpRegisterChallenge.Algorithm, otp.AlgorithmSHA1.String())
					code, err := totp.GenerateCodeCustom(totpRegisterChallenge.Secret, clock.Now(), totp.ValidateOpts{
						Period:    uint(totpRegisterChallenge.PeriodSeconds),
						Digits:    otp.Digits(totpRegisterChallenge.Digits),
						Algorithm: otp.AlgorithmSHA1,
					})
					require.NoError(t, err)

					totpSecrets["totp-dev"] = totpRegisterChallenge.Secret
					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_TOTP{TOTP: &proto.TOTPRegisterResponse{
							Code: code,
						}},
					}
				},
				checkRegisterErr: require.NoError,
				wantDev: func(t *testing.T) *types.MFADevice {
					wantDev, err := services.NewTOTPDevice("totp-dev", totpSecrets["totp-dev"], clock.Now())
					require.NoError(t, err)
					return wantDev
				},
			},
		},
		{
			desc: "add a U2F device",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "u2f-dev",
					Type:       proto.AddMFADeviceRequestInit_U2F,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Respond to challenge using the existing TOTP device.
					require.NotNil(t, req.TOTP)
					code, err := totp.GenerateCode(totpSecrets["totp-dev"], clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, req *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					u2fRegisterChallenge := req.GetU2F()
					require.NotEmpty(t, u2fRegisterChallenge)

					mdev, err := mocku2f.Create()
					require.NoError(t, err)
					u2fDevices["u2f-dev"] = mdev
					mresp, err := mdev.RegisterResponse(&u2f.RegisterChallenge{
						Challenge: u2fRegisterChallenge.Challenge,
						AppID:     u2fRegisterChallenge.AppID,
					})
					require.NoError(t, err)

					return &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{U2F: &proto.U2FRegisterResponse{
						RegistrationData: mresp.RegistrationData,
						ClientData:       mresp.ClientData,
					}}}
				},
				checkRegisterErr: require.NoError,
				wantDev: func(t *testing.T) *types.MFADevice {
					wantDev, err := u2f.NewDevice(
						"u2f-dev",
						&u2f.Registration{
							KeyHandle: u2fDevices["u2f-dev"].KeyHandle,
							PubKey:    u2fDevices["u2f-dev"].PrivateKey.PublicKey,
						},
						clock.Now(),
					)
					require.NoError(t, err)
					return wantDev
				},
			},
		},
		{
			desc: "fail U2F auth challenge",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "fail-dev",
					Type:       proto.AddMFADeviceRequestInit_U2F,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.Len(t, req.U2F, 1)
					chal := req.U2F[0]

					// Use a different, unregistered device, which should fail
					// the authentication challenge.
					keyHandle, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(chal.KeyHandle)
					require.NoError(t, err)
					badDev, err := mocku2f.CreateWithKeyHandle(keyHandle)
					require.NoError(t, err)
					mresp, err := badDev.SignResponse(&u2f.AuthenticateChallenge{
						Challenge: chal.Challenge,
						KeyHandle: chal.KeyHandle,
						AppID:     chal.AppID,
					})
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_U2F{U2F: &proto.U2FResponse{
						KeyHandle:  mresp.KeyHandle,
						ClientData: mresp.ClientData,
						Signature:  mresp.SignatureData,
					}}}
				},
				checkAuthErr: require.Error,
			},
		},
		{
			desc: "fail TOTP auth challenge",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "fail-dev",
					Type:       proto.AddMFADeviceRequestInit_U2F,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, req.TOTP)

					// Respond to challenge using an unregistered TOTP device,
					// which should fail the auth challenge.
					badDev, err := totp.Generate(totp.GenerateOpts{Issuer: "Teleport", AccountName: user.GetName()})
					require.NoError(t, err)
					code, err := totp.GenerateCode(badDev.Secret(), clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkAuthErr: require.Error,
			},
		},
		{
			desc: "fail a U2F registration challenge",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "fail-dev",
					Type:       proto.AddMFADeviceRequestInit_U2F,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Respond to challenge using the existing TOTP device.
					require.NotNil(t, req.TOTP)
					code, err := totp.GenerateCode(totpSecrets["totp-dev"], clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, req *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					u2fRegisterChallenge := req.GetU2F()
					require.NotEmpty(t, u2fRegisterChallenge)

					mdev, err := mocku2f.Create()
					require.NoError(t, err)
					mresp, err := mdev.RegisterResponse(&u2f.RegisterChallenge{
						Challenge: u2fRegisterChallenge.Challenge,
						AppID:     "wrong app ID", // This should cause registration to fail.
					})
					require.NoError(t, err)

					return &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{U2F: &proto.U2FRegisterResponse{
						RegistrationData: mresp.RegistrationData,
						ClientData:       mresp.ClientData,
					}}}
				},
				checkRegisterErr: require.Error,
			},
		},
		{
			desc: "fail a TOTP registration challenge",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "fail-dev",
					Type:       proto.AddMFADeviceRequestInit_TOTP,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Respond to challenge using the existing TOTP device.
					require.NotNil(t, req.TOTP)
					code, err := totp.GenerateCode(totpSecrets["totp-dev"], clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, req *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					totpRegisterChallenge := req.GetTOTP()
					require.NotEmpty(t, totpRegisterChallenge)
					require.Equal(t, totpRegisterChallenge.Algorithm, otp.AlgorithmSHA1.String())
					// Use the wrong secret for registration, causing server
					// validation to fail.
					code, err := totp.GenerateCodeCustom(base32.StdEncoding.EncodeToString([]byte("wrong-secret")), clock.Now(), totp.ValidateOpts{
						Period:    uint(totpRegisterChallenge.PeriodSeconds),
						Digits:    otp.Digits(totpRegisterChallenge.Digits),
						Algorithm: otp.AlgorithmSHA1,
					})
					require.NoError(t, err)

					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_TOTP{TOTP: &proto.TOTPRegisterResponse{
							Code: code,
						}},
					}
				},
				checkRegisterErr: require.Error,
			},
		},
	}
	for _, tt := range addTests {
		t.Run(tt.desc, func(t *testing.T) {
			testAddMFADevice(ctx, t, cl, tt.opts)
			// Advance the time to roll TOTP tokens.
			clock.Advance(30 * time.Second)
		})
	}

	// Check that all new devices are registered.
	resp, err = cl.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	deviceNames := make([]string, 0, len(resp.Devices))
	deviceIDs := make(map[string]string)
	for _, dev := range resp.Devices {
		deviceNames = append(deviceNames, dev.GetName())
		deviceIDs[dev.GetName()] = dev.Id
	}
	sort.Strings(deviceNames)
	require.Equal(t, deviceNames, []string{"totp-dev", "u2f-dev"})

	// Delete several of the MFA devices.
	deleteTests := []struct {
		desc string
		opts mfaDeleteTestOpts
	}{
		{
			desc: "fail to delete an unknown device",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: "unknown-dev",
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, req.TOTP)
					code, err := totp.GenerateCode(totpSecrets["totp-dev"], clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkErr: require.Error,
			},
		},
		{
			desc: "fail a TOTP auth challenge",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: "totp-dev",
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, req.TOTP)

					// Respond to challenge using an unregistered TOTP device,
					// which should fail the auth challenge.
					badDev, err := totp.Generate(totp.GenerateOpts{Issuer: "Teleport", AccountName: user.GetName()})
					require.NoError(t, err)
					code, err := totp.GenerateCode(badDev.Secret(), clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkErr: require.Error,
			},
		},
		{
			desc: "fail a U2F auth challenge",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: "totp-dev",
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.Len(t, req.U2F, 1)
					chal := req.U2F[0]

					// Use a different, unregistered device, which should fail
					// the authentication challenge.
					keyHandle, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(chal.KeyHandle)
					require.NoError(t, err)
					badDev, err := mocku2f.CreateWithKeyHandle(keyHandle)
					require.NoError(t, err)
					mresp, err := badDev.SignResponse(&u2f.AuthenticateChallenge{
						Challenge: chal.Challenge,
						KeyHandle: chal.KeyHandle,
						AppID:     chal.AppID,
					})
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_U2F{U2F: &proto.U2FResponse{
						KeyHandle:  mresp.KeyHandle,
						ClientData: mresp.ClientData,
						Signature:  mresp.SignatureData,
					}}}
				},
				checkErr: require.Error,
			},
		},
		{
			desc: "delete TOTP device by name",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: "totp-dev",
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Respond to the challenge using the TOTP device being deleted.
					require.NotNil(t, req.TOTP)
					code, err := totp.GenerateCode(totpSecrets["totp-dev"], clock.Now())
					require.NoError(t, err)

					delete(totpSecrets, "totp-dev")

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkErr: require.NoError,
			},
		},
		{
			desc: "delete last U2F device by ID",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: deviceIDs["u2f-dev"],
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.Len(t, req.U2F, 1)
					chal := req.U2F[0]

					mdev := u2fDevices["u2f-dev"]
					mresp, err := mdev.SignResponse(&u2f.AuthenticateChallenge{
						Challenge: chal.Challenge,
						KeyHandle: chal.KeyHandle,
						AppID:     chal.AppID,
					})
					require.NoError(t, err)

					delete(u2fDevices, "u2f-dev")
					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_U2F{U2F: &proto.U2FResponse{
						KeyHandle:  mresp.KeyHandle,
						ClientData: mresp.ClientData,
						Signature:  mresp.SignatureData,
					}}}
				},
				checkErr: require.NoError,
			},
		},
	}
	for _, tt := range deleteTests {
		t.Run(tt.desc, func(t *testing.T) {
			testDeleteMFADevice(ctx, t, cl, tt.opts)
			// Advance the time to roll TOTP tokens.
			clock.Advance(30 * time.Second)
		})
	}

	// Check the remaining number of devices
	resp, err = cl.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.Devices)
}

type mfaAddTestOpts struct {
	initReq          *proto.AddMFADeviceRequestInit
	authHandler      func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
	checkAuthErr     require.ErrorAssertionFunc
	registerHandler  func(*testing.T, *proto.MFARegisterChallenge) *proto.MFARegisterResponse
	checkRegisterErr require.ErrorAssertionFunc
	wantDev          func(*testing.T) *types.MFADevice
}

func testAddMFADevice(ctx context.Context, t *testing.T, cl *Client, opts mfaAddTestOpts) {
	addStream, err := cl.AddMFADevice(ctx)
	require.NoError(t, err)
	err = addStream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_Init{Init: opts.initReq}})
	require.NoError(t, err)

	authChallenge, err := addStream.Recv()
	require.NoError(t, err)
	authResp := opts.authHandler(t, authChallenge.GetExistingMFAChallenge())
	err = addStream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_ExistingMFAResponse{ExistingMFAResponse: authResp}})
	require.NoError(t, err)

	registerChallenge, err := addStream.Recv()
	opts.checkAuthErr(t, err)
	if err != nil {
		return
	}
	registerResp := opts.registerHandler(t, registerChallenge.GetNewMFARegisterChallenge())
	err = addStream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_NewMFARegisterResponse{NewMFARegisterResponse: registerResp}})
	require.NoError(t, err)

	registerAck, err := addStream.Recv()
	opts.checkRegisterErr(t, err)
	if err != nil {
		return
	}
	require.Empty(t, cmp.Diff(registerAck.GetAck(), &proto.AddMFADeviceResponseAck{
		Device: opts.wantDev(t),
	}, cmpopts.IgnoreFields(types.MFADevice{}, "Id")))

	require.NoError(t, addStream.CloseSend())
}

type mfaDeleteTestOpts struct {
	initReq     *proto.DeleteMFADeviceRequestInit
	authHandler func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
	checkErr    require.ErrorAssertionFunc
}

func testDeleteMFADevice(ctx context.Context, t *testing.T, cl *Client, opts mfaDeleteTestOpts) {
	deleteStream, err := cl.DeleteMFADevice(ctx)
	require.NoError(t, err)
	err = deleteStream.Send(&proto.DeleteMFADeviceRequest{Request: &proto.DeleteMFADeviceRequest_Init{Init: opts.initReq}})
	require.NoError(t, err)

	authChallenge, err := deleteStream.Recv()
	require.NoError(t, err)
	authResp := opts.authHandler(t, authChallenge.GetMFAChallenge())
	err = deleteStream.Send(&proto.DeleteMFADeviceRequest{Request: &proto.DeleteMFADeviceRequest_MFAResponse{MFAResponse: authResp}})
	require.NoError(t, err)

	deleteAck, err := deleteStream.Recv()
	opts.checkErr(t, err)
	if err != nil {
		return
	}
	require.Empty(t, cmp.Diff(deleteAck.GetAck(), &proto.DeleteMFADeviceResponseAck{}))

	require.NoError(t, deleteStream.CloseSend())
}

func TestGenerateUserSingleUseCert(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)
	clock := srv.Clock()

	// Enable U2F support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		U2F: &types.U2F{
			AppID:  "teleport",
			Facets: []string{"teleport"},
		}})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Register an SSH node.
	node := &types.ServerV2{
		Kind:    types.KindKubeService,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "node-a",
		},
		Spec: types.ServerSpecV2{
			Hostname: "node-a",
		},
	}
	_, err = srv.Auth().UpsertNode(ctx, node)
	require.NoError(t, err)
	// Register a k8s cluster.
	k8sSrv := &types.ServerV2{
		Kind:    types.KindKubeService,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "kube-a",
		},
		Spec: types.ServerSpecV2{
			KubernetesClusters: []*types.KubernetesCluster{{Name: "kube-a"}},
		},
	}
	err = srv.Auth().UpsertKubeService(ctx, k8sSrv)
	require.NoError(t, err)
	// Register a database.
	db, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "db-a",
	}, types.DatabaseServerSpecV3{
		Protocol: "postgres",
		URI:      "localhost",
		Hostname: "localhost",
		HostID:   "localhost",
	})
	require.NoError(t, err)

	_, err = srv.Auth().UpsertDatabaseServer(ctx, db)
	require.NoError(t, err)

	// Create a fake user.
	user, role, err := CreateUserAndRole(srv.Auth(), "mfa-user", []string{"role"})
	require.NoError(t, err)
	// Make sure MFA is required for this user.
	roleOpt := role.GetOptions()
	roleOpt.RequireSessionMFA = true
	role.SetOptions(roleOpt)
	err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// Register a U2F device for the fake user.
	u2fDev, err := mocku2f.Create()
	require.NoError(t, err)
	testAddMFADevice(ctx, t, cl, mfaAddTestOpts{
		initReq: &proto.AddMFADeviceRequestInit{
			DeviceName: "u2f-dev",
			Type:       proto.AddMFADeviceRequestInit_U2F,
		},
		authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
			// The challenge should be empty for the first device.
			require.Empty(t, cmp.Diff(req, &proto.MFAAuthenticateChallenge{}))
			return &proto.MFAAuthenticateResponse{}
		},
		checkAuthErr: require.NoError,
		registerHandler: func(t *testing.T, req *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
			u2fRegisterChallenge := req.GetU2F()
			require.NotEmpty(t, u2fRegisterChallenge)

			mresp, err := u2fDev.RegisterResponse(&u2f.RegisterChallenge{
				Challenge: u2fRegisterChallenge.Challenge,
				AppID:     u2fRegisterChallenge.AppID,
			})
			require.NoError(t, err)

			return &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{U2F: &proto.U2FRegisterResponse{
				RegistrationData: mresp.RegistrationData,
				ClientData:       mresp.ClientData,
			}}}
		},
		checkRegisterErr: require.NoError,
		wantDev: func(t *testing.T) *types.MFADevice {
			wantDev, err := u2f.NewDevice(
				"u2f-dev",
				&u2f.Registration{
					KeyHandle: u2fDev.KeyHandle,
					PubKey:    u2fDev.PrivateKey.PublicKey,
				},
				clock.Now(),
			)
			require.NoError(t, err)
			return wantDev
		},
	})
	// Fetch MFA device ID.
	devs, err := srv.Auth().Identity.GetMFADevices(ctx, user.GetName(), false)
	require.NoError(t, err)
	require.Len(t, devs, 1)
	u2fDevID := devs[0].Id

	u2fChallengeHandler := func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
		require.Len(t, req.U2F, 1)
		chal := req.U2F[0]

		mresp, err := u2fDev.SignResponse(&u2f.AuthenticateChallenge{
			Challenge: chal.Challenge,
			KeyHandle: chal.KeyHandle,
			AppID:     chal.AppID,
		})
		require.NoError(t, err)

		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_U2F{U2F: &proto.U2FResponse{
			KeyHandle:  mresp.KeyHandle,
			ClientData: mresp.ClientData,
			Signature:  mresp.SignatureData,
		}}}
	}
	_, pub, err := srv.Auth().GenerateKeyPair("")
	require.NoError(t, err)

	tests := []struct {
		desc string
		opts generateUserSingleUseCertTestOpts
	}{
		{
			desc: "ssh",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_SSH,
					NodeName:  "node-a",
				},
				checkInitErr: require.NoError,
				authHandler:  u2fChallengeHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetSSH()
					require.NotEmpty(t, crt)

					cert, err := sshutils.ParseCertificate(crt)
					require.NoError(t, err)

					require.Equal(t, cert.Extensions[teleport.CertExtensionMFAVerified], u2fDevID)
					require.True(t, net.ParseIP(cert.Extensions[teleport.CertExtensionClientIP]).IsLoopback())
					require.Equal(t, cert.ValidBefore, uint64(clock.Now().Add(teleport.UserSingleUseCertTTL).Unix()))
				},
			},
		},
		{
			desc: "k8s",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey:         pub,
					Username:          user.GetName(),
					Expires:           clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:             proto.UserCertsRequest_Kubernetes,
					KubernetesCluster: "kube-a",
				},
				checkInitErr: require.NoError,
				authHandler:  u2fChallengeHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetTLS()
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, identity.MFAVerified, u2fDevID)
					require.True(t, net.ParseIP(identity.ClientIP).IsLoopback())
					require.Equal(t, identity.Usage, []string{teleport.UsageKubeOnly})
					require.Equal(t, identity.KubernetesCluster, "kube-a")
				},
			},
		},
		{
			desc: "db",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_Database,
					RouteToDatabase: proto.RouteToDatabase{
						ServiceName: "db-a",
					},
				},
				checkInitErr: require.NoError,
				authHandler:  u2fChallengeHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetTLS()
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, identity.MFAVerified, u2fDevID)
					require.True(t, net.ParseIP(identity.ClientIP).IsLoopback())
					require.Equal(t, identity.Usage, []string{teleport.UsageDatabaseOnly})
					require.Equal(t, identity.RouteToDatabase.ServiceName, "db-a")
				},
			},
		},
		{
			desc: "fail - wrong usage",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_All,
					NodeName:  "node-a",
				},
				checkInitErr: require.Error,
			},
		},
		{
			desc: "ssh - adjusted expiry",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:  clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:    proto.UserCertsRequest_SSH,
					NodeName: "node-a",
				},
				checkInitErr: require.NoError,
				authHandler:  u2fChallengeHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetSSH()
					require.NotEmpty(t, crt)

					cert, err := sshutils.ParseCertificate(crt)
					require.NoError(t, err)

					require.Equal(t, cert.Extensions[teleport.CertExtensionMFAVerified], u2fDevID)
					require.True(t, net.ParseIP(cert.Extensions[teleport.CertExtensionClientIP]).IsLoopback())
					require.Equal(t, cert.ValidBefore, uint64(clock.Now().Add(teleport.UserSingleUseCertTTL).Unix()))
				},
			},
		},
		{
			desc: "fail - mfa challenge fail",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_SSH,
					NodeName:  "node-a",
				},
				checkInitErr: require.NoError,
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Return no challenge response.
					return &proto.MFAAuthenticateResponse{}
				},
				checkAuthErr: require.Error,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			testGenerateUserSingleUseCert(ctx, t, cl, tt.opts)
		})
	}
}

type generateUserSingleUseCertTestOpts struct {
	initReq      *proto.UserCertsRequest
	checkInitErr require.ErrorAssertionFunc
	authHandler  func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
	checkAuthErr require.ErrorAssertionFunc
	validateCert func(*testing.T, *proto.SingleUseUserCert)
}

func testGenerateUserSingleUseCert(ctx context.Context, t *testing.T, cl *Client, opts generateUserSingleUseCertTestOpts) {
	stream, err := cl.GenerateUserSingleUseCerts(ctx)
	require.NoError(t, err)
	err = stream.Send(&proto.UserSingleUseCertsRequest{Request: &proto.UserSingleUseCertsRequest_Init{Init: opts.initReq}})
	require.NoError(t, err)

	authChallenge, err := stream.Recv()
	opts.checkInitErr(t, err)
	if err != nil {
		return
	}
	authResp := opts.authHandler(t, authChallenge.GetMFAChallenge())
	err = stream.Send(&proto.UserSingleUseCertsRequest{Request: &proto.UserSingleUseCertsRequest_MFAResponse{MFAResponse: authResp}})
	require.NoError(t, err)

	certs, err := stream.Recv()
	opts.checkAuthErr(t, err)
	if err != nil {
		return
	}
	opts.validateCert(t, certs.GetCert())

	require.NoError(t, stream.CloseSend())
}

func TestIsMFARequired(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Enable MFA support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		U2F: &types.U2F{
			AppID:  "teleport",
			Facets: []string{"teleport"},
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Register an SSH node.
	node := &types.ServerV2{
		Kind:    types.KindKubeService,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "node-a",
		},
		Spec: types.ServerSpecV2{
			Hostname: "node-a",
		},
	}
	_, err = srv.Auth().UpsertNode(ctx, node)
	require.NoError(t, err)

	// Create a fake user.
	user, role, err := CreateUserAndRole(srv.Auth(), "no-mfa-user", []string{"role"})
	require.NoError(t, err)

	for _, required := range []bool{true, false} {
		t.Run(fmt.Sprintf("required=%v", required), func(t *testing.T) {
			roleOpt := role.GetOptions()
			roleOpt.RequireSessionMFA = required
			role.SetOptions(roleOpt)
			err = srv.Auth().UpsertRole(ctx, role)
			require.NoError(t, err)

			cl, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			resp, err := cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
				Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
					Login: user.GetName(),
					Node:  "node-a",
				}},
			})
			require.NoError(t, err)
			require.Equal(t, resp.Required, required)
		})
	}
}

func TestIsMFARequiredUnauthorized(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Enable MFA support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		U2F: &types.U2F{
			AppID:  "teleport",
			Facets: []string{"teleport"},
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Register an SSH node.
	node1 := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "node1",
			Namespace: apidefaults.Namespace,
			Labels:    map[string]string{"a": "b"},
		},
		Spec: types.ServerSpecV2{
			Hostname: "node1",
			Addr:     "localhost:3022",
		},
	}
	_, err = srv.Auth().UpsertNode(ctx, node1)
	require.NoError(t, err)

	// Register another SSH node with a duplicate hostname.
	node2 := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "node2",
			Namespace: apidefaults.Namespace,
			Labels:    map[string]string{"a": "c"},
		},
		Spec: types.ServerSpecV2{
			Hostname: "node1",
			Addr:     "localhost:3022",
		},
	}
	_, err = srv.Auth().UpsertNode(ctx, node2)
	require.NoError(t, err)

	user, role, err := CreateUserAndRole(srv.Auth(), "alice", []string{"alice"})
	require.NoError(t, err)

	// Require MFA.
	roleOpt := role.GetOptions()
	roleOpt.RequireSessionMFA = true
	role.SetOptions(roleOpt)
	role.SetNodeLabels(types.Allow, map[string]apiutils.Strings{"a": []string{"c"}})
	err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// Call the endpoint for an authorized login. The user is only authorized
	// for the 2nd node, but should still be asked for MFA.
	resp, err := cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
		Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
			Login: "alice",
			Node:  "node1",
		}},
	})
	require.NoError(t, err)
	require.True(t, resp.Required)

	// Call the endpoint for an unauthorized login.
	resp, err = cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
		Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
			Login: "bob",
			Node:  "node1",
		}},
	})

	// When unauthorized, expect a silent `false`.
	require.NoError(t, err)
	require.False(t, resp.Required)
}

func TestDeleteLastMFADevice(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)
	clock := srv.Clock().(clockwork.FakeClock)

	// Enable MFA support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		U2F: &types.U2F{
			AppID:  "teleport",
			Facets: []string{"teleport"},
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Create a fake user.
	user, _, err := CreateUserAndRole(srv.Auth(), "mfa-user", []string{"role"})
	require.NoError(t, err)
	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// Register a U2F device.
	var u2fDev *mocku2f.Key
	testAddMFADevice(ctx, t, cl, mfaAddTestOpts{
		initReq: &proto.AddMFADeviceRequestInit{
			DeviceName: "u2f-dev",
			Type:       proto.AddMFADeviceRequestInit_U2F,
		},
		authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
			// The challenge should be empty for the first device.
			require.Empty(t, cmp.Diff(req, &proto.MFAAuthenticateChallenge{}))
			return &proto.MFAAuthenticateResponse{}
		},
		checkAuthErr: require.NoError,
		registerHandler: func(t *testing.T, req *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
			u2fRegisterChallenge := req.GetU2F()
			require.NotEmpty(t, u2fRegisterChallenge)

			mdev, err := mocku2f.Create()
			require.NoError(t, err)
			u2fDev = mdev
			mresp, err := mdev.RegisterResponse(&u2f.RegisterChallenge{
				Challenge: u2fRegisterChallenge.Challenge,
				AppID:     u2fRegisterChallenge.AppID,
			})
			require.NoError(t, err)

			return &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{U2F: &proto.U2FRegisterResponse{
				RegistrationData: mresp.RegistrationData,
				ClientData:       mresp.ClientData,
			}}}
		},
		checkRegisterErr: require.NoError,
		wantDev: func(t *testing.T) *types.MFADevice {
			wantDev, err := u2f.NewDevice(
				"u2f-dev",
				&u2f.Registration{
					KeyHandle: u2fDev.KeyHandle,
					PubKey:    u2fDev.PrivateKey.PublicKey,
				},
				clock.Now(),
			)
			require.NoError(t, err)
			return wantDev
		},
	})

	// Try to delete the only MFA device of the user.
	testDeleteMFADevice(ctx, t, cl, mfaDeleteTestOpts{
		initReq: &proto.DeleteMFADeviceRequestInit{
			DeviceName: "u2f-dev",
		},
		authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
			require.Len(t, req.U2F, 1)
			chal := req.U2F[0]

			mresp, err := u2fDev.SignResponse(&u2f.AuthenticateChallenge{
				Challenge: chal.Challenge,
				KeyHandle: chal.KeyHandle,
				AppID:     chal.AppID,
			})
			require.NoError(t, err)

			return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_U2F{U2F: &proto.U2FResponse{
				KeyHandle:  mresp.KeyHandle,
				ClientData: mresp.ClientData,
				Signature:  mresp.SignatureData,
			}}}
		},
		checkErr: require.Error,
	})
}

// TestRoleVersions tests that downgraded V3 roles are returned to older
// clients, and V4 roles are returned to newer clients.
func TestRoleVersions(t *testing.T) {
	srv := newTestTLSServer(t)

	role := &types.RoleV4{
		Kind:    types.KindRole,
		Version: types.V4,
		Metadata: types.Metadata{
			Name: "test_role",
		},
		Spec: types.RoleSpecV4{
			Allow: types.RoleConditions{
				Rules: []types.Rule{
					types.NewRule(types.KindRole, services.RO()),
					types.NewRule(types.KindEvent, services.RW()),
				},
			},
		},
	}
	user, err := CreateUser(srv.Auth(), "test_user", role)
	require.NoError(t, err)

	client, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	testCases := []struct {
		desc                string
		clientVersion       string
		disableMetadata     bool
		expectedRoleVersion string
		assertErr           require.ErrorAssertionFunc
	}{
		{
			desc:                "old",
			clientVersion:       "6.2.1",
			expectedRoleVersion: "v3",
			assertErr:           require.NoError,
		},
		{
			desc:                "new",
			clientVersion:       "6.3.0",
			expectedRoleVersion: "v4",
			assertErr:           require.NoError,
		},
		{
			desc:                "alpha",
			clientVersion:       "6.2.4-alpha.0",
			expectedRoleVersion: "v4",
			assertErr:           require.NoError,
		},
		{
			desc:                "greater than 10",
			clientVersion:       "10.0.0-beta",
			expectedRoleVersion: "v4",
			assertErr:           require.NoError,
		},
		{
			desc:          "empty version",
			clientVersion: "",
			assertErr:     require.Error,
		},
		{
			desc:          "invalid version",
			clientVersion: "foo",
			assertErr:     require.Error,
		},
		{
			desc:                "no version metadata",
			disableMetadata:     true,
			expectedRoleVersion: "v3",
			assertErr:           require.NoError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// setup client metadata
			ctx := context.Background()
			if tc.disableMetadata {
				ctx = context.WithValue(ctx, metadata.DisableInterceptors{}, struct{}{})
			} else {
				ctx = metadata.AddMetadataToContext(ctx, map[string]string{
					metadata.VersionKey: tc.clientVersion,
				})
			}

			// test GetRole
			gotRole, err := client.GetRole(ctx, role.GetName())
			tc.assertErr(t, err)
			if err == nil {
				require.Equal(t, tc.expectedRoleVersion, gotRole.GetVersion())
			}

			// test GetRoles
			gotRoles, err := client.GetRoles(ctx)
			tc.assertErr(t, err)
			if err == nil {
				foundTestRole := false
				for _, gotRole := range gotRoles {
					if gotRole.GetName() == role.GetName() {
						require.Equal(t, tc.expectedRoleVersion, gotRole.GetVersion())
						foundTestRole = true
					}
				}
				require.True(t, foundTestRole)
			}
		})
	}
}

// testOriginDynamicStored tests setting a ResourceWithOrigin via the server
// API always results in the resource being stored with OriginDynamic.
func testOriginDynamicStored(t *testing.T, setWithOrigin func(*Client, string) error, getStored func(*Server) (types.ResourceWithOrigin, error)) {
	srv := newTestTLSServer(t)

	// Create a fake user.
	user, _, err := CreateUserAndRole(srv.Auth(), "configurer", []string{})
	require.NoError(t, err)
	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	for _, origin := range types.OriginValues {
		t.Run(fmt.Sprintf("setting with origin %q", origin), func(t *testing.T) {
			err := setWithOrigin(cl, origin)
			require.NoError(t, err)

			stored, err := getStored(srv.Auth())
			require.NoError(t, err)
			require.Equal(t, stored.Origin(), types.OriginDynamic)
		})
	}
}

func TestAuthPreferenceOriginDynamic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setWithOrigin := func(cl *Client, origin string) error {
		authPref := types.DefaultAuthPreference()
		authPref.SetOrigin(origin)
		return cl.SetAuthPreference(ctx, authPref)
	}

	getStored := func(asrv *Server) (types.ResourceWithOrigin, error) {
		return asrv.GetAuthPreference(ctx)
	}

	testOriginDynamicStored(t, setWithOrigin, getStored)
}

func TestClusterNetworkingConfigOriginDynamic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setWithOrigin := func(cl *Client, origin string) error {
		netConfig := types.DefaultClusterNetworkingConfig()
		netConfig.SetOrigin(origin)
		return cl.SetClusterNetworkingConfig(ctx, netConfig)
	}

	getStored := func(asrv *Server) (types.ResourceWithOrigin, error) {
		return asrv.GetClusterNetworkingConfig(ctx)
	}

	testOriginDynamicStored(t, setWithOrigin, getStored)
}

func TestSessionRecordingConfigOriginDynamic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setWithOrigin := func(cl *Client, origin string) error {
		recConfig := types.DefaultSessionRecordingConfig()
		recConfig.SetOrigin(origin)
		return cl.SetSessionRecordingConfig(ctx, recConfig)
	}

	getStored := func(asrv *Server) (types.ResourceWithOrigin, error) {
		return asrv.GetSessionRecordingConfig(ctx)
	}

	testOriginDynamicStored(t, setWithOrigin, getStored)
}

func TestNodesCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// node1 and node2 will be added to default namespace
	node1, err := types.NewServerWithLabels("node1", types.KindNode, types.ServerSpecV2{}, nil)
	require.NoError(t, err)
	node2, err := types.NewServerWithLabels("node2", types.KindNode, types.ServerSpecV2{}, nil)
	require.NoError(t, err)

	t.Run("CreateNode", func(t *testing.T) {
		// Initially expect no nodes to be returned.
		nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Empty(t, nodes)

		// Create nodes.
		_, err = clt.UpsertNode(ctx, node1)
		require.NoError(t, err)

		_, err = clt.UpsertNode(ctx, node2)
		require.NoError(t, err)
	})

	// Run NodeGetters in nested subtests to allow parallelization.
	t.Run("NodeGetters", func(t *testing.T) {
		t.Run("List Nodes", func(t *testing.T) {
			t.Parallel()
			// list nodes one at a time, last page should be empty
			nodes, nextKey, err := clt.ListNodes(ctx, apidefaults.Namespace, 1, "")
			require.NoError(t, err)
			require.Len(t, nodes, 1)
			require.Empty(t, cmp.Diff([]types.Server{node1}, nodes,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))
			require.Equal(t, backend.NextPaginationKey(node1), nextKey)

			nodes, nextKey, err = clt.ListNodes(ctx, apidefaults.Namespace, 1, nextKey)
			require.NoError(t, err)
			require.Len(t, nodes, 1)
			require.Empty(t, cmp.Diff([]types.Server{node2}, nodes,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))
			require.Equal(t, backend.NextPaginationKey(node2), nextKey)

			nodes, nextKey, err = clt.ListNodes(ctx, apidefaults.Namespace, 1, nextKey)
			require.NoError(t, err)
			require.Empty(t, nodes)
			require.Equal(t, "", nextKey)

			// ListNodes should fail if namespace isn't provided
			_, _, err = clt.ListNodes(ctx, "", 1, "")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())

			// ListNodes should fail if limit is nonpositive
			_, _, err = clt.ListNodes(ctx, apidefaults.Namespace, 0, "")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())

			_, _, err = clt.ListNodes(ctx, apidefaults.Namespace, -1, "")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())
		})
		t.Run("GetNodes", func(t *testing.T) {
			t.Parallel()
			// Get all nodes
			nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
			require.NoError(t, err)
			require.Len(t, nodes, 2)
			require.Empty(t, cmp.Diff([]types.Server{node1, node2}, nodes,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// GetNodes should fail if namespace isn't provided
			_, err = clt.GetNodes(ctx, "")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())
		})
		t.Run("GetNode", func(t *testing.T) {
			t.Parallel()
			// Get Node
			node, err := clt.GetNode(ctx, apidefaults.Namespace, "node1")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(node1, node,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// GetNode should fail if node name isn't provided
			_, err = clt.GetNode(ctx, apidefaults.Namespace, "")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())

			// GetNode should fail if namespace isn't provided
			_, err = clt.GetNode(ctx, "", "node1")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())
		})
	})

	t.Run("DeleteNode", func(t *testing.T) {
		// Make sure can't delete with empty namespace or name.
		err = clt.DeleteNode(ctx, apidefaults.Namespace, "")
		require.Error(t, err)
		require.IsType(t, trace.BadParameter(""), err)

		err = clt.DeleteNode(ctx, "", node1.GetName())
		require.Error(t, err)
		require.IsType(t, trace.BadParameter(""), err)

		// Delete node.
		err = clt.DeleteNode(ctx, apidefaults.Namespace, node1.GetName())
		require.NoError(t, err)

		// Expect node not found
		_, err := clt.GetNode(ctx, apidefaults.Namespace, "node1")
		require.IsType(t, trace.NotFound(""), err)
	})

	t.Run("DeleteAllNodes", func(t *testing.T) {
		// Make sure can't delete with empty namespace.
		err = clt.DeleteAllNodes(ctx, "")
		require.Error(t, err)
		require.IsType(t, trace.BadParameter(""), err)

		// Delete nodes
		err = clt.DeleteAllNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)

		// Now expect no nodes to be returned.
		nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Empty(t, nodes)
	})
}

func TestLocksCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	now := srv.Clock().Now()
	lock1, err := types.NewLock("lock1", types.LockSpecV2{
		Target: types.LockTarget{
			User: "user-A",
		},
		Expires: &now,
	})
	require.NoError(t, err)

	lock2, err := types.NewLock("lock2", types.LockSpecV2{
		Target: types.LockTarget{
			Node: "node",
		},
		Message: "node compromised",
	})
	require.NoError(t, err)

	t.Run("CreateLock", func(t *testing.T) {
		// Initially expect no locks to be returned.
		locks, err := clt.GetLocks(ctx, false)
		require.NoError(t, err)
		require.Empty(t, locks)

		// Create locks.
		err = clt.UpsertLock(ctx, lock1)
		require.NoError(t, err)

		err = clt.UpsertLock(ctx, lock2)
		require.NoError(t, err)
	})

	// Run LockGetters in nested subtests to allow parallelization.
	t.Run("LockGetters", func(t *testing.T) {
		t.Run("GetLocks", func(t *testing.T) {
			t.Parallel()
			locks, err := clt.GetLocks(ctx, false)
			require.NoError(t, err)
			require.Len(t, locks, 2)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))
		})
		t.Run("GetLocks with targets", func(t *testing.T) {
			t.Parallel()
			// Match both locks with the targets.
			locks, err := clt.GetLocks(ctx, false, lock1.Target(), lock2.Target())
			require.NoError(t, err)
			require.Len(t, locks, 2)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// Match only one of the locks.
			roleTarget := types.LockTarget{Role: "role-A"}
			locks, err = clt.GetLocks(ctx, false, lock1.Target(), roleTarget)
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock1}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// Match none of the locks.
			locks, err = clt.GetLocks(ctx, false, roleTarget)
			require.NoError(t, err)
			require.Empty(t, locks)
		})
		t.Run("GetLock", func(t *testing.T) {
			t.Parallel()
			// Get one of the locks.
			lock, err := clt.GetLock(ctx, lock1.GetName())
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(lock1, lock,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// Attempt to get a nonexistent lock.
			_, err = clt.GetLock(ctx, "lock3")
			require.Error(t, err)
			require.True(t, trace.IsNotFound(err))
		})
	})

	t.Run("UpsertLock", func(t *testing.T) {
		// Get one of the locks.
		lock, err := clt.GetLock(ctx, lock1.GetName())
		require.NoError(t, err)
		require.Empty(t, lock.Message())

		msg := "cluster maintenance"
		lock1.SetMessage(msg)
		err = clt.UpsertLock(ctx, lock1)
		require.NoError(t, err)

		lock, err = clt.GetLock(ctx, lock1.GetName())
		require.NoError(t, err)
		require.Equal(t, msg, lock.Message())
	})

	t.Run("DeleteLock", func(t *testing.T) {
		// Delete lock.
		err = clt.DeleteLock(ctx, lock1.GetName())
		require.NoError(t, err)

		// Expect lock not found.
		_, err := clt.GetLock(ctx, lock1.GetName())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

// TestApplicationServersCRUD tests application server operations.
func TestApplicationServersCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create a couple app servers.
	app1, err := types.NewAppV3(types.Metadata{Name: "app-1"},
		types.AppSpecV3{URI: "localhost"})
	require.NoError(t, err)
	server1, err := types.NewAppServerV3FromApp(app1, "server-1", "server-1")
	require.NoError(t, err)
	app2, err := types.NewAppV3(types.Metadata{Name: "app-2"},
		types.AppSpecV3{URI: "localhost"})
	require.NoError(t, err)
	server2, err := types.NewAppServerV3FromApp(app2, "server-2", "server-2")
	require.NoError(t, err)

	// Create a legacy app server.
	app3, err := types.NewAppV3(types.Metadata{Name: "app-3"},
		types.AppSpecV3{URI: "localhost"})
	require.NoError(t, err)
	server3Legacy, err := types.NewLegacyAppServer(app3, "server-3", "server-3")
	require.NoError(t, err)
	server3, err := types.NewAppServerV3FromApp(app3, "server-3", "server-3")
	require.NoError(t, err)

	// Initially we expect no app servers.
	out, err := clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Register all app servers.
	_, err = clt.UpsertApplicationServer(ctx, server1)
	require.NoError(t, err)
	_, err = clt.UpsertApplicationServer(ctx, server2)
	require.NoError(t, err)
	_, err = clt.UpsertAppServer(ctx, server3Legacy)
	require.NoError(t, err)

	// Fetch all app servers.
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server1, server2, server3}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Update an app server.
	server1.Metadata.Description = "description"
	_, err = clt.UpsertApplicationServer(ctx, server1)
	require.NoError(t, err)
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server1, server2, server3}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete an app server.
	err = clt.DeleteApplicationServer(ctx, server1.GetNamespace(), server1.GetHostID(), server1.GetName())
	require.NoError(t, err)
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server2, server3}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete all app servers.
	err = clt.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	err = clt.DeleteAllAppServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
}

// TestDatabasesCRUD tests database resource operations.
func TestDatabasesCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create a couple databases.
	db1, err := types.NewDatabaseV3(types.Metadata{
		Name:   "db1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	db2, err := types.NewDatabaseV3(types.Metadata{
		Name:   "db2",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
	})
	require.NoError(t, err)

	// Initially we expect no databases.
	out, err := clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Create both databases.
	err = clt.CreateDatabase(ctx, db1)
	require.NoError(t, err)
	err = clt.CreateDatabase(ctx, db2)
	require.NoError(t, err)

	// Fetch all databases.
	out, err = clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{db1, db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a specific database.
	db, err := clt.GetDatabase(ctx, db2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(db2, db,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to fetch a database that doesn't exist.
	_, err = clt.GetDatabase(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create the same database.
	err = clt.CreateDatabase(ctx, db1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Update a database.
	db1.Metadata.Description = "description"
	err = clt.UpdateDatabase(ctx, db1)
	require.NoError(t, err)
	db, err = clt.GetDatabase(ctx, db1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(db1, db,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete a database.
	err = clt.DeleteDatabase(ctx, db1.GetName())
	require.NoError(t, err)
	out, err = clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete a database that doesn't exist.
	err = clt.DeleteDatabase(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all databases.
	err = clt.DeleteAllDatabases(ctx)
	require.NoError(t, err)
	out, err = clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Len(t, out, 0)
}

func TestCustomRateLimiting(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		fn   func(*Client) error
	}{
		{
			name: "RPC ApproveAccountRecovery",
			fn: func(clt *Client) error {
				_, err := clt.ApproveAccountRecovery(context.Background(), &proto.ApproveAccountRecoveryRequest{})
				return err
			},
		},
		{
			name: "RPC ChangeUserAuthentication",
			fn: func(clt *Client) error {
				_, err := clt.ChangeUserAuthentication(context.Background(), &proto.ChangeUserAuthenticationRequest{})
				return err
			},
		},
		{
			name: "RPC GetAccountRecoveryToken",
			fn: func(clt *Client) error {
				_, err := clt.GetAccountRecoveryToken(context.Background(), &proto.GetAccountRecoveryTokenRequest{})
				return err
			},
		},
		{
			name: "RPC StartAccountRecovery",
			fn: func(clt *Client) error {
				_, err := clt.StartAccountRecovery(context.Background(), &proto.StartAccountRecoveryRequest{})
				return err
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// For now since we only have one custom rate limit,
			// test limit for 1 request per minute with bursts up to 10 requests.
			const maxAttempts = 11
			var err error

			// Create new instance per test case, to troubleshoot
			// which test case specifically failed, otherwise
			// multiple cases can fail from running cases in parallel.
			srv := newTestTLSServer(t)
			clt, err := srv.NewClient(TestNop())
			require.NoError(t, err)

			for i := 0; i < maxAttempts; i++ {
				err = c.fn(clt)
				require.Error(t, err)
			}
			require.True(t, trace.IsLimitExceeded(err))
		})
	}
}
