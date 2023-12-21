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
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestStatusFromStatusCode(t *testing.T) {
	testCases := []struct {
		httpCode int
		want     types.PluginStatusCode
	}{
		{
			httpCode: http.StatusOK,
			want:     types.PluginStatusCode_RUNNING,
		},
		{
			httpCode: http.StatusNoContent,
			want:     types.PluginStatusCode_RUNNING,
		},

		{
			httpCode: http.StatusUnauthorized,
			want:     types.PluginStatusCode_UNAUTHORIZED,
		},
		{
			httpCode: http.StatusInternalServerError,
			want:     types.PluginStatusCode_OTHER_ERROR,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d", tc.httpCode), func(t *testing.T) {
			require.Equal(t, tc.want, StatusFromStatusCode(tc.httpCode).GetCode())
		})
	}
}
