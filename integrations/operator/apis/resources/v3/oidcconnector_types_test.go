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

package v3

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
)

// This tests that `redirect_url` is consistently marshaled as a list of string
// This is not the case of wrappers.Strings which marshals as a string if it contains a single element
func TestTeleportOIDCConnectorSpec_MarshalJSON(t *testing.T) {
	tests := []struct {
		name         string
		spec         TeleportOIDCConnectorSpec
		expectedJSON string
	}{
		{
			"Empty string",
			TeleportOIDCConnectorSpec{RedirectURLs: wrappers.Strings{""}},
			`{"redirect_url":[""],"issuer_url":"","client_id":"","client_secret":""}`,
		},
		{
			"Single string",
			TeleportOIDCConnectorSpec{RedirectURLs: wrappers.Strings{"foo"}},
			`{"redirect_url":["foo"],"issuer_url":"","client_id":"","client_secret":""}`,
		},
		{
			"Multiple strings",
			TeleportOIDCConnectorSpec{RedirectURLs: wrappers.Strings{"foo", "bar"}},
			`{"redirect_url":["foo","bar"],"issuer_url":"","client_id":"","client_secret":""}`,
		},
		{
			"MaxAge",
			TeleportOIDCConnectorSpec{MaxAge: &types.MaxAge{Value: types.Duration(time.Hour)}},
			`{"max_age":"1h0m0s","issuer_url":"","client_id":"","client_secret":""}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := json.Marshal(tc.spec)
			require.NoError(t, err)
			require.Equal(t, tc.expectedJSON, string(result))
		})
	}
}
func TestTeleportOIDCConnectorSpec_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name         string
		expectedSpec TeleportOIDCConnectorSpec
		inputJSON    string
	}{
		{
			"Empty string",
			TeleportOIDCConnectorSpec{RedirectURLs: wrappers.Strings{""}},
			`{"redirect_url":[""],"issuer_url":"","client_id":"","client_secret":""}`,
		},
		{
			"Single string",
			TeleportOIDCConnectorSpec{RedirectURLs: wrappers.Strings{"foo"}},
			`{"redirect_url":["foo"],"issuer_url":"","client_id":"","client_secret":""}`,
		},
		{
			"Multiple strings",
			TeleportOIDCConnectorSpec{RedirectURLs: wrappers.Strings{"foo", "bar"}},
			`{"redirect_url":["foo","bar"],"issuer_url":"","client_id":"","client_secret":""}`,
		},
		{
			"MaxAge",
			TeleportOIDCConnectorSpec{MaxAge: &types.MaxAge{Value: types.Duration(time.Hour)}},
			`{"max_age":"1h0m0s","issuer_url":"","client_id":"","client_secret":""}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var spec TeleportOIDCConnectorSpec
			require.NoError(t, json.Unmarshal([]byte(tc.inputJSON), &spec))
			require.Equal(t, tc.expectedSpec, spec)
		})
	}
}
