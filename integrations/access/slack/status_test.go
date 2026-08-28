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

package slack

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestStatusFromResponse(t *testing.T) {
	testCases := []struct {
		name     string
		response *APIResponse
		want     types.PluginStatusCode
	}{
		{
			name:     "ok",
			response: &APIResponse{Ok: true},
			want:     types.PluginStatusCode_RUNNING,
		},
		{
			name:     "not_in_channel",
			response: &APIResponse{Error: "not_in_channel"},
			want:     types.PluginStatusCode_SLACK_NOT_IN_CHANNEL,
		},
		{
			name:     "unauthorized",
			response: &APIResponse{Error: "token_revoked"},
			want:     types.PluginStatusCode_UNAUTHORIZED,
		},
		{
			name:     "other",
			response: &APIResponse{Error: "some_error"},
			want:     types.PluginStatusCode_OTHER_ERROR,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, statusFromResponse(tc.response).GetCode())
		})
	}
}
