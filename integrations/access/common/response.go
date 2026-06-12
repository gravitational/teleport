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

package common

import (
	"net/http"

	"github.com/go-resty/resty/v2"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

// ErrWrapperFunc are functions used to wrap http errors by plugin clients that return structured error responses.
type ErrWrapperFunc func(statusCode int, body []byte) error

// OnAfterResponse is a generic resty ResponseMiddleware that wraps errors and updates the status sink for plugins.
func OnAfterResponse(pluginName string, errWrapper ErrWrapperFunc, sink StatusSink) resty.ResponseMiddleware {
	return func(_ *resty.Client, resp *resty.Response) error {
		if sink != nil {
			var code types.PluginStatusCode
			switch {
			case resp.StatusCode() == http.StatusUnauthorized:
				code = types.PluginStatusCode_UNAUTHORIZED
			case resp.StatusCode() >= 200 && resp.StatusCode() < 400:
				code = types.PluginStatusCode_RUNNING
			default:
				code = types.PluginStatusCode_OTHER_ERROR
			}
			if err := sink.Emit(resp.Request.Context(), &types.PluginStatusV1{Code: code}); err != nil {
				logger.Get(resp.Request.Context()).
					Errorf("Error while emitting plugin status for plugin %q: status code: %v: %v", pluginName, resp.StatusCode(), err)
			}
		}
		if resp.IsError() {
			return errWrapper(resp.StatusCode(), resp.Body())
		}
		return nil
	}
}
