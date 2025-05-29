/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package mcputils

import (
	"errors"
	"io"

	"github.com/gravitational/teleport/lib/utils"
)

// IsOKCloseError checks if provided error is a common close error that
// indicates the connection is ended.
func IsOKCloseError(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) ||
		utils.IsOKNetworkError(err)
}
