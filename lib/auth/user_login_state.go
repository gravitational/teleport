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

package auth

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userloginstate"
)

// GeneratePureULS is a variant of user login state generation that emits no usage events and ignores any existing user login state
// in the backend. Used for auditing/introspection purposes.
func (a *Server) GeneratePureULS(ctx context.Context, user types.User) (*userloginstate.UserLoginState, error) {
	return a.ulsGenerator.GeneratePureULS(ctx, user)
}
