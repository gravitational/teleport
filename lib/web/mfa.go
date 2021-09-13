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
	"github.com/gravitational/teleport/lib/web/ui"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// getMFADevicesWithTokenHandle retrieves the list of registered MFA devices for the user defined in token.
func (h *Handler) getMFADevicesWithTokenHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	recoveryApprovedTokenID := r.URL.Query().Get("token")

	mfas, err := h.cfg.ProxyClient.GetMFADevices(r.Context(), &proto.GetMFADevicesRequest{
		RecoveryApprovedTokenID: recoveryApprovedTokenID,
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

// deleteMFADeviceHandle deletes a mfa device for the user defined in the `token`, given as a query parameter.
func (h *Handler) deleteMFADeviceHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	tokenID := r.URL.Query().Get("token")
	deviceName := p.ByName("deviceName")

	if tokenID == "" || deviceName == "" {
		return nil, trace.BadParameter("missing token or device name")
	}

	if err := h.GetProxyClient().DeleteMFADeviceSync(r.Context(), &proto.DeleteMFADeviceSyncRequest{
		TokenID:    tokenID,
		DeviceName: deviceName,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}
