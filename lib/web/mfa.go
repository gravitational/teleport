/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

import (
	"net/http"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web/ui"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// getMFADevicesWithTokenHandle retrieves the list of registered MFA devices for the user defined in token.
func (h *Handler) getMFADevicesWithTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	mfas, err := h.cfg.ProxyClient.GetMFADevices(r.Context(), &proto.GetMFADevicesRequest{
		RecoveryApprovedTokenID: p.ByName("token"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeMFADevices(mfas.GetDevices()), nil
}

// getMFADevicesHandle retrieves the list of registered MFA devices for the user in context (logged in user).
func (h *Handler) getMFADevicesHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mfas, err := clt.GetMFADevices(r.Context(), &proto.GetMFADevicesRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeMFADevices(mfas.GetDevices()), nil
}

// deleteMFADeviceWithTokenHandle deletes a mfa device for the user defined in the `token`, given as a query parameter.
func (h *Handler) deleteMFADeviceWithTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	if err := h.GetProxyClient().DeleteMFADeviceSync(r.Context(), &proto.DeleteMFADeviceSyncRequest{
		TokenID:    p.ByName("token"),
		DeviceName: p.ByName("devicename"),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

type addMFADeviceRequest struct {
	// PrivilegeTokenID is privilege token id.
	PrivilegeTokenID string `json:"tokenId"`
	// DeviceName is the name of new mfa device.
	DeviceName string `json:"deviceName"`
	// SecondFactorToken is the totp code.
	SecondFactorToken string `json:"secondFactorToken"`
	// U2FRegisterResponse is U2F registration challenge response.
	U2FRegisterResponse *u2f.RegisterChallengeResponse `json:"u2fRegisterResponse"`
	// WebauthnRegisterResponse is U2F registration challenge response.
	WebauthnRegisterResponse *webauthn.CredentialCreationResponse `json:"webauthnRegisterResponse"`
}

// addMFADeviceHandle adds a new mfa device for the user defined in the token.
func (h *Handler) addMFADeviceHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req addMFADeviceRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq := &proto.AddMFADeviceSyncRequest{
		PrivilegeTokenID: req.PrivilegeTokenID,
		NewDeviceName:    req.DeviceName,
	}

	switch {
	case req.SecondFactorToken != "":
		protoReq.NewMFAResponse = &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{Code: req.SecondFactorToken},
		}}
	case req.U2FRegisterResponse != nil:
		protoReq.NewMFAResponse = &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{
			U2F: &proto.U2FRegisterResponse{
				RegistrationData: req.U2FRegisterResponse.RegistrationData,
				ClientData:       req.U2FRegisterResponse.ClientData,
			},
		}}
	case req.WebauthnRegisterResponse != nil:
		protoReq.NewMFAResponse = &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_Webauthn{
			Webauthn: webauthn.CredentialCreationResponseToProto(req.WebauthnRegisterResponse),
		}}
	default:
		return nil, trace.BadParameter("missing new mfa credentials")
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := clt.AddMFADeviceSync(r.Context(), protoReq); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// createAuthenticateChallengeHandle creates and returns MFA authentication challenges for the user in context (logged in user).
// Used when users need to re-authenticate their second factors.
func (h *Handler) createAuthenticateChallengeHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params, c *SessionContext) (interface{}, error) {
	clt, err := c.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	chal, err := clt.CreateAuthenticateChallenge(r.Context(), &proto.CreateAuthenticateChallengeRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.MakeAuthenticateChallenge(chal), nil
}

// createAuthenticateChallengeWithTokenHandle creates and returns MFA authenticate challenges for the user defined in token.
func (h *Handler) createAuthenticateChallengeWithTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	chal, err := h.cfg.ProxyClient.CreateAuthenticateChallenge(r.Context(), &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_RecoveryStartTokenID{RecoveryStartTokenID: p.ByName("token")},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.MakeAuthenticateChallenge(chal), nil
}

type createRegisterChallengeRequest struct {
	// DeviceType is the type of MFA device to get a register challenge for.
	DeviceType string `json:"deviceType"`
}

// createRegisterChallengeWithTokenHandle creates and returns MFA register challenges for a new device for the specified device type.
func (h *Handler) createRegisterChallengeWithTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req createRegisterChallengeRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	var deviceType proto.DeviceType
	switch req.DeviceType {
	case "totp":
		deviceType = proto.DeviceType_DEVICE_TYPE_TOTP
	case "u2f":
		deviceType = proto.DeviceType_DEVICE_TYPE_U2F
	case "webauthn":
		deviceType = proto.DeviceType_DEVICE_TYPE_WEBAUTHN
	default:
		return nil, trace.BadParameter("MFA device type %q unsupported", req.DeviceType)
	}

	chal, err := h.cfg.ProxyClient.CreateRegisterChallenge(r.Context(), &proto.CreateRegisterChallengeRequest{
		TokenID:    p.ByName("token"),
		DeviceType: deviceType,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client.MakeRegisterChallenge(chal), nil
}
