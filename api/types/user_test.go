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
