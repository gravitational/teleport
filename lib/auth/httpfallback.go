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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
)

// httpfallback.go holds endpoints that have been converted to gRPC
// but still need http fallback logic in the old client.

// DELETE IN 13.0.0
type legacyChangePasswordReq struct {
	// User is user ID
	User string
	// OldPassword is user current password
	OldPassword []byte `json:"old_password"`
	// NewPassword is user new password
	NewPassword []byte `json:"new_password"`
	// SecondFactorToken is user 2nd factor token
	SecondFactorToken string `json:"second_factor_token"`
	// WebauthnResponse is Webauthn sign response
	WebauthnResponse *wantypes.CredentialAssertionResponse `json:"webauthn_response"`
}

// ChangePassword updates users password based on the old password.
// REMOVE IN 13.0.0
func (c *Client) ChangePassword(ctx context.Context, req *proto.ChangePasswordRequest) error {
	if err := c.APIClient.ChangePassword(ctx, req); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	_, err := c.PutJSON(ctx, c.Endpoint("users", req.User, "web", "password"), legacyChangePasswordReq{
		OldPassword:       req.OldPassword,
		NewPassword:       req.NewPassword,
		SecondFactorToken: req.SecondFactorToken,
		WebauthnResponse:  wantypes.CredentialAssertionResponseFromProto(req.Webauthn),
	})
	return trace.Wrap(err)
}
