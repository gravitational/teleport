// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package joining_test

import (
	"fmt"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes/joining"
)

func TestValidateScopedToken(t *testing.T) {
	// baseToken contains a valid scoped token using the token join method.
	// It's used as a base for constructing scoped tokens in various valid
	// and invalid states.
	baseToken := &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Scope:   "/aa/bb",
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "testtoken",
		},
		Spec: &joiningv1.ScopedTokenSpec{
			Roles:         []string{types.RoleNode.String()},
			AssignedScope: "/aa/bb",
			JoinMethod:    string(types.JoinMethodToken),
			UsageMode:     string(joining.TokenUsageModeUnlimited),
			ImmutableLabels: &joiningv1.ImmutableLabels{
				Ssh: map[string]string{
					"one":   "1",
					"two":   "2",
					"three": "3",
				},
			},
		},
		Status: &joiningv1.ScopedTokenStatus{
			Secret: "secret",
		},
	}
	cases := []struct {
		name              string
		modFn             func(*joiningv1.ScopedToken)
		expectedStrongErr string
		expectedWeakErr   string
	}{
		{
			name: "invalid kind",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Kind = ""
			},
			expectedStrongErr: fmt.Sprintf("expected kind %v, got %q", types.KindScopedToken, ""),
		},
		{
			name: "invalid version",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Version = ""
			},
			expectedStrongErr: fmt.Sprintf("expected version %v, got %q", types.V1, ""),
		},
		{
			name: "invalid subkind",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.SubKind = "subkind"
			},
			expectedStrongErr: fmt.Sprintf("expected sub_kind %v, got %q", "", "subkind"),
		},
		{
			name: "missing name",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Metadata.Name = ""
			},
			expectedStrongErr: "missing name",
		},
		{
			name: "missing spec",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec = nil
			},
			expectedStrongErr: "spec must not be nil",
			expectedWeakErr:   "validating scoped token assigned scope",
		},
		{
			name: "missing scope",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Scope = ""
			},
			expectedStrongErr: "scoped token must have a scope assigned",
			expectedWeakErr:   "validating scoped token resource scope",
		},
		{
			name: "non-absolute scope",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Scope = "aa/bb"
			},
			expectedStrongErr: "validating scoped token resource scope",
		},
		{
			name: "scope with invalid characters",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Scope = "/aa/bb}"
			},
			expectedStrongErr: "validating scoped token resource scope",
			expectedWeakErr:   "validating scoped token resource scope",
		},
		{
			name: "missing assigned scope",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.AssignedScope = ""
			},
			expectedStrongErr: "validating scoped token assigned scope",
			expectedWeakErr:   "validating scoped token assigned scope",
		},
		{
			name: "non-absolute assigned scope",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.AssignedScope = "aa/bb"
			},
			expectedStrongErr: "validating scoped token assigned scope",
		},
		{
			name: "assigned scope with invalid character",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.AssignedScope = "aa/bb}"
			},
			expectedStrongErr: "validating scoped token assigned scope",
			expectedWeakErr:   "validating scoped token assigned scope",
		},
		{
			name: "assigned scope is not descendant of token scope",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.AssignedScope = "/bb/aa"
			},
			expectedStrongErr: "scoped token assigned scope must be descendant of its resource scope",
		},
		{
			name: "invalid join method",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodUnspecified)
			},
			expectedStrongErr: fmt.Sprintf("join method %q does not support scoping", types.JoinMethodUnspecified),
			expectedWeakErr:   fmt.Sprintf("join method %q does not support scoping", types.JoinMethodUnspecified),
		},
		{
			name: "missing roles",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.Roles = nil
			},
			expectedStrongErr: "scoped token must have at least one role",
			expectedWeakErr:   "scoped token must have at least one role",
		},
		{
			name: "invalid roles",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.Roles = []string{"random_role"}
			},
			expectedStrongErr: "validating scoped token roles",
		},
		{
			name: "unsupported roles",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.Roles = []string{types.RoleNode.String(), types.RoleInstance.String()}
			},
			expectedStrongErr: fmt.Sprintf("role %q does not support scoping", types.RoleInstance),
		},
		{
			name: "invalid usage mode",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.UsageMode = "invalid"
			},
			expectedStrongErr: "scoped token mode is not supported",
		},
		{
			name: "invalid labels key",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.ImmutableLabels = &joiningv1.ImmutableLabels{
					Ssh: map[string]string{
						"one":  "1",
						"two;": "2",
					},
				}
			},
			expectedStrongErr: "invalid immutable label key \"two;\"",
		},
		{
			name: "setting ssh labels for role other than node",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.Roles = []string{types.RoleBot.String()}
				tok.Spec.ImmutableLabels = &joiningv1.ImmutableLabels{
					Ssh: map[string]string{
						"one":   "1",
						"two":   "2",
						"three": "3",
					},
				}
			},
			expectedStrongErr: "immutable ssh labels are only supported for tokens that allow the node role",
		},
		{
			name: "no secret with token join method",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Status = nil
			},
			expectedStrongErr: "secret value must be defined for a scoped token when using the token join method",
			expectedWeakErr:   "secret value must be defined for a scoped token when using the token join method",
		},
		{
			name: "ec2 token without aws configuration",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodEC2)
			},
			expectedStrongErr: "aws configuration must be defined for a scoped token when using the ec2 or iam join methods",
			expectedWeakErr:   "aws configuration must be defined for a scoped token when using the ec2 or iam join methods",
		},
		{
			name: "iam token without aws configuration",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodIAM)
			},
			expectedStrongErr: "aws configuration must be defined for a scoped token when using the ec2 or iam join methods",
			expectedWeakErr:   "aws configuration must be defined for a scoped token when using the ec2 or iam join methods",
		},
		{
			name: "gcp token without gcp configuration",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodGCP)
			},
			expectedStrongErr: "gcp configuration must be defined for a scoped token when using the gcp join method",
			expectedWeakErr:   "gcp configuration must be defined for a scoped token when using the gcp join method",
		},
		{
			name: "azure token without azure configuration",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodAzure)
			},
			expectedStrongErr: "azure configuration must be defined for a scoped token when using the azure join method",
			expectedWeakErr:   "azure configuration must be defined for a scoped token when using the azure join method",
		},
		{
			name: "azure_devops token without azure configuration",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodAzureDevops)
			},
			expectedStrongErr: "azure_devops configuration must be defined for a scoped token when using the azure_devops join method",
			expectedWeakErr:   "azure_devops configuration must be defined for a scoped token when using the azure_devops join method",
		},
		{
			name: "oracle token without oracle configuration",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodOracle)
			},
			expectedStrongErr: "oracle configuration must be defined for a scoped token when using the oracle join method",
			expectedWeakErr:   "oracle configuration must be defined for a scoped token when using the oracle join method",
		},
		{
			name: "valid scoped token",
		},
		{
			name: "valid ec2 scoped token",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodEC2)
				tok.Spec.Aws = &joiningv1.AWS{
					Allow: []*joiningv1.AWS_Rule{
						{
							AwsAccount: "1234567890",
						},
					},
				}
			},
		},
		{
			name: "valid iam scoped token",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodIAM)
				tok.Spec.Aws = &joiningv1.AWS{
					Allow: []*joiningv1.AWS_Rule{
						{
							AwsAccount: "1234567890",
						},
					},
				}
			},
		},
		{
			name: "valid gcp scoped token",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodGCP)
				tok.Spec.Gcp = &joiningv1.GCP{
					Allow: []*joiningv1.GCP_Rule{
						{
							ProjectIds: []string{"1234567890"},
						},
					},
				}
			},
		},
		{
			name: "valid azure scoped token",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodAzure)
				tok.Spec.Azure = &joiningv1.Azure{
					Allow: []*joiningv1.Azure_Rule{
						{
							Subscription: "1234567890",
						},
					},
				}
			},
		},
		{
			name: "valid azure_devops scoped token",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodAzureDevops)
				tok.Spec.AzureDevops = &joiningv1.AzureDevops{
					Allow: []*joiningv1.AzureDevops_Rule{
						{
							Sub: "1234567890",
						},
					},
				}
			},
		},
		{
			name: "valid oracle scoped token",
			modFn: func(tok *joiningv1.ScopedToken) {
				tok.Spec.JoinMethod = string(types.JoinMethodOracle)
				tok.Spec.Oracle = &joiningv1.Oracle{
					Allow: []*joiningv1.Oracle_Rule{
						{
							Tenancy: "1234567890",
						},
					},
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tok := proto.CloneOf(baseToken)
			if c.modFn != nil {
				c.modFn(tok)
			}
			err := joining.StrongValidateToken(tok)
			if c.expectedStrongErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, c.expectedStrongErr)
			}

			err = joining.WeakValidateToken(tok)
			if c.expectedWeakErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, c.expectedWeakErr)
			}
		})
	}
}

func TestImmutableLabelHashing(t *testing.T) {
	labels := &joiningv1.ImmutableLabels{
		Ssh: map[string]string{
			"one":   "1",
			"two":   "2",
			"hello": "world",
		},
	}

	// assert that the same labels match with their hash
	initialHash := joining.HashImmutableLabels(labels)
	require.True(t, joining.VerifyImmutableLabelsHash(proto.CloneOf(labels), initialHash))

	// assert that changing a label value fails the hash check
	labels.Ssh["hello"] = "other"
	require.False(t, joining.VerifyImmutableLabelsHash(proto.CloneOf(labels), initialHash))

	// assert that adding a label fails the hash check
	labels.Ssh["three"] = "3"
	require.False(t, joining.VerifyImmutableLabelsHash(proto.CloneOf(labels), initialHash))
}

func TestImmutableLabelHashCollision(t *testing.T) {
	// Assert labels that could feasibly result in the same set of strings in the same order do not collide
	// unless they're the exact same keys and values. Represented as a slice of test cases to make it easier
	// to extend once immutable labels are made up of more than SSH labels.
	cases := []struct {
		name    string
		labelsA *joiningv1.ImmutableLabels
		labelsB *joiningv1.ImmutableLabels
	}{
		{
			// guards against map entries being naively concatenated as they're hashed. e.g.
			// aaa=bbbcccddd should not collide with aaa=bbb,ccc=ddd
			name: "split label concatenation",
			labelsA: &joiningv1.ImmutableLabels{
				Ssh: map[string]string{
					"aaa": "bbbcccddd",
				},
			},

			labelsB: &joiningv1.ImmutableLabels{
				Ssh: map[string]string{
					"aaa": "bbb",
					"ccc": "ddd",
				},
			},
		},
		{
			// guards against single entries being naively concatenated as they're hashed. e.g.
			// aaa=bbb should not collide with aaab=bb
			name: "single label concatenation",
			labelsA: &joiningv1.ImmutableLabels{
				Ssh: map[string]string{
					"aaa": "bbb",
				},
			},

			labelsB: &joiningv1.ImmutableLabels{
				Ssh: map[string]string{
					"aaab": "bb",
				},
			},
		},
		// TODO (eriktate): add test case for identical labels applied to different service types once immutable
		// labels support more than SSH
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hashA := joining.HashImmutableLabels(c.labelsA)
			require.False(t, joining.VerifyImmutableLabelsHash(c.labelsB, hashA))
		})
	}
}

// TestImmutableLabelHashGolden tests the immutable labels hashing implementation against a set of known-good hashes
// to help guard against regressions.
func TestImmutableLabelHashGolden(t *testing.T) {
	cases := []struct {
		name   string
		labels *joiningv1.ImmutableLabels
		hash   string
	}{
		{
			name: "single ssh label",
			labels: &joiningv1.ImmutableLabels{
				Ssh: map[string]string{
					"aaa": "bbb",
				},
			},
			hash: "5dd8fad69587f17535a4dea3ab41400914c3fbecd1972d4e194b1c18c0f4c4ff",
		},
		{
			name: "multiple ssh labels",
			labels: &joiningv1.ImmutableLabels{
				Ssh: map[string]string{
					"aaa": "bbb",
					"ccc": "ddd",
					"eee": "fff",
				},
			},
			hash: "b4757712bb94a422f835ca983e9ab3a9ce9925617496e9eeea676fb65b28f2b9",
		},
		{
			name: "empty labels",
			labels: &joiningv1.ImmutableLabels{
				Ssh: map[string]string{},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hash := joining.HashImmutableLabels(c.labels)
			// assert both VerifyImmutableLabelsHash and a regular equality check just in case
			// the VerifyImmutableLabelsHash implementation drifts
			assert.True(t, joining.VerifyImmutableLabelsHash(c.labels, hash))
			assert.Equal(t, c.hash, hash)
		})
	}
}

func FuzzImmutableLabelHash(f *testing.F) {
	f.Add("hello", "world", "foo", "bar", "baz", "qux", true)   // base case
	f.Add("aaa", "bbbcccddd", "aaa", "bbb", "ccc", "ddd", true) // split label concatenation
	f.Add("aaa", "bbb", "aaab", "bb", "", "", false)            // single label concatenation

	f.Fuzz(func(t *testing.T, key1, value1, key2, value2, key3, value3 string, multiLabel bool) {
		labelsA := &joiningv1.ImmutableLabels{
			Ssh: map[string]string{
				key1: value1,
			},
		}
		labelsB := &joiningv1.ImmutableLabels{
			Ssh: map[string]string{
				key2: value2,
			},
		}
		// assign a second label only if multiLabel is true
		if multiLabel {
			labelsB.Ssh[key3] = value3
		}

		// assert we can generate hashes for both labels without panicking
		hashA := joining.HashImmutableLabels(labelsA)
		require.NotEmpty(t, hashA)
		hashB := joining.HashImmutableLabels(labelsB)
		require.NotEmpty(t, hashB)

		// assert that hashes are verified against their own labels
		assert.True(t, joining.VerifyImmutableLabelsHash(proto.CloneOf(labelsA), hashA))
		assert.True(t, joining.VerifyImmutableLabelsHash(proto.CloneOf(labelsB), hashB))

		// assert that the same labels always result in the same hash and different labels always result in different hashes
		assertFn := assert.False
		if maps.Equal(labelsA.Ssh, labelsB.Ssh) {
			assertFn = assert.True
		}

		assertFn(t, joining.VerifyImmutableLabelsHash(proto.CloneOf(labelsA), hashB))
		assertFn(t, joining.VerifyImmutableLabelsHash(proto.CloneOf(labelsB), hashA))
	})
}

func TestValidateTokenUpdate(t *testing.T) {
	baseToken := &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "test-token",
		},
		Scope: "/test",
		Spec: &joiningv1.ScopedTokenSpec{
			AssignedScope: "/test/one",
			JoinMethod:    string(types.JoinMethodToken),
			Roles:         []string{types.RoleNode.String()},
			UsageMode:     string(joining.TokenUsageModeUnlimited),
		},
		Status: &joiningv1.ScopedTokenStatus{
			Secret: "secret-value",
		},
	}

	for _, tc := range []struct {
		name            string
		modifyTokenFunc func(*joiningv1.ScopedToken)
		wantErr         string
	}{
		{
			name: "check scope change",
			modifyTokenFunc: func(t *joiningv1.ScopedToken) {
				t.Scope = "/other"
				t.Spec.AssignedScope = "/other/one"
			},
			wantErr: "cannot modify scope of existing scoped token test-token with scope /test to /other",
		},
		{
			name: "check usage mode change",
			modifyTokenFunc: func(t *joiningv1.ScopedToken) {
				t.Spec.UsageMode = string(joining.TokenUsageModeSingle)
			},
			wantErr: fmt.Sprintf("cannot modify usage mode of existing scoped token test-token from usage mode %s to %s", joining.TokenUsageModeUnlimited, joining.TokenUsageModeSingle),
		},
		{
			name: "check secret change",
			modifyTokenFunc: func(t *joiningv1.ScopedToken) {
				t.Status.Secret = "new-secret-value"
			},
			wantErr: "cannot modify secret of existing scoped token test-token",
		},
		{
			name: "valid update",
			modifyTokenFunc: func(t *joiningv1.ScopedToken) {
				t.Metadata.Labels = map[string]string{"env": "production"}
				t.Spec.AssignedScope = "/test/one/two"
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			modified := proto.CloneOf(baseToken)
			tc.modifyTokenFunc(modified)

			err := joining.ValidateTokenUpdate(baseToken, modified)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}
