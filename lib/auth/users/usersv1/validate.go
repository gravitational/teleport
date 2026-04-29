// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package usersv1

import (
	"github.com/gravitational/trace"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
)

func validateResetUserRequest(r *userspb.ResetUserRequest) error {
	if r.Name == "" {
		return trace.BadParameter("user name can't be empty")
	}

	if r.Ttl == nil {
		return trace.BadParameter("TTL can't be nil")
	}
	if r.Ttl.AsDuration() < 0 {
		return trace.BadParameter("TTL can't be negative")
	}

	switch r.Type {
	case authclient.UserTokenTypeResetPasswordInvite:
		if r.Ttl.AsDuration() > defaults.MaxSignupTokenTTL {
			return trace.BadParameter(
				"maximum token TTL for the user invitation flow is %v hours",
				defaults.MaxSignupTokenTTL)
		}

	case authclient.UserTokenTypeResetPassword:
		if r.Ttl.AsDuration() > defaults.MaxChangePasswordTokenTTL {
			return trace.BadParameter(
				"maximum token TTL for the password reset flow is %v hours",
				defaults.MaxChangePasswordTokenTTL)
		}

	default:
		return trace.BadParameter("unknown user token request type(%v)", r.Type)
	}

	return nil
}
