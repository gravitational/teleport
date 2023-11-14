/*
Copyright 2023 Gravitational, Inc.

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

package web

import (
	mobilev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mobile/v1"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

type MobileAuthCreateResponse struct {
	Token string `json:"token"`
}

// mobileAuthCreate starts a mobile authentication flow. This creates a token
// which can be redeemed using mobileAuthRedeem on the mobile device.
func (h *Handler) mobileAuthCreate(
	_ http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext,
) (any, error) {
	c := mobilev1.NewMobileServiceClient(sctx.GetClientConnection())

	res, err := c.CreateAuthToken(r.Context(), &mobilev1.CreateAuthTokenRequest{})
	if err != nil {
		return nil, trace.Wrap(err, "creating token")
	}

	return MobileAuthCreateResponse{
		Token: res.Token,
	}, nil
}
