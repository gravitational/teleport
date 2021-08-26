/*
Copyright 2021 Gravitational, Inc.

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
