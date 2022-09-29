/*
Copyright 2022 Gravitational, Inc.

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
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestNewProvisionToken(t *testing.T) {
	name := "foo"
	roles := SystemRoles{RoleNode}
	expires := time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)
	tok, err := NewProvisionToken(name, roles, expires)
	require.NoError(t, err)
	require.Equal(t, &ProvisionTokenV3{
		Kind:    KindToken,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Expires:   &expires,
			Namespace: defaults.Namespace,
		},
		Spec: ProvisionTokenSpecV3{
			JoinMethod: JoinMethodToken,
			Roles:      roles,
		},
	}, tok)
}

func TestNewProvisionTokenFromSpec(t *testing.T) {
	name := "foo"
	expires := time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)
	spec := ProvisionTokenSpecV3{
		Roles:      SystemRoles{RoleNop},
		JoinMethod: JoinMethodToken,
	}
	tok, err := NewProvisionTokenFromSpec(name, expires, spec)
	require.NoError(t, err)
	require.Equal(t, &ProvisionTokenV3{
		Kind:    KindToken,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Expires:   &expires,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}, tok)
}

func TestProvisionTokenV1_V3(t *testing.T) {
	roles := SystemRoles{RoleNop}
	name := "foo-tok"
	expires := time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)
	v1 := ProvisionTokenV1{
		Roles:   roles,
		Token:   name,
		Expires: expires,
	}

	v3 := v1.V3()
	require.Equal(t, &ProvisionTokenV3{
		Kind:    KindToken,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Expires:   &expires,
			Namespace: defaults.Namespace,
		},
		Spec: ProvisionTokenSpecV3{
			Roles:      roles,
			JoinMethod: JoinMethodToken,
		},
	}, v3)
}

func TestProvisionTokenV2_V3(t *testing.T) {
	tests := []struct {
		name  string
		token *ProvisionTokenV2
		want  *ProvisionTokenV3
	}{
		{
			name: "token",
			token: &ProvisionTokenV2{
				Kind:    KindToken,
				Version: V2,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      SystemRoles{RoleBot},
					JoinMethod: JoinMethodToken,
					BotName:    "bot-foo",
					SuggestedLabels: Labels{
						"label": []string{"foo"},
					},
				},
			},
			want: &ProvisionTokenV3{
				Kind:    KindToken,
				Version: V3,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV3{
					Roles:      SystemRoles{RoleBot},
					JoinMethod: JoinMethodToken,
					BotName:    "bot-foo",
					SuggestedLabels: Labels{
						"label": []string{"foo"},
					},
				},
			},
		},
		{
			name: "iam",
			token: &ProvisionTokenV2{
				Kind:    KindToken,
				Version: V2,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      SystemRoles{RoleNop},
					JoinMethod: JoinMethodIAM,
					Allow: []*TokenRule{
						{
							AWSAccount: "xyzzy",
							AWSARN:     "arn-123",
						},
					},
				},
			},
			want: &ProvisionTokenV3{
				Kind:    KindToken,
				Version: V3,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV3{
					Roles:      SystemRoles{RoleNop},
					JoinMethod: JoinMethodIAM,
					IAM: &ProvisionTokenSpecV3AWSIAM{
						Allow: []*ProvisionTokenSpecV3AWSIAM_Rule{
							{
								Account: "xyzzy",
								ARN:     "arn-123",
							},
						},
					},
				},
			},
		},
		{
			name: "ec2",
			token: &ProvisionTokenV2{
				Kind:    KindToken,
				Version: V2,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      SystemRoles{RoleNop},
					JoinMethod: JoinMethodEC2,
					AWSIIDTTL:  NewDuration(time.Second * 37),
					Allow: []*TokenRule{
						{
							AWSAccount: "a-account",
							AWSRegions: []string{"a-region"},
							AWSRole:    "a-role",
						},
					},
				},
			},
			want: &ProvisionTokenV3{
				Kind:    KindToken,
				Version: V3,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV3{
					Roles:      SystemRoles{RoleNop},
					JoinMethod: JoinMethodEC2,
					EC2: &ProvisionTokenSpecV3AWSEC2{
						IIDTTL: NewDuration(time.Second * 37),
						Allow: []*ProvisionTokenSpecV3AWSEC2_Rule{
							{
								Account: "a-account",
								Regions: []string{"a-region"},
								RoleARN: "a-role",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v3 := tt.token.V3()
			require.Equal(t, tt.want, v3)
		})
	}
}

func validProvisionTokenV3(modifier func(p *ProvisionTokenV3)) *ProvisionTokenV3 {
	token := &ProvisionTokenV3{
		Kind:    KindToken,
		Version: V3,
		Metadata: Metadata{
			Name:      "foo",
			Namespace: defaults.Namespace,
		},
		Spec: ProvisionTokenSpecV3{
			Roles:      SystemRoles{RoleNop},
			JoinMethod: JoinMethodToken,
		},
	}
	if modifier != nil {
		modifier(token)
	}
	return token
}

func TestProvisionTokenV3_CheckAndSetDefaults(t *testing.T) {
	tests := []struct {
		name  string
		token *ProvisionTokenV3
		// want indicates the token that should be present after the validation
		// has been called. If this is not provided, the original value of token
		// is used.
		want    *ProvisionTokenV3
		wantErr error
	}{
		{
			name:  "valid token",
			token: validProvisionTokenV3(nil),
		},
		{
			name: "invalid missing roles",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.Roles = SystemRoles{}
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "invalid non-existent role",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.Roles = SystemRoles{"supreme_leader"}
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "valid bot",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.Roles = SystemRoles{RoleBot}
				p.Spec.BotName = "a_bot"
			}),
		},
		{
			name: "invalid missing bot name",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.Roles = SystemRoles{RoleBot}
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "invalid bot name set but not bot token",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.BotName = "set_by_mistake"
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "missing join method",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = ""
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "invalid join method",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = "ethereal-presence"
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "missing iam configuration",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = JoinMethodIAM
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "missing ec2 configuration",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = JoinMethodEC2
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "valid iam configuration",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = JoinMethodIAM
				p.Spec.IAM = &ProvisionTokenSpecV3AWSIAM{
					Allow: []*ProvisionTokenSpecV3AWSIAM_Rule{
						{
							Account: "foo",
						},
					},
				}
			}),
		},
		{
			name: "valid ec2 configuration",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = JoinMethodEC2
				p.Spec.EC2 = &ProvisionTokenSpecV3AWSEC2{
					Allow: []*ProvisionTokenSpecV3AWSEC2_Rule{
						{
							Account: "foo",
						},
					},
					IIDTTL: NewDuration(time.Minute * 12),
				}
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure we run check n set on a clone - so we can check
			// for no changes
			token := proto.Clone(tt.token).(*ProvisionTokenV3)
			err := token.CheckAndSetDefaults()
			if tt.wantErr != nil {
				require.ErrorAs(t, err, &tt.wantErr)
				return
			}
			require.NoError(t, err)

			want := tt.want
			if want == nil {
				want = tt.token
			}
			require.Equal(t, want, token)
		})
	}
}

func TestProvisionTokenV3_GetAllowRules(t *testing.T) {
	tests := []struct {
		name  string
		token ProvisionTokenV3
		want  []*TokenRule
	}{
		{
			name: "ec2",
			token: ProvisionTokenV3{
				Spec: ProvisionTokenSpecV3{
					JoinMethod: JoinMethodEC2,
					EC2: &ProvisionTokenSpecV3AWSEC2{
						Allow: []*ProvisionTokenSpecV3AWSEC2_Rule{
							{
								Account: "foo",
								Regions: []string{"eu-west-666", "us-coast-612"},
								RoleARN: "a-role",
							},
							{
								Account: "bar",
								Regions: []string{"a-region"},
								RoleARN: "b-role",
							},
						},
					},
				},
			},
			want: []*TokenRule{
				{
					AWSAccount: "foo",
					AWSRegions: []string{"eu-west-666", "us-coast-612"},
					AWSRole:    "a-role",
				},
				{
					AWSAccount: "bar",
					AWSRegions: []string{"a-region"},
					AWSRole:    "b-role",
				},
			},
		},
		{
			name: "iam",
			token: ProvisionTokenV3{
				Spec: ProvisionTokenSpecV3{
					JoinMethod: JoinMethodIAM,
					IAM: &ProvisionTokenSpecV3AWSIAM{
						Allow: []*ProvisionTokenSpecV3AWSIAM_Rule{
							{
								Account: "foo",
								ARN:     "arn-amazon-foo",
							},
							{
								Account: "bar",
								ARN:     "arn-amazon-bar",
							},
						},
					},
				},
			},
			want: []*TokenRule{
				{
					AWSAccount: "foo",
					AWSARN:     "arn-amazon-foo",
				},
				{
					AWSAccount: "bar",
					AWSARN:     "arn-amazon-bar",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := tt.token.GetAllowRules()
			require.Equal(t, tt.want, rules)
		})
	}
}

func TestProvisionTokenV3_GetAWSIIDTTL(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		p := ProvisionTokenV3{
			Spec: ProvisionTokenSpecV3{},
		}
		require.Equal(t, Duration(0), p.GetAWSIIDTTL())
	})

	t.Run("set", func(t *testing.T) {
		duration := NewDuration(time.Second * 6)
		p := ProvisionTokenV3{
			Spec: ProvisionTokenSpecV3{
				EC2: &ProvisionTokenSpecV3AWSEC2{
					IIDTTL: duration,
				},
			},
		}
		got := p.GetAWSIIDTTL()
		require.Equal(t, duration, got)
	})
}

func TestProvisionTokenV3_V2(t *testing.T) {
	tests := []struct {
		name      string
		token     *ProvisionTokenV3
		want      *ProvisionTokenV2
		wantError string
	}{
		{
			name: "token",
			token: &ProvisionTokenV3{
				Kind:    KindToken,
				Version: V3,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV3{
					Roles:      SystemRoles{RoleNop},
					BotName:    "foo",
					JoinMethod: JoinMethodToken,
					SuggestedLabels: Labels{
						"foo": []string{"bar"},
					},
				},
			},
			want: &ProvisionTokenV2{
				Kind:    KindToken,
				Version: V2,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      SystemRoles{RoleNop},
					BotName:    "foo",
					JoinMethod: JoinMethodToken,
					SuggestedLabels: Labels{
						"foo": []string{"bar"},
					},
					Allow: []*TokenRule{},
				},
			},
		},
		{
			name: "ec2",
			token: &ProvisionTokenV3{
				Kind:    KindToken,
				Version: V3,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV3{
					Roles:      SystemRoles{RoleNop},
					JoinMethod: JoinMethodEC2,
					EC2: &ProvisionTokenSpecV3AWSEC2{
						IIDTTL: NewDuration(time.Minute * 300),
						Allow: []*ProvisionTokenSpecV3AWSEC2_Rule{
							{
								Account: "xyzzy",
								Regions: []string{"eurasia-1"},
								RoleARN: "lord-commander",
							},
						},
					},
				},
			},
			want: &ProvisionTokenV2{
				Kind:    KindToken,
				Version: V2,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      SystemRoles{RoleNop},
					JoinMethod: JoinMethodEC2,
					Allow: []*TokenRule{
						{
							AWSAccount: "xyzzy",
							AWSRegions: []string{"eurasia-1"},
							AWSRole:    "lord-commander",
						},
					},
					AWSIIDTTL: NewDuration(time.Minute * 300),
				},
			},
		},
		{
			name: "iam",
			token: &ProvisionTokenV3{
				Kind:    KindToken,
				Version: V3,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV3{
					Roles:      SystemRoles{RoleNop},
					JoinMethod: JoinMethodIAM,
					IAM: &ProvisionTokenSpecV3AWSIAM{
						Allow: []*ProvisionTokenSpecV3AWSIAM_Rule{
							{
								Account: "xyzzy",
								ARN:     "arn-123",
							},
						},
					},
				},
			},
			want: &ProvisionTokenV2{
				Kind:    KindToken,
				Version: V2,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      SystemRoles{RoleNop},
					JoinMethod: JoinMethodIAM,
					Allow: []*TokenRule{
						{
							AWSAccount: "xyzzy",
							AWSARN:     "arn-123",
						},
					},
				},
			},
		},
		{
			name: "inconvertible type",
			token: &ProvisionTokenV3{
				Kind:    KindToken,
				Version: V3,
				Metadata: Metadata{
					Name: "foo",
				},
				Spec: ProvisionTokenSpecV3{
					Roles:      SystemRoles{RoleNop},
					JoinMethod: "join-method-does-not-exist",
					IAM: &ProvisionTokenSpecV3AWSIAM{
						Allow: []*ProvisionTokenSpecV3AWSIAM_Rule{
							{
								Account: "xyzzy",
								ARN:     "arn-123",
							},
						},
					},
				},
			},
			want:      nil,
			wantError: ProvisionTokenNotBackwardsCompatibleErr.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.token.V2()
			require.Equal(t, tt.want, got)
			if tt.wantError != "" {
				require.EqualError(t, err, tt.wantError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
