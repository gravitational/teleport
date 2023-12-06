/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"net/http"

	"github.com/gravitational/teleport/api/types"
)

// StatusSink defines a destination for PluginStatus
type StatusSink interface {
	Emit(ctx context.Context, s types.PluginStatus) error
}

func StatusFromStatusCode(httpCode int) types.PluginStatus {
	var code types.PluginStatusCode
	switch {
	case httpCode == http.StatusUnauthorized:
		code = types.PluginStatusCode_UNAUTHORIZED
	case httpCode >= 200 && httpCode < 400:
		code = types.PluginStatusCode_RUNNING
	default:
		code = types.PluginStatusCode_OTHER_ERROR
	}
	return &types.PluginStatusV1{Code: code}
}
