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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web/ui"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// getMFADevicesHandle gets mfa devices for the logged in user.
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

// deleteMFADeviceWithTokenHandle deletes a mfa device for the user defined in the token.
func (h *Handler) deleteMFADeviceWithTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	tokenID := params.ByName("token")
	deviceName := params.ByName("name")

	if err := h.GetProxyClient().DeleteMFADeviceSync(r.Context(), &proto.DeleteMFADeviceSyncRequest{
		TokenID:    tokenID,
		DeviceName: deviceName,
	}); err != nil {
		return nil, trace.Wrap(err)
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

	protoReq := &proto.AddMFADeviceSyncRequest{
		TokenID:       req.TokenID,
		NewDeviceName: req.DeviceName,
	}

	switch {
	case req.SecondFactorToken != "":
		protoReq.NewMFARegisterResponse = &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_TOTP{
			TOTP: &proto.TOTPRegisterResponse{Code: req.SecondFactorToken},
		}}
	case req.U2FRegisterResponse != nil:
		protoReq.NewMFARegisterResponse = &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{
			U2F: &proto.U2FRegisterResponse{
				RegistrationData: req.U2FRegisterResponse.RegistrationData,
				ClientData:       req.U2FRegisterResponse.ClientData,
			},
		}}
	default:
		return nil, trace.BadParameter("missing new mfa credentials")
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.AddMFADeviceSync(r.Context(), protoReq); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

// createQRCodeWithTokenHandle creates and returns qr code for the specified token ID.
func (h *Handler) createQRCodeWithTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	secrets, err := h.auth.proxyClient.RotateUserTokenSecrets(r.Context(), params.ByName("token"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return secrets.GetQRCode(), nil
}

// createAuthnChallengWithTokeneHandle creates and returns qr code for the specified token ID.
func (h *Handler) createAuthnChallengWithTokeneHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	protoReq := &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_TokenID{TokenID: params.ByName("token")},
	}

	res, err := h.auth.proxyClient.CreateAuthenticateChallenge(r.Context(), protoReq)
	if err != nil {
		h.log.WithError(err).Warn("Failed to get mfa auth challenges.")
		return nil, trace.AccessDenied("unable to get mfa challenges")
	}

	return makeMFAAuthenticateChallenge(res), nil
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
