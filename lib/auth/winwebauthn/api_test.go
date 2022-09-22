package winwebauthn_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncose"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/auth/winwebauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister_errors(t *testing.T) {
	resetNativeAfterTests(t)

	*winwebauthn.Native = fakeNative{}

	const origin = "https://example.com"
	okCC := &wanlib.CredentialCreation{
		Response: protocol.PublicKeyCredentialCreationOptions{
			Challenge: make([]byte, 32),
			RelyingParty: protocol.RelyingPartyEntity{
				ID: "example.com",
			},
			Parameters: []protocol.CredentialParameter{
				{Type: protocol.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
			},
			AuthenticatorSelection: protocol.AuthenticatorSelection{
				UserVerification: protocol.VerificationDiscouraged,
			},
			Attestation: protocol.PreferNoAttestation,
		},
	}

	pwdlessOK := *okCC
	pwdlessOK.Response.RelyingParty.Name = "Teleport"
	pwdlessOK.Response.User = protocol.UserEntity{
		CredentialEntity: protocol.CredentialEntity{
			Name: "llama",
		},
		DisplayName: "Llama",
		ID:          []byte{1, 2, 3, 4, 5}, // arbitrary
	}
	rrk := true
	pwdlessOK.Response.AuthenticatorSelection.RequireResidentKey = &rrk
	pwdlessOK.Response.AuthenticatorSelection.UserVerification = protocol.VerificationRequired

	tests := []struct {
		name     string
		origin   string
		createCC func() *wanlib.CredentialCreation
		wantErr  string
	}{
		{
			// right now there is not need to provide anything in fakeNative
			name:     "ok",
			origin:   origin,
			createCC: func() *wanlib.CredentialCreation { return okCC },
			wantErr:  "not implemented in fakeNative",
		},
		{
			name:     "nil origin",
			createCC: func() *wanlib.CredentialCreation { return okCC },
			wantErr:  "origin",
		},
		{
			name:     "nil cc",
			origin:   origin,
			createCC: func() *wanlib.CredentialCreation { return nil },
			wantErr:  "credential creation required",
		},
		{
			name:   "cc without challenge",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.Challenge = nil
				return &cp
			},
			wantErr: "challenge",
		},
		{
			name:   "cc without RPID",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := *okCC
				cp.Response.RelyingParty.ID = ""
				return &cp
			},
			wantErr: "relying party ID",
		},
		{
			name:   "rrk empty RP name",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.RelyingParty.Name = ""
				return &cp
			},
			wantErr: "relying party name",
		},
		{
			name:   "rrk empty user name",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.User.Name = ""
				return &cp
			},
			wantErr: "user name",
		},
		{
			name:   "rrk empty user display name",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.User.DisplayName = ""
				return &cp
			},
			wantErr: "user display name",
		},
		{
			name:   "rrk nil user ID",
			origin: origin,
			createCC: func() *wanlib.CredentialCreation {
				cp := pwdlessOK
				cp.Response.User.ID = nil
				return &cp
			},
			wantErr: "user ID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			_, err := winwebauthn.Register(ctx, test.origin, test.createCC())
			require.Error(t, err, "Register returned err = nil, want %q", test.wantErr)
			assert.Contains(t, err.Error(), test.wantErr, "Register returned err = %q, want %q", err, test.wantErr)
		})
	}
}

func TestLogin_errors(t *testing.T) {
	resetNativeAfterTests(t)

	*winwebauthn.Native = fakeNative{}

	const origin = "https://example.com"
	okAssertion := &wanlib.CredentialAssertion{
		Response: protocol.PublicKeyCredentialRequestOptions{
			Challenge:      make([]byte, 32),
			RelyingPartyID: "example.com",
			AllowedCredentials: []protocol.CredentialDescriptor{
				{Type: protocol.PublicKeyCredentialType, CredentialID: []byte{1, 2, 3, 4, 5}},
			},
		},
	}

	nilChallengeAssertion := *okAssertion
	nilChallengeAssertion.Response.Challenge = nil

	emptyRPIDAssertion := *okAssertion
	emptyRPIDAssertion.Response.RelyingPartyID = ""

	tests := []struct {
		name      string
		origin    string
		assertion *wanlib.CredentialAssertion
		wantErr   string
	}{
		{
			// right now there is not need to provide anything in fakeNative
			name:      "ok",
			origin:    origin,
			assertion: okAssertion,
			wantErr:   "not implemented in fakeNative",
		},
		{
			name:      "nil origin",
			assertion: okAssertion,
			wantErr:   "origin",
		},
		{
			name:    "nil assertion",
			origin:  origin,
			wantErr: "assertion required",
		},
		{
			name:      "assertion without challenge",
			origin:    origin,
			assertion: &nilChallengeAssertion,
			wantErr:   "challenge",
		},
		{
			name:      "assertion without RPID",
			origin:    origin,
			assertion: &emptyRPIDAssertion,
			wantErr:   "relying party ID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			_, _, err := winwebauthn.Login(ctx, test.origin, test.assertion, nil /* opts */)
			require.Error(t, err, "Login returned err = nil, want %q", test.wantErr)
			assert.Contains(t, err.Error(), test.wantErr, "Login returned err = %q, want %q", err, test.wantErr)
		})
	}
}

func resetNativeAfterTests(t *testing.T) {
	n := *winwebauthn.Native
	t.Cleanup(func() {
		*winwebauthn.Native = n
	})
}

type fakeNative struct{}

func (f fakeNative) CheckSupport() winwebauthn.CheckSupportResult {
	return winwebauthn.CheckSupportResult{
		HasCompileSupport: true,
		IsAvailable:       true,
	}
}

func (n fakeNative) GetAssertion(origin string, in protocol.PublicKeyCredentialRequestOptions, loginOpts *winwebauthn.LoginOpts) (*wanlib.CredentialAssertionResponse, error) {
	return nil, fmt.Errorf("not implemented in fakeNative")
}

func (n fakeNative) MakeCredential(origin string, in protocol.PublicKeyCredentialCreationOptions) (*wanlib.CredentialCreationResponse, error) {
	return nil, fmt.Errorf("not implemented in fakeNative")
}
