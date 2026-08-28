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

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestSliceOrString(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		t.Run("nil slice", func(t *testing.T) {
			var empty SliceOrString
			bytes, err := json.Marshal(empty)
			require.NoError(t, err)
			require.Equal(t, "[]", string(bytes))
		})

		t.Run("single string", func(t *testing.T) {
			single := SliceOrString{"single"}
			bytes, err := json.Marshal(single)
			require.NoError(t, err)
			require.Equal(t, "\"single\"", string(bytes))
		})

		t.Run("slice", func(t *testing.T) {
			slice := SliceOrString{"e1", "e2"}
			bytes, err := json.Marshal(slice)
			require.NoError(t, err)
			require.Equal(t, "[\"e1\",\"e2\"]", string(bytes))
		})
	})

	t.Run("unmarshal", func(t *testing.T) {
		t.Run("single string", func(t *testing.T) {
			var single SliceOrString
			err := json.Unmarshal([]byte(`"single"`), &single)
			require.NoError(t, err)
			require.Equal(t, SliceOrString{"single"}, single)
		})

		t.Run("slice", func(t *testing.T) {
			var slice SliceOrString
			err := json.Unmarshal([]byte(`["e1", "e2"]`), &slice)
			require.NoError(t, err)
			require.Equal(t, SliceOrString{"e1", "e2"}, slice)
		})

		t.Run("error int", func(t *testing.T) {
			var slice SliceOrString
			err := json.Unmarshal([]byte(`5`), &slice)
			require.Error(t, err)
		})

		t.Run("error invalid json", func(t *testing.T) {
			var slice SliceOrString
			err := json.Unmarshal([]byte(`"e1,`), &slice)
			require.Error(t, err)
		})
	})
}

func TestStringOrMap(t *testing.T) {
	t.Run("marshal", func(t *testing.T) {
		t.Run("nil input", func(t *testing.T) {
			var empty StringOrMap
			bytes, err := json.Marshal(empty)
			require.NoError(t, err)
			require.Equal(t, "{}", string(bytes))
		})

		t.Run("single entity with single entry", func(t *testing.T) {
			in := StringOrMap{"AWS": SliceOrString{"x"}}
			bytes, err := json.Marshal(in)
			require.NoError(t, err)
			require.Equal(t, `{"AWS":"x"}`, string(bytes))
		})
		t.Run("single entity with multiple entries", func(t *testing.T) {
			in := StringOrMap{"AWS": SliceOrString{"x", "y"}}
			bytes, err := json.Marshal(in)
			require.NoError(t, err)
			require.Equal(t, `{"AWS":["x","y"]}`, string(bytes))
		})
		t.Run("multiple entities with multiple entries", func(t *testing.T) {
			in := StringOrMap{
				"AWS":       SliceOrString{"x", "y"},
				"Principal": SliceOrString{"x", "y"},
			}
			bytes, err := json.Marshal(in)
			require.NoError(t, err)
			require.Equal(t, `{"AWS":["x","y"],"Principal":["x","y"]}`, string(bytes))
		})
		t.Run("single entity without entries", func(t *testing.T) {
			in := StringOrMap{"AWS": SliceOrString{}}
			bytes, err := json.Marshal(in)
			require.NoError(t, err)
			require.Equal(t, `{"AWS":[]}`, string(bytes))
		})
		t.Run("single entity without entries but is wildcard", func(t *testing.T) {
			in := StringOrMap{"*": SliceOrString{}}
			bytes, err := json.Marshal(in)
			require.NoError(t, err)
			require.Equal(t, `"*"`, string(bytes))
		})
		t.Run("wildcard but at least one entry", func(t *testing.T) {
			in := StringOrMap{"*": SliceOrString{"x"}}
			bytes, err := json.Marshal(in)
			require.NoError(t, err)
			require.Equal(t, `{"*":"x"}`, string(bytes))
		})
		t.Run("multiple entities but only one of them is wildcard", func(t *testing.T) {
			in := StringOrMap{
				"*":         SliceOrString{"x"},
				"Principal": SliceOrString{"x"},
			}
			bytes, err := json.Marshal(in)
			require.NoError(t, err)
			require.Equal(t, `{"*":"x","Principal":"x"}`, string(bytes))
		})
	})

	t.Run("unmarshal", func(t *testing.T) {
		t.Run("empty map", func(t *testing.T) {
			var single StringOrMap
			err := json.Unmarshal([]byte(`{}`), &single)
			require.NoError(t, err)
			require.Equal(t, StringOrMap{}, single)
		})
		t.Run("single entity with single entry", func(t *testing.T) {
			var single StringOrMap
			err := json.Unmarshal([]byte(`{"AWS":"x"}`), &single)
			require.NoError(t, err)
			require.Equal(t, StringOrMap{"AWS": SliceOrString{"x"}}, single)
		})
		t.Run("single entity with multiple entries", func(t *testing.T) {
			var single StringOrMap
			err := json.Unmarshal([]byte(`{"AWS":["x","y"]}`), &single)
			require.NoError(t, err)
			require.Equal(t, StringOrMap{"AWS": SliceOrString{"x", "y"}}, single)
		})
		t.Run("multiple entities with multiple entries", func(t *testing.T) {
			var single StringOrMap
			err := json.Unmarshal([]byte(`{"AWS":["x","y"],"Principal":["x","y"]}`), &single)
			require.NoError(t, err)
			require.Equal(t, StringOrMap{
				"AWS":       SliceOrString{"x", "y"},
				"Principal": SliceOrString{"x", "y"},
			}, single)
		})
		t.Run("single entity without entries", func(t *testing.T) {
			var single StringOrMap
			err := json.Unmarshal([]byte(`{"AWS":[]}`), &single)
			require.NoError(t, err)
			require.Equal(t, StringOrMap{"AWS": SliceOrString{}}, single)
		})
		t.Run("single entity without entries but is wildcard", func(t *testing.T) {
			var single StringOrMap
			err := json.Unmarshal([]byte(`"*"`), &single)
			require.NoError(t, err)
			require.Equal(t, StringOrMap{"*": SliceOrString{}}, single)
		})
		t.Run("wildcard but at least one entry", func(t *testing.T) {
			var single StringOrMap
			err := json.Unmarshal([]byte(`{"*":"x"}`), &single)
			require.NoError(t, err)
			require.Equal(t, StringOrMap{"*": SliceOrString{"x"}}, single)
		})
		t.Run("multiple entities but only one of them is wildcard", func(t *testing.T) {
			var single StringOrMap
			err := json.Unmarshal([]byte(`{"*":"x","Principal":"x"}`), &single)
			require.NoError(t, err)
			require.Equal(t, StringOrMap{
				"*":         SliceOrString{"x"},
				"Principal": SliceOrString{"x"},
			}, single)
		})
	})
}

func TestParsePolicyDocument(t *testing.T) {
	t.Run("parse without principals", func(t *testing.T) {
		policyDoc, err := ParsePolicyDocument(`{
			"Version": "2012-10-17",
			"Statement": [
			  {
				"Effect": "Allow",
				"Action": "rds-db:connect",
				"Resource": ["arn:aws:rds-db:us-west-1:12345:dbuser:id/*"]
			  }
			]
		  }`)
		require.NoError(t, err)
		require.Equal(t, PolicyDocument{
			Version: PolicyVersion,
			Statements: []*Statement{{
				Effect:    EffectAllow,
				Actions:   SliceOrString{"rds-db:connect"},
				Resources: SliceOrString{"arn:aws:rds-db:us-west-1:12345:dbuser:id/*"},
			}},
		}, *policyDoc)
	})
	t.Run("parse without resource", func(t *testing.T) {
		policyDoc, err := ParsePolicyDocument(`{
			"Version": "2012-10-17",
			"Statement": [
			  {
				"Effect": "Allow",
				"Action": "rds-db:connect",
				"Principal": {
					"Service": "ecs-tasks.amazonaws.com"
				}
			  }
			]
		  }`)
		require.NoError(t, err)
		require.Equal(t, PolicyDocument{
			Version: PolicyVersion,
			Statements: []*Statement{{
				Effect:  EffectAllow,
				Actions: SliceOrString{"rds-db:connect"},
				Principals: map[string]SliceOrString{
					"Service": {"ecs-tasks.amazonaws.com"},
				},
			}},
		}, *policyDoc)
	})
}

func TestMarshalPolicyDocument(t *testing.T) {
	t.Run("marshal without principal", func(t *testing.T) {
		doc := PolicyDocument{
			Version: PolicyVersion,
			Statements: []*Statement{{
				Effect:    EffectAllow,
				Actions:   SliceOrString{"rds-db:connect"},
				Resources: SliceOrString{"arn:aws:rds-db:us-west-1:12345:dbuser:id/*"},
			}},
		}

		docString, err := doc.Marshal()
		require.NoError(t, err)

		require.Equal(t, `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "rds-db:connect",
            "Resource": "arn:aws:rds-db:us-west-1:12345:dbuser:id/*"
        }
    ]
}`, docString)
	})

	t.Run("marshal without resources", func(t *testing.T) {
		doc := PolicyDocument{
			Version: PolicyVersion,
			Statements: []*Statement{{
				Effect:  EffectAllow,
				Actions: SliceOrString{"rds-db:connect"},
				Principals: map[string]SliceOrString{
					"Service": {"ecs-tasks.amazonaws.com"},
				},
			}},
		}

		docString, err := doc.Marshal()
		require.NoError(t, err)

		require.Equal(t, `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "rds-db:connect",
            "Principal": {
                "Service": "ecs-tasks.amazonaws.com"
            }
        }
    ]
}`, docString)
	})

	t.Run("marshal with condition", func(t *testing.T) {
		doc := PolicyDocument{
			Version: PolicyVersion,
			Statements: []*Statement{{
				Effect:  EffectAllow,
				Actions: SliceOrString{"sts:AssumeRoleWithWebIdentity"},
				Principals: map[string]SliceOrString{
					"Federated": {"arn:aws:iam::123456789012:oidc-provider/proxy.example.com"},
				},
				Conditions: map[string]StringOrMap{
					"StringEquals": {
						"proxy.example.com:aud": SliceOrString{"discover.teleport"},
					},
				},
			}},
		}

		docString, err := doc.Marshal()
		require.NoError(t, err)

		require.Equal(t, `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Principal": {
                "Federated": "arn:aws:iam::123456789012:oidc-provider/proxy.example.com"
            },
            "Condition": {
                "StringEquals": {
                    "proxy.example.com:aud": "discover.teleport"
                }
            }
        }
    ]
}`, docString)
	})
}

// TestIAMPolicy verifies AWS IAM policy manipulations.
func TestIAMPolicy(t *testing.T) {
	policy := NewPolicyDocument()

	// Add a new action/resource.
	updated := policy.EnsureResourceAction(EffectAllow, "action-1", "resource-1", nil)
	require.True(t, updated)
	require.Equal(t, &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-1"},
			},
		},
	}, policy)

	// Add the same action/resource.
	updated = policy.EnsureResourceAction(EffectAllow, "action-1", "resource-1", nil)
	require.False(t, updated)
	require.Equal(t, &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-1"},
			},
		},
	}, policy)

	// Add a new resource to existing action.
	updated = policy.EnsureResourceAction(EffectAllow, "action-1", "resource-2", nil)
	require.True(t, updated)
	require.Equal(t, &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-1", "resource-2"},
			},
		},
	}, policy)

	// Add another action/resource.
	updated = policy.EnsureResourceAction(EffectAllow, "action-2", "resource-3", nil)
	require.True(t, updated)
	require.Equal(t, &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-1", "resource-2"},
			},
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-2"},
				Resources: []string{"resource-3"},
			},
		},
	}, policy)

	// Delete existing resource action.
	policy.DeleteResourceAction(EffectAllow, "action-1", "resource-1", nil)
	require.Equal(t, &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-2"},
			},
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-2"},
				Resources: []string{"resource-3"},
			},
		},
	}, policy)

	// Delete last resource from first action, statement should get removed as well.
	policy.DeleteResourceAction(EffectAllow, "action-1", "resource-2", nil)
	require.Equal(t, &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-2"},
				Resources: []string{"resource-3"},
			},
		},
	}, policy)

	// Delete last resource action, policy should be empty.
	policy.DeleteResourceAction(EffectAllow, "action-2", "resource-3", nil)
	require.Equal(t, &PolicyDocument{
		Version: PolicyVersion,
	}, policy)

	// Policy with duplicate statement.
	policy = &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-1"},
			},
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-1"},
			},
		},
	}
	policy.DeleteResourceAction(EffectAllow, "action-1", "resource-1", nil)
	require.Equal(t, &PolicyDocument{
		Version: PolicyVersion,
	}, policy)

	// Policy with deny statement.
	policy = &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-1", "resource-2"},
			},
			{
				Effect:    EffectDeny,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-2"},
			},
		},
	}
	policy.DeleteResourceAction(EffectAllow, "action-1", "resource-2", nil)
	require.Equal(t, &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-1"},
			},
			{
				Effect:    EffectDeny,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-2"},
			},
		},
	}, policy)
}

func TestPolicyEnsureStatements(t *testing.T) {
	policy := NewPolicyDocument(
		&Statement{
			Effect:    EffectAllow,
			Actions:   []string{"action-1"},
			Resources: []string{"resource-1"},
		},
		&Statement{
			Effect:    EffectDeny,
			Actions:   []string{"action-1"},
			Resources: []string{"resource-2"},
		},
	)

	policy.EnsureStatements(
		// Existing/new action and existing resource.
		&Statement{
			Effect:    EffectAllow,
			Actions:   []string{"action-1", "action-2"},
			Resources: []string{"resource-1"},
		},
		// Existing action and new resource.
		&Statement{
			Effect:    EffectAllow,
			Actions:   []string{"action-1"},
			Resources: []string{"resource-3"},
		},
		// Existing action with different condition and new principals
		&Statement{
			Effect:  EffectAllow,
			Actions: []string{"action-1"},
			Principals: StringOrMap{
				"Federated": []string{"arn:aws:iam::123456789012:oidc-provider/example.com"},
			},
			Conditions: Conditions{
				"StringEquals": StringOrMap{
					"example.com:aud": []string{"discover.teleport"},
				},
			},
		},
		// New actions and new resources.
		&Statement{
			Effect:    EffectAllow,
			Actions:   []string{"action-2", "action-3", "action-4"},
			Resources: []string{"resource-4"},
		},
		// Test nil.
		nil,
		// Existing action and resource.
		&Statement{
			Effect:    EffectDeny,
			Actions:   []string{"action-1"},
			Resources: []string{"resource-2"},
		},
	)
	require.Equal(t, &PolicyDocument{
		Version: PolicyVersion,
		Statements: []*Statement{
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-1", "resource-3"},
			},
			{
				Effect:    EffectDeny,
				Actions:   []string{"action-1"},
				Resources: []string{"resource-2"},
			},
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-2"},
				Resources: []string{"resource-1", "resource-4"},
			},
			{
				Effect:  EffectAllow,
				Actions: []string{"action-1"},
				Principals: StringOrMap{
					"Federated": []string{"arn:aws:iam::123456789012:oidc-provider/example.com"},
				},
				Conditions: Conditions{
					"StringEquals": StringOrMap{
						"example.com:aud": []string{"discover.teleport"},
					},
				},
			},
			{
				Effect:    EffectAllow,
				Actions:   []string{"action-3", "action-4"},
				Resources: []string{"resource-4"},
			},
		},
	}, policy)
}

func TestGetPolicyVersions(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		tags        map[string]string
		iamMock     *iamMock
		returnError bool
	}{
		"PolicyFound": {
			iamMock: &iamMock{
				policy:         &iamtypes.Policy{},
				policyVersions: []iamtypes.PolicyVersion{{VersionId: aws.String("v1")}},
			},
		},
		"PolicyMatchLabels": {
			tags: map[string]string{"env": "prod"},
			iamMock: &iamMock{
				policy:         &iamtypes.Policy{Tags: []iamtypes.Tag{{Key: aws.String("env"), Value: aws.String("prod")}}},
				policyVersions: []iamtypes.PolicyVersion{{VersionId: aws.String("v1")}},
			},
		},
		"PolicyNotMatchingLabels": {
			tags:        map[string]string{"env": "prod"},
			returnError: true,
			iamMock: &iamMock{
				policy:         &iamtypes.Policy{},
				policyVersions: []iamtypes.PolicyVersion{{VersionId: aws.String("v1")}},
			},
		},
		"PolicyNotFound": {
			iamMock:     &iamMock{},
			returnError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// getPolicyVersions doesn't use `identity` so we can pass an empty value.
			policies := NewPolicies("", "", test.iamMock)

			versions, err := policies.getPolicyVersions(ctx, "", test.tags)
			if test.returnError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Empty(t, cmp.Diff(test.iamMock.policyVersions, versions,
				cmp.AllowUnexported(iamtypes.PolicyVersion{}),
			))
		})
	}
}

func TestUpsertPolicy(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	accountID := "123456789012"
	partitionID := "aws"

	tests := map[string]struct {
		expectedPolicyArn string
		returnError       bool
		iamMock           *iamMock
	}{
		"CreateNewPolicy": {
			expectedPolicyArn: "expected-arn",
			iamMock: &iamMock{
				policyCreated: &iamtypes.Policy{Arn: aws.String("expected-arn")},
			},
		},
		"AddPolicyVersion": {
			expectedPolicyArn: fmt.Sprintf("arn:aws:iam::%s:policy/", accountID),
			iamMock: &iamMock{
				policy: &iamtypes.Policy{Arn: aws.String("expected-arn")},
				policyVersions: []iamtypes.PolicyVersion{
					{VersionId: aws.String("v1"), IsDefaultVersion: false, CreateDate: aws.Time(now.Add(time.Second))},
				},
				policyVersionCreated: &iamtypes.PolicyVersion{},
			},
		},
		"DeleteAndAddPolicyVersion": {
			expectedPolicyArn: fmt.Sprintf("arn:aws:iam::%s:policy/", accountID),
			iamMock: &iamMock{
				policy: &iamtypes.Policy{Arn: aws.String("expected-arn")},
				policyVersions: []iamtypes.PolicyVersion{
					{VersionId: aws.String("v1"), IsDefaultVersion: false, CreateDate: aws.Time(now.Add(time.Second))},
					{VersionId: aws.String("v2"), IsDefaultVersion: false, CreateDate: aws.Time(now.Add(2 * time.Second))},
					{VersionId: aws.String("v3"), IsDefaultVersion: false, CreateDate: aws.Time(now.Add(3 * time.Second))},
					{VersionId: aws.String("v4"), IsDefaultVersion: false, CreateDate: aws.Time(now.Add(4 * time.Second))},
					{VersionId: aws.String("v5"), IsDefaultVersion: true, CreateDate: aws.Time(now.Add(5 * time.Second))},
				},
				policyVersionDeleted: true,
				policyVersionCreated: &iamtypes.PolicyVersion{},
			},
		},
		"PolicyCreateError": {
			returnError: true,
			iamMock:     &iamMock{},
		},
		"PolicyVersionCreateError": {
			returnError: true,
			iamMock: &iamMock{
				policy: &iamtypes.Policy{Arn: aws.String("expected-arn")},
				policyVersions: []iamtypes.PolicyVersion{
					{VersionId: aws.String("v1"), IsDefaultVersion: false, CreateDate: aws.Time(now.Add(time.Second))},
				},
			},
		},
		"PolicyVersionDeleteError": {
			returnError: true,
			iamMock: &iamMock{
				policy: &iamtypes.Policy{Arn: aws.String("expected-arn")},
				policyVersions: []iamtypes.PolicyVersion{
					{VersionId: aws.String("v1"), IsDefaultVersion: false, CreateDate: aws.Time(now.Add(time.Second))},
					{VersionId: aws.String("v2"), IsDefaultVersion: false, CreateDate: aws.Time(now.Add(2 * time.Second))},
					{VersionId: aws.String("v3"), IsDefaultVersion: false, CreateDate: aws.Time(now.Add(3 * time.Second))},
					{VersionId: aws.String("v4"), IsDefaultVersion: false, CreateDate: aws.Time(now.Add(4 * time.Second))},
					{VersionId: aws.String("v5"), IsDefaultVersion: true, CreateDate: aws.Time(now.Add(5 * time.Second))},
				},
				policyVersionCreated: &iamtypes.PolicyVersion{},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			policies := NewPolicies(partitionID, accountID, test.iamMock)

			arn, err := policies.Upsert(ctx, &Policy{})
			if test.returnError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.expectedPolicyArn, arn)
		})
	}
}

func TestAttachPolicy(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		returnError bool
		identity    Identity
		iamMock     *iamMock
	}{
		"AttachToUser": {
			identity: userIdentity(),
			iamMock: &iamMock{
				attachUserPolicy: true,
			},
		},
		"AttachToRole": {
			identity: roleIdentity(),
			iamMock: &iamMock{
				attachRolePolicy: true,
			},
		},
		"UnsupportedIdentity": {
			returnError: true,
			identity:    unknownIdentity(),
			iamMock: &iamMock{
				// "enable" both attach to ensure the error doesn't come from
				// the IAM client.
				attachUserPolicy: true,
				attachRolePolicy: true,
			},
		},
		"AttachError": {
			returnError: true,
			identity:    userIdentity(),
			iamMock:     &iamMock{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			policies := NewPolicies("", "", test.iamMock)

			err := policies.Attach(ctx, "", test.identity)
			if test.returnError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

// userIdentity helper function to generate an user `Identity` .
func userIdentity() Identity {
	return &User{
		identityBase: identityBase{
			arn: arn.ARN{AccountID: "1234567", Resource: "user/example-user"},
		},
	}
}

// roleIdentity helper function to generate a role `Identity` .
func roleIdentity() Identity {
	return &Role{
		identityBase: identityBase{
			arn: arn.ARN{AccountID: "1234567", Resource: "role/example-role"},
		},
	}
}

// roleIdentity helper function to generate a role `Identity` .
func unknownIdentity() Identity {
	return &Unknown{}
}

type iamMock struct {
	policy               *iamtypes.Policy
	policyVersions       []iamtypes.PolicyVersion
	policyCreated        *iamtypes.Policy
	policyVersionCreated *iamtypes.PolicyVersion
	policyVersionDeleted bool

	attachUserPolicy bool
	attachRolePolicy bool
}

func (m *iamMock) GetPolicy(ctx context.Context, params *iam.GetPolicyInput, optFns ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {

	if m.policy == nil {
		return nil, &iamtypes.NoSuchEntityException{
			Message: aws.String("not found"),
		}
	}

	return &iam.GetPolicyOutput{Policy: m.policy}, nil
}

func (m *iamMock) ListPolicyVersions(ctx context.Context, params *iam.ListPolicyVersionsInput, optFns ...func(*iam.Options)) (*iam.ListPolicyVersionsOutput, error) {
	if len(m.policyVersions) == 0 {
		return nil, &iamtypes.NoSuchEntityException{
			Message: aws.String("not found"),
		}
	}

	return &iam.ListPolicyVersionsOutput{Versions: m.policyVersions}, nil
}

func (m *iamMock) CreatePolicy(ctx context.Context, params *iam.CreatePolicyInput, optFns ...func(*iam.Options)) (*iam.CreatePolicyOutput, error) {
	if m.policyCreated == nil {
		return nil, trace.NotImplemented("CreatePolicy not implemented")
	}

	return &iam.CreatePolicyOutput{Policy: m.policyCreated}, nil
}

func (m *iamMock) CreatePolicyVersion(ctx context.Context, params *iam.CreatePolicyVersionInput, optFns ...func(*iam.Options)) (*iam.CreatePolicyVersionOutput, error) {
	if m.policyVersionCreated == nil {
		return nil, trace.NotImplemented("CreatePolicyVersion not implemented")
	}

	return &iam.CreatePolicyVersionOutput{PolicyVersion: m.policyVersionCreated}, nil
}

func (m *iamMock) DeletePolicyVersion(ctx context.Context, params *iam.DeletePolicyVersionInput, optFns ...func(*iam.Options)) (*iam.DeletePolicyVersionOutput, error) {
	if !m.policyVersionDeleted {
		return nil, trace.NotImplemented("DeletePolicyVersion not implemented")
	}

	return &iam.DeletePolicyVersionOutput{}, nil
}

func (m *iamMock) AttachUserPolicy(ctx context.Context, params *iam.AttachUserPolicyInput, optFns ...func(*iam.Options)) (*iam.AttachUserPolicyOutput, error) {
	if !m.attachUserPolicy {
		return nil, trace.NotImplemented("AttachUserPolicy not implemented")
	}

	return &iam.AttachUserPolicyOutput{}, nil
}

func (m *iamMock) AttachRolePolicy(ctx context.Context, params *iam.AttachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.AttachRolePolicyOutput, error) {
	if !m.attachRolePolicy {
		return nil, trace.NotImplemented("AttachRolePolicy not implemented")
	}

	return &iam.AttachRolePolicyOutput{}, nil
}

func TestEqualStatement(t *testing.T) {
	for _, tt := range []struct {
		name       string
		statementA *Statement
		statementB *Statement
		expected   bool
	}{
		{
			name:       "empty statement",
			statementA: &Statement{},
			statementB: &Statement{},
			expected:   true,
		},
		{
			name: "statement id is ignored",
			statementA: &Statement{
				StatementID: "x",
			},
			statementB: &Statement{
				StatementID: "y",
			},
			expected: true,
		},
		{
			name: "different number of actions",
			statementA: &Statement{
				Actions: SliceOrString{"x", "y"},
			},
			statementB: &Statement{
				Actions: SliceOrString{"y"},
			},
			expected: false,
		},
		{
			name: "different actions",
			statementA: &Statement{
				Actions: SliceOrString{"x"},
			},
			statementB: &Statement{
				Actions: SliceOrString{"y"},
			},
			expected: false,
		},
		{
			name: "different number of principals",
			statementA: &Statement{
				Principals: StringOrMap{"AWS": []string{"123456789012", "123456789013"}},
			},
			statementB: &Statement{
				Principals: StringOrMap{
					"AWS":            []string{"123456789012", "123456789014"},
					"OtherPrincipal": []string{"x"},
				},
			},
			expected: false,
		},
		{
			name: "different principals",
			statementA: &Statement{
				Principals: StringOrMap{"AWS": []string{"*"}},
			},
			statementB: &Statement{
				Principals: StringOrMap{"*": []string{}},
			},
			expected: false,
		},
		{
			name: "different number of conditions",
			statementA: &Statement{
				Conditions: map[string]StringOrMap{
					"NumericLessThanEquals": {"aws:MultiFactorAuthAge": []string{"3600"}},
					"StringLike":            {"s3:prefix": []string{"janedoe/*"}},
				},
			},
			statementB: &Statement{
				Conditions: map[string]StringOrMap{
					"NumericLessThanEquals": {"aws:MultiFactorAuthAge": []string{"3601"}},
				},
			},
			expected: false,
		},
		{
			name: "different conditions",
			statementA: &Statement{
				Conditions: map[string]StringOrMap{
					"NumericLessThanEquals": {"aws:MultiFactorAuthAge": []string{"3600"}},
				},
			},
			statementB: &Statement{
				Conditions: map[string]StringOrMap{
					"NumericLessThanEquals": {"aws:MultiFactorAuthAge": []string{"3601"}},
				},
			},
			expected: false,
		},
		{
			name: "different condition values",
			statementA: &Statement{
				Conditions: map[string]StringOrMap{
					"NumericLessThanEquals": {"aws:MultiFactorAuthAge": []string{"3600", "3601"}},
				},
			},
			statementB: &Statement{
				Conditions: map[string]StringOrMap{
					"NumericLessThanEquals": {"aws:MultiFactorAuthAge": []string{"3600"}},
				},
			},
			expected: false,
		},
		{
			name: "different resource values",
			statementA: &Statement{
				Resources: SliceOrString{"arn:aws:s3:::bucket-2/prefix-2/*"},
			},
			statementB: &Statement{
				Resources: SliceOrString{"arn:aws:s3:::bucket-1/*"},
			},
			expected: false,
		},
		{
			name: "equal statements",
			statementA: &Statement{
				Effect: EffectAllow,
				Principals: StringOrMap{
					wildcard: []string{},
				},
				Actions:   []string{"s3:GetObject"},
				Resources: []string{"arn:aws:s3:::my-bucket/my-prefix/*"},
				Conditions: map[string]StringOrMap{
					"StringLike": {"s3:prefix": []string{"my-prefix/*"}},
				},
			},
			statementB: &Statement{
				Effect: EffectAllow,
				Principals: StringOrMap{
					wildcard: []string{},
				},
				Actions:   []string{"s3:GetObject"},
				Resources: []string{"arn:aws:s3:::my-bucket/my-prefix/*"},
				Conditions: map[string]StringOrMap{
					"StringLike": {"s3:prefix": []string{"my-prefix/*"}},
				},
			},
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.statementA.EqualStatement(tt.statementB))
		})
	}
}
