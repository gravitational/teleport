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
	"google.golang.org/protobuf/types/known/durationpb"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
)

func setResetUserRequestDefaults(r *userspb.ResetUserRequest) {
	if r.Type == "" {
		r.Type = authclient.UserTokenTypeResetPassword
	}

	if r.GetTtl().AsDuration() == 0 {
		switch r.Type {
		case authclient.UserTokenTypeResetPasswordInvite:
			r.Ttl = durationpb.New(defaults.SignupTokenTTL)

		case authclient.UserTokenTypeResetPassword:
			r.Ttl = durationpb.New(defaults.ChangePasswordTokenTTL)

		default:
			// This is invalid, but we are not validating here, so set up any non-nil
			// value just to reduce a risk of panic.
			r.Ttl = durationpb.New(0)
		}
	}
}
