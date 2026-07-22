/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package servicecfg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateJamfCredentials(t *testing.T) {
	const expectedErr = "either username+password or clientID+clientSecret must be provided"
	tests := []struct {
		name    string
		creds   *JamfCredentials
		wantErr string
	}{
		{
			name: "valid credentials with username and password",
			creds: &JamfCredentials{
				Username: "username",
				Password: "password",
			},
		},
		{
			name: "valid credentials with client ID and client secret",
			creds: &JamfCredentials{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			},
		},
		{
			name: "credentials with all fields set",
			creds: &JamfCredentials{
				Username:     "username",
				Password:     "password",
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			},
		},
		{
			name: "invalid credentials missing password",
			creds: &JamfCredentials{
				Username: "username",
			},
			wantErr: expectedErr,
		},
		{
			name: "invalid credentials missing username",
			creds: &JamfCredentials{
				Password: "password",
			},
			wantErr: expectedErr,
		},
		{
			name: "invalid credentials missing client secret",
			creds: &JamfCredentials{
				ClientID: "id",
			},
			wantErr: expectedErr,
		},
		{
			name: "invalid credentials missing client id",
			creds: &JamfCredentials{
				ClientSecret: "secret",
			},
			wantErr: expectedErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJamfCredentials(tt.creds)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
