/*
Copyright 2026 Gravitational, Inc.

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

package types

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newUser returns a fully populated UserV2 with LocalAuth including MFA
// devices of every type.
func newUser(t *testing.T) *UserV2 {
	t.Helper()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	exp := now.Add(24 * time.Hour)
	return &UserV2{
		Kind:    KindUser,
		Version: V2,
		Metadata: Metadata{
			Name:      "alice",
			Namespace: "default",
			Expires:   &exp,
			Labels:    map[string]string{"env": "prod"},
		},
		Spec: UserSpecV2{
			Roles: []string{"admin", "dev"},
			Traits: map[string][]string{
				"logins": {"root", "alice"},
			},
			OIDCIdentities:   []ExternalIdentity{{ConnectorID: "oidc-1", Username: "alice@oidc"}},
			SAMLIdentities:   []ExternalIdentity{{ConnectorID: "saml-1", Username: "alice@saml"}},
			GithubIdentities: []ExternalIdentity{{ConnectorID: "github-1", Username: "alice@github"}},
			CreatedBy: CreatedBy{
				Time: now,
				User: UserRef{Name: "admin"},
			},
			Expires:          now.Add(time.Hour),
			TrustedDeviceIDs: []string{"device-1"},
			Status: LoginStatus{
				IsLocked:      false,
				LockedMessage: "",
			},
			LocalAuth: &LocalAuthSecrets{
				PasswordHash: []byte("hash"),
				TOTPKey:      "totp-key",
				MFA: []*MFADevice{
					mustNewMFADevice(t, "totp-dev", "id-1", now, &MFADevice_Totp{
						Totp: &TOTPDevice{Key: "secret-1"},
					}),
					mustNewMFADevice(t, "webauthn-dev", "id-2", now, &MFADevice_Webauthn{
						Webauthn: &WebauthnDevice{
							CredentialId:  []byte("cred-1"),
							PublicKeyCbor: []byte("cbor-1"),
						},
					}),
				},
				Webauthn: &WebauthnLocalAuth{UserID: []byte("webauthn-user")},
			},
		},
	}
}

func mustNewMFADevice(t *testing.T, name, id string, addedAt time.Time, device isMFADevice_Device) *MFADevice {
	t.Helper()
	d, err := NewMFADevice(name, id, addedAt, device)
	require.NoError(t, err)
	return d
}

func TestUserV2IsEqual(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// mockUser is a minimal implemention of the User interface to exercise
	// the non-*UserV2 type assertion path in IsEqual.
	type mockUser struct {
		UserV2
	}

	tests := []struct {
		name string
		a, b func(t *testing.T) User
		want bool
	}{
		{
			name: "both typed nil *UserV2",
			a: func(t *testing.T) User {
				var u *UserV2
				return u
			},
			b: func(t *testing.T) User {
				var u *UserV2
				return u
			},
			want: true,
		},
		{
			name: "typed nil *UserV2 vs populated user",
			a: func(t *testing.T) User {
				var u *UserV2
				return u
			},
			b:    func(t *testing.T) User { return newUser(t) },
			want: false,
		},
		{
			name: "identical users",
			a:    func(t *testing.T) User { return newUser(t) },
			b:    func(t *testing.T) User { return newUser(t) },
			want: true,
		},
		{
			name: "both nil LocalAuth",
			a: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth = nil
				return u
			},
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth = nil
				return u
			},
			want: true,
		},
		{
			name: "Revision difference ignored",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Metadata.SetRevision("different-revision")
				return u
			},
			want: true,
		},
		{
			name: "top-level Status difference ignored",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Status.PasswordState = 42
				return u
			},
			want: true,
		},
		{
			name: "MFA devices in different order",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				mfa := u.Spec.LocalAuth.MFA
				mfa[0], mfa[1] = mfa[1], mfa[0]
				return u
			},
			want: true,
		},
		{
			name: "non-UserV2 type returns false",
			a:    func(t *testing.T) User { return newUser(t) },
			b:    func(t *testing.T) User { return &mockUser{} },
			want: false,
		},
		{
			name: "different Name",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Metadata.Name = "bob"
				return u
			},
			want: false,
		},
		{
			name: "different Namespace",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Metadata.Namespace = "other"
				return u
			},
			want: false,
		},
		{
			name: "different Labels",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Metadata.Labels = map[string]string{"env": "staging"}
				return u
			},
			want: false,
		},
		{
			name: "different Metadata.Expires",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				exp := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
				u.Metadata.Expires = &exp
				return u
			},
			want: false,
		},
		{
			name: "different Roles",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.Roles = []string{"viewer"}
				return u
			},
			want: false,
		},
		{
			name: "different Traits",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.Traits = map[string][]string{"logins": {"nobody"}}
				return u
			},
			want: false,
		},
		{
			name: "different OIDCIdentities",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.OIDCIdentities = []ExternalIdentity{{ConnectorID: "other", Username: "x"}}
				return u
			},
			want: false,
		},
		{
			name: "different SAMLIdentities",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.SAMLIdentities = []ExternalIdentity{{ConnectorID: "other", Username: "x"}}
				return u
			},
			want: false,
		},
		{
			name: "different GithubIdentities",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.GithubIdentities = []ExternalIdentity{{ConnectorID: "other", Username: "x"}}
				return u
			},
			want: false,
		},
		{
			name: "different CreatedBy",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.CreatedBy = CreatedBy{User: UserRef{Name: "other"}}
				return u
			},
			want: false,
		},
		{
			name: "different Spec.Expires",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.Expires = time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
				return u
			},
			want: false,
		},
		{
			name: "different TrustedDeviceIDs",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.TrustedDeviceIDs = []string{"device-other"}
				return u
			},
			want: false,
		},
		{
			name: "different Spec.Status (LoginStatus)",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.Status = LoginStatus{IsLocked: true, LockedMessage: "banned"}
				return u
			},
			want: false,
		},
		{
			name: "different PasswordHash",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth.PasswordHash = []byte("other")
				return u
			},
			want: false,
		},
		{
			name: "different TOTPKey",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth.TOTPKey = "other"
				return u
			},
			want: false,
		},
		{
			name: "different Webauthn LocalAuth",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth.Webauthn = &WebauthnLocalAuth{UserID: []byte("other")}
				return u
			},
			want: false,
		},
		{
			name: "nil vs non-nil LocalAuth",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth = nil
				return u
			},
			want: false,
		},
		{
			name: "different MFA device count",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth.MFA = u.Spec.LocalAuth.MFA[:1]
				return u
			},
			want: false,
		},
		{
			name: "different TOTP key in MFA device",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth.MFA[0].Device.(*MFADevice_Totp).Totp.Key = "changed"
				return u
			},
			want: false,
		},
		{
			name: "different Webauthn CredentialId in MFA device",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth.MFA[1].Device.(*MFADevice_Webauthn).Webauthn.CredentialId = []byte("changed")
				return u
			},
			want: false,
		},
		{
			name: "swapped device type (TOTP replaced with SSO)",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth.MFA[0] = mustNewMFADevice(t, "totp-dev", "id-1", now, &MFADevice_Sso{
					Sso: &SSOMFADevice{ConnectorId: "c", ConnectorType: "saml", DisplayName: "d"},
				})
				return u
			},
			want: false,
		},
		{
			name: "nil MFA vs non-nil MFA",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth.MFA = nil
				return u
			},
			want: false,
		},
		{
			name: "empty MFA vs populated MFA",
			a:    func(t *testing.T) User { return newUser(t) },
			b: func(t *testing.T) User {
				u := newUser(t)
				u.Spec.LocalAuth.MFA = []*MFADevice{}
				return u
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.a(t)
			b := tt.b(t)
			require.Equal(t, tt.want, a.IsEqual(b))
		})
	}
}

// TestUserMatchSearch tests the SearchKeywords filter for users, which includes keys and values
// for labels and traits.
func TestUserMatchSearch(t *testing.T) {
	u := newUser(t)

	tests := []struct {
		name           string
		searchKeywords []string
		want           bool
	}{
		{
			name:           "match empty search",
			searchKeywords: []string{""},
			want:           true,
		},
		{
			name:           "match by name",
			searchKeywords: []string{"alice"},
			want:           true,
		},
		{
			name:           "match by label",
			searchKeywords: []string{"env", "prod"},
			want:           true,
		},
		{
			name:           "match by role",
			searchKeywords: []string{"admin", "dev"},
			want:           true,
		},
		{
			name:           "match by trait",
			searchKeywords: []string{"logins", "root"},
			want:           true,
		},
		{
			name:           "match none",
			searchKeywords: []string{"fake"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, u.MatchSearch(tt.searchKeywords))
		})
	}
}

func TestUserMatchTraits(t *testing.T) {
	u := newUser(t)

	tests := []struct {
		name   string
		traits map[string][]string
		want   bool
	}{
		{
			name:   "match nil",
			traits: nil,
			want:   true,
		},
		{
			name:   "match empty",
			traits: map[string][]string{},
			want:   true,
		},
		{
			name:   "match full",
			traits: map[string][]string{"logins": {"root", "alice"}},
			want:   true,
		},
		{
			name:   "match subset",
			traits: map[string][]string{"logins": {"alice"}},
			want:   true,
		},
		{
			name:   "match none",
			traits: map[string][]string{"logins": {"nobody"}},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, u.MatchTraits(tt.traits))
		})
	}
}

func TestUserDisplayValues(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		traits        map[string][]string
		labels        map[string]string
		wantPrimary   string
		wantSecondary string
	}{
		{
			name:     "Okta traits take precedence over Entra ID, generic, and SCIM attrs",
			username: "123456",
			traits: map[string][]string{
				displayNameTrait:        {"Alice Generic"},
				emailTrait:              {"generic@example.com"},
				entraIDDisplayNameTrait: {"Alice Entra"},
				entraIDEmailTrait:       {"entra@example.com"},
				oktaDisplayNameTrait:    {"Alice Adams"},
				oktaEmailTrait:          {"alice@example.com"},
			},
			labels: map[string]string{
				scimAttrsLabel: `{"displayName":"Alice SCIM","emails":[{"primary":true,"value":"scim@example.com"}]}`,
			},
			wantPrimary:   "Alice Adams",
			wantSecondary: "alice@example.com",
		},
		{
			name:     "Okta given and family fallback",
			username: "123456",
			traits: map[string][]string{
				oktaGivenNameTrait:  {"Alice"},
				oktaFamilyNameTrait: {"Adams"},
			},
			wantPrimary: "Alice Adams",
		},
		{
			name:     "Okta first and last name fallback",
			username: "123456",
			traits: map[string][]string{
				oktaFirstNameTrait: {"Alice"},
				oktaLastNameTrait:  {"Adams"},
			},
			wantPrimary: "Alice Adams",
		},
		{
			name:     "single Okta name part fallback",
			username: "123456",
			traits: map[string][]string{
				oktaGivenNameTrait: {"Alice"},
			},
			wantPrimary: "Alice",
		},
		{
			name:     "generic traits take precedence over SCIM attrs",
			username: "123456",
			traits: map[string][]string{
				displayNameTrait: {"Generic Person"},
				emailTrait:       {"generic@example.com"},
			},
			labels: map[string]string{
				scimAttrsLabel: `{"displayName":"Alice SCIM","emails":[{"value":"backup@example.com"},{"primary":true,"value":"alice@example.com"}]}`,
			},
			wantPrimary:   "Generic Person",
			wantSecondary: "generic@example.com",
		},
		{
			name:     "SCIM display attrs",
			username: "123456",
			labels: map[string]string{
				scimAttrsLabel: `{"displayName":"Alice SCIM","emails":[{"value":"backup@example.com"},{"primary":true,"value":"alice@example.com"}]}`,
			},
			wantPrimary:   "Alice SCIM",
			wantSecondary: "alice@example.com",
		},
		{
			name:     "SCIM name fallback and first email",
			username: "123456",
			labels: map[string]string{
				scimAttrsLabel: `{"name":{"givenName":"Alice","familyName":"SCIM"},"emails":[{"value":"alice@example.com"},{"value":"backup@example.com"}]}`,
			},
			wantPrimary:   "Alice SCIM",
			wantSecondary: "alice@example.com",
		},
		{
			name:     "Entra ID synced traits take precedence over generic traits and SCIM attrs",
			username: "123456",
			traits: map[string][]string{
				nameTrait:               {"Alice Name"},
				displayNameTrait:        {"Alice Generic"},
				emailTrait:              {"generic@example.com"},
				entraIDDisplayNameTrait: {"Alice Entra"},
				entraIDEmailTrait:       {"alice@example.com"},
			},
			labels: map[string]string{
				scimAttrsLabel: `{"displayName":"Alice SCIM","emails":[{"primary":true,"value":"scim@example.com"}]}`,
			},
			wantPrimary:   "Alice Entra",
			wantSecondary: "alice@example.com",
		},
		{
			name:     "username dedupe",
			username: "alice@example.com",
			traits: map[string][]string{
				oktaDisplayNameTrait: {"alice@example.com"},
				oktaEmailTrait:       {"alice@example.com"},
			},
			wantPrimary:   "",
			wantSecondary: "",
		},
		{
			name:     "username dedupe falls through to next display candidates",
			username: "alice@example.com",
			traits: map[string][]string{
				oktaDisplayNameTrait: {"alice@example.com"},
				oktaGivenNameTrait:   {"Alice"},
				oktaFamilyNameTrait:  {"Adams"},
				oktaEmailTrait:       {"alice@example.com"},
				emailTrait:           {"alice.alt@example.com"},
			},
			wantPrimary:   "Alice Adams",
			wantSecondary: "alice.alt@example.com",
		},
		{
			name:     "malformed SCIM attrs",
			username: "123456",
			labels: map[string]string{
				scimAttrsLabel: `{"displayName"`,
			},
			wantPrimary:   "",
			wantSecondary: "",
		},
		{
			name:          "missing values",
			username:      "123456",
			wantPrimary:   "",
			wantSecondary: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := NewUser(tt.username)
			require.NoError(t, err)
			u.SetTraits(tt.traits)
			u.SetStaticLabels(tt.labels)

			display := u.GetDisplay()
			require.Equal(t, tt.wantPrimary, display.Primary)
			require.Equal(t, tt.wantSecondary, display.Secondary)
		})
	}
}

func BenchmarkMatchSearch(b *testing.B) {
	const count = 50

	u := &UserV2{
		Kind:    KindUser,
		Version: V2,
		Metadata: Metadata{
			Name:      "alice",
			Namespace: "default",
		},
	}

	u.Metadata.Labels = make(map[string]string, count)
	u.Spec.Traits = make(map[string][]string, count)
	for i := range count {
		u.Metadata.Labels[fmt.Sprintf("label-%d", i)] = fmt.Sprintf("value-%d", i)
		u.Spec.Roles = append(u.Spec.Roles, fmt.Sprintf("role-%d", i))
		u.Spec.Traits[fmt.Sprintf("trait-%d", i)] = []string{
			fmt.Sprintf("trait-value-%d-a", i),
			fmt.Sprintf("trait-value-%d-b", i),
		}
	}

	benchmarks := []struct {
		name           string
		searchKeywords []string
		want           bool
	}{
		{
			name:           "match empty search",
			searchKeywords: []string{""},
			want:           true,
		},
		{
			name:           "match by name",
			searchKeywords: []string{"alice"},
			want:           true,
		},
		{
			name:           "match by label",
			searchKeywords: []string{"label-49", "value-49"},
			want:           true,
		},
		{
			name:           "match by role",
			searchKeywords: []string{"role-49"},
			want:           true,
		},
		{
			name:           "match by trait",
			searchKeywords: []string{"trait-49", "trait-value-49-b"},
			want:           true,
		},
		{
			name:           "match none",
			searchKeywords: []string{"fake"},
			want:           false,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				require.Equal(b, bm.want, u.MatchSearch(bm.searchKeywords))
			}
		})
	}
}
