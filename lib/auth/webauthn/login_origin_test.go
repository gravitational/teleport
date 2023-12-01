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

package webauthn_test

import (
	"testing"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

func TestValidateOrigin(t *testing.T) {
	tests := []struct {
		name         string
		origin, rpID string
		wantErr      bool
	}{
		{
			name:   "OK: Origin host same as RPID",
			origin: "https://example.com:3080",
			rpID:   "example.com",
		},
		{
			name:   "OK: Origin without port",
			origin: "https://example.com",
			rpID:   "example.com",
		},
		{
			name:   "OK: Origin is a subdomain of RPID",
			origin: "https://teleport.example.com:3080",
			rpID:   "example.com",
		},
		{
			name:    "NOK: Origin empty",
			origin:  "",
			rpID:    "example.com",
			wantErr: true,
		},
		{
			name:    "NOK: Origin without host",
			origin:  "/this/is/a/path",
			rpID:    "example.com",
			wantErr: true,
		},
		{
			name:    "NOK: Origin has different host",
			origin:  "https://someotherplace.com:3080",
			rpID:    "example.com",
			wantErr: true,
		},
		{
			name:    "NOK: Origin with subdomain and different host",
			origin:  "https://teleport.someotherplace.com:3080",
			rpID:    "example.com",
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := wanlib.ValidateOrigin(test.origin, test.rpID)
			hasErr := err != nil
			if hasErr != test.wantErr {
				t.Fatalf("ValidateOrigin() returned err %q, wantErr = %v", err, test.wantErr)
			}
		})
	}
}
