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
