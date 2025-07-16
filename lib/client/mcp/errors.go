// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package mcp

import (
	"errors"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/utils"
)

// IsSessionExpiredError returns if the MCP error is related to the `tsh`
// session being expired.
func IsSessionExpiredError(err error) bool {
	return errors.Is(err, apiclient.ErrClientCredentialsHaveExpired) || utils.IsCertExpiredError(err)
}

// ReloginRequiredErrorMessage is the message returned to the MCP client
// when the tsh session expired.
const ReloginRequiredErrorMessage = `It looks like your Teleport session expired,
you must relogin (using "tsh login" on a terminal) before continue using this
tool. After that, there is no need to update or relaunch the MCP client - just
try using it again.`
