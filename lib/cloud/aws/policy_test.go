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

package aws

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIAMPolicy verifies AWS IAM policy manipulations.
func TestIAMPolicy(t *testing.T) {
	policy := NewPolicyDocument()

	// Add a new action/resource.
	alreadyExisted := policy.Ensure(EffectAllow, "action-1", "resource-1")
	require.False(t, alreadyExisted)
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
	alreadyExisted = policy.Ensure(EffectAllow, "action-1", "resource-1")
	require.True(t, alreadyExisted)
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
	alreadyExisted = policy.Ensure(EffectAllow, "action-1", "resource-2")
	require.False(t, alreadyExisted)
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
	alreadyExisted = policy.Ensure(EffectAllow, "action-2", "resource-3")
	require.False(t, alreadyExisted)
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
	policy.Delete(EffectAllow, "action-1", "resource-1")
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
	policy.Delete(EffectAllow, "action-1", "resource-2")
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
	policy.Delete(EffectAllow, "action-2", "resource-3")
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
	policy.Delete(EffectAllow, "action-1", "resource-1")
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
	policy.Delete(EffectAllow, "action-1", "resource-2")
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
