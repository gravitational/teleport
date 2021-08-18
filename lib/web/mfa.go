// Copyright 2021 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package web

import (
	"net/http"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
)

type mfaChallengeRequestWithTokenRequest struct {
	TokenID string `json:"tokenId"`
}

// getMFAChallengeRequestWithTokenHandle retrieves mfa challengges for the user defined in token.
func (h *Handler) getMFAChallengeRequestWithTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	var req mfaChallengeRequestWithTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := h.GetProxyClient().GetMFAAuthenticateChallengeWithToken(r.Context(), &proto.GetMFAAuthenticateChallengeWithTokenRequest{
		TokenID: req.TokenID,
	})
	if err != nil {
		h.log.WithError(err).Warn("Failed to get mfa auth challenges.")
		return nil, trace.AccessDenied("unable to get mfa challenges")
	}

	return makeMFAAuthenticateChallenge(res), nil
}

// getMFAChallengeRequestWithAuth retrieves mfa challenges for the currently logged in user.
func (h *Handler) getMFAChallengeRequestWithAuthHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := clt.GetMFAAuthenticateChallengeWithAuth(r.Context(), &proto.GetMFAAuthenticateChallengeWithAuthRequest{
		Username: ctx.GetUser(),
	})
	if err != nil {
		h.log.WithError(err).Warn("Failed to get mfa auth challenges.")
		return nil, trace.AccessDenied("unable to get mfa challenges")
	}

	return makeMFAAuthenticateChallenge(res), nil
}

// getMFADevicesWithTokenHandle retrieves all mfa devices for the user defined in the token.
func (h *Handler) getMFADevicesWithTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	res, err := h.GetProxyClient().GetMFADevicesWithToken(r.Context(), &proto.GetMFADevicesWithTokenRequest{
		TokenID: params.ByName("token"),
	})
	if err != nil {
		h.log.WithError(err).Warn("Failed to get mfa devices.")
		return nil, trace.AccessDenied("unable to get mfa devices")
	}

	return ui.MakeMFADevices(res.GetDevices()), nil
}

type deleteMFADeviceWithTokenRequest struct {
	TokenID    string `json:"tokenId"`
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
}

// deleteMFADeviceWithTokenHandle deletes a mfa device for the user defined in the token.
func (h *Handler) deleteMFADeviceWithTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	var req deleteMFADeviceWithTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := h.GetProxyClient().DeleteMFADeviceWithToken(r.Context(), &proto.DeleteMFADeviceWithTokenRequest{
		TokenID:  req.TokenID,
		DeviceID: req.DeviceID,
	}); err != nil {
		h.log.WithError(err).Warnf("Failed to delete mfa device %v (%v)", req.DeviceName, req.DeviceID)
		return nil, trace.AccessDenied("unable to delete mfa device %v", req.DeviceName)
	}

	return OK(), nil
}

type addMFADeviceWithTokenRequest struct {
	// TokenID is privilege token id.
	TokenID string `json:"tokenId"`
	// DeviceName is the name of new mfa device.
	DeviceName string `json:"deviceName"`
	// SecondFactorToken is the otp value.
	SecondFactorToken string `json:"secondFactorToken"`
	// U2FRegisterResponse is U2F registration challenge response.
	U2FRegisterResponse *u2f.RegisterChallengeResponse `json:"u2fRegisterResponse"`
}

// addMFADeviceWithTokenHandle adds a new mfa device for the user defined in the token.
func (h *Handler) addMFADeviceWithTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req addMFADeviceWithTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq := &proto.AddMFADeviceWithTokenRequest{
		TokenID:    req.TokenID,
		DeviceName: req.DeviceName,
	}

	switch {
	case req.SecondFactorToken != "":
		protoReq.SecondFactorCred = &proto.AddMFADeviceWithTokenRequest_SecondFactorToken{SecondFactorToken: req.SecondFactorToken}
	case req.U2FRegisterResponse != nil:
		protoReq.SecondFactorCred = &proto.AddMFADeviceWithTokenRequest_U2FRegisterResponse{U2FRegisterResponse: &proto.U2FRegisterResponse{
			RegistrationData: req.U2FRegisterResponse.RegistrationData,
			ClientData:       req.U2FRegisterResponse.ClientData,
		}}
	default:
		return nil, trace.BadParameter("missing new second factor credentials")
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.AddMFADeviceWithToken(r.Context(), protoReq); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// getMFADevicesHandle adds a new mfa device for the user defined in the token.
func (h *Handler) getMFADevicesHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	devices, err := clt.GetMFADevices(r.Context(), &proto.GetMFADevicesRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeMFADevices(devices.GetDevices()), nil
}

func makeMFAAuthenticateChallenge(res *proto.MFAAuthenticateChallenge) *auth.MFAAuthenticateChallenge {
	// Convert from proto to JSON format.
	chal := &auth.MFAAuthenticateChallenge{
		TOTPChallenge: res.TOTP != nil,
	}

	for _, u2fChal := range res.U2F {
		ch := u2f.AuthenticateChallenge{
			Version:   u2fChal.Version,
			Challenge: u2fChal.Challenge,
			KeyHandle: u2fChal.KeyHandle,
			AppID:     u2fChal.AppID,
		}
		chal.U2FChallenges = append(chal.U2FChallenges, ch)
	}

	return chal
}
