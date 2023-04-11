/*
Copyright 2023 Gravitational, Inc.

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

package slack

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
			require.Equal(t, tc.want, statusFromStatusCode(tc.httpCode).GetCode())
		})
	}
}

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
