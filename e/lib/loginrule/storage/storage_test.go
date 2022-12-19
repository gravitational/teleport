package storage_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

type testPack struct {
	clock clockwork.FakeClock
	mem   *memory.Memory
	s     *storage.S
}

func newTestPack(t *testing.T) *testPack {
	t.Helper()

	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	sanitized := backend.NewSanitizer(mem)

	s := storage.New(func() backend.Backend { return sanitized })

	return &testPack{
		clock: clock,
		mem:   mem,
		s:     s,
	}
}

func TestCreateAndUpsertLoginRule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	testCases := []struct {
		desc          string
		rule          *loginrulepb.LoginRule
		errorContains string
		noUpsertError bool
	}{
		{
			desc: "expression",
			rule: &loginrulepb.LoginRule{
				Metadata: &types.Metadata{
					Name: "expression_rule",
				},
				Version:          "v1",
				TraitsExpression: "external",
			},
		},
		{
			desc: "map",
			rule: &loginrulepb.LoginRule{
				Metadata: &types.Metadata{
					Name: "map_rule",
				},
				Version: "v1",
				TraitsMap: map[string]*wrappers.StringValues{
					"groups": &wrappers.StringValues{
						Values: []string{"external.groups"},
					},
				},
			},
		},
		{
			desc: "duplicate",
			rule: &loginrulepb.LoginRule{
				Metadata: &types.Metadata{
					Name: "map_rule",
				},
				Version: "v1",
				TraitsMap: map[string]*wrappers.StringValues{
					"groups": &wrappers.StringValues{
						Values: []string{"external.groups"},
					},
				},
			},
			errorContains: "already exists",
			noUpsertError: true,
		},
		{
			desc: "no metadata",
			rule: &loginrulepb.LoginRule{
				Version: "v1",
				TraitsMap: map[string]*wrappers.StringValues{
					"groups": &wrappers.StringValues{
						Values: []string{"external.groups"},
					},
				},
			},
			errorContains: "must contain metadata",
		},
		{
			desc: "no name",
			rule: &loginrulepb.LoginRule{
				Metadata: &types.Metadata{
					Name: "",
				},
				Version: "v1",
				TraitsMap: map[string]*wrappers.StringValues{
					"groups": &wrappers.StringValues{
						Values: []string{"external.groups"},
					},
				},
			},
			errorContains: "must have non-empty metadata.name",
		},
		{
			desc: "no expressions",
			rule: &loginrulepb.LoginRule{
				Metadata: &types.Metadata{
					Name: "expressionless_rule",
				},
				Version: "v1",
			},
			errorContains: "both traits_map and traits_expression are empty",
		},
		{
			desc: "too many expressions",
			rule: &loginrulepb.LoginRule{
				Metadata: &types.Metadata{
					Name: "expressionless_rule",
				},
				Version:          "v1",
				TraitsExpression: "external",
				TraitsMap: map[string]*wrappers.StringValues{
					"groups": &wrappers.StringValues{
						Values: []string{"external.groups"},
					},
				},
			},
			errorContains: "both traits_map and traits_expression are non-empty",
		},
	}

	t.Run("create", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				outRule, err := p.s.CreateLoginRule(ctx, tc.rule)
				if tc.errorContains != "" {
					require.ErrorContains(t, err, tc.errorContains, "error from CreateLoginRule does not match expected")
					return
				}
				require.NoError(t, err, "unexpected error from CreateLoginRule")
				require.Equal(t, tc.rule.String(), outRule.String(), "returned rule from CreateLoginRule does not match expected")
			})
		}
	})

	t.Run("upsert", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				outRule, err := p.s.UpsertLoginRule(ctx, tc.rule)
				if tc.errorContains != "" && !tc.noUpsertError {
					require.ErrorContains(t, err, tc.errorContains, "error from UpsertLoginRule does not match expected")
					return
				}
				require.NoError(t, err, "unexpected error from UpsertLoginRule")
				require.Equal(t, tc.rule.String(), outRule.String(), "returned rule from UpsertLoginRule does not match expected")
			})
		}
	})
}

func TestGetLoginRule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	expiry := p.clock.Now().Add(time.Minute)

	seededRules := map[string]*loginrulepb.LoginRule{
		"expression_rule": &loginrulepb.LoginRule{
			Metadata: &types.Metadata{
				Name: "expression_rule",
			},
			Priority:         0,
			Version:          "v1",
			TraitsExpression: "external",
		},
		"map_rule": &loginrulepb.LoginRule{
			Metadata: &types.Metadata{
				Name:    "map_rule",
				Expires: &expiry,
			},
			Priority: 1,
			Version:  "v1",
			TraitsMap: map[string]*wrappers.StringValues{
				"groups": &wrappers.StringValues{
					Values: []string{"external.groups"},
				},
			},
		},
	}
	for _, rule := range seededRules {
		_, err := p.s.CreateLoginRule(ctx, rule)
		require.NoError(t, err)
	}

	// Inject some corrupted data just to make sure nothing panics.
	corruptedRules := map[string]string{
		"bad_json":    `{ "metadata": {"name": "bad_json"}, "traits_map": {}`, // no closing brace
		"no_metadata": `{ "priority": 1, "version": "v1", "traits_expression": "external" }`,
	}
	for name, corruptedRule := range corruptedRules {
		_, err := p.mem.Create(ctx, backend.Item{
			Key:   backend.Key("login_rules", name),
			Value: []byte(corruptedRule),
		})
		require.NoError(t, err)
	}

	for _, tc := range []struct {
		name          string
		errorContains string
	}{
		{
			name: "expression_rule",
		},
		{
			name: "map_rule",
		},
		{
			name:          "bad_json",
			errorContains: "error unmarshalling login rule from storage",
		},
		{
			name:          "no_metadata",
			errorContains: "unable to unmarshal login rule metadata from storage",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule, err := p.s.GetLoginRule(ctx, tc.name)
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains, "unexpected error from GetLoginRule")
				return
			}
			require.NoError(t, err, "unexpected error from GetLoginRule")
			require.Equal(t, seededRules[tc.name].String(), rule.String())
		})
	}
}

func TestListLoginRules(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	for _, tc := range []struct {
		desc                       string
		totalRules                 int
		pageSize                   int
		deleteLastRuleBetweenCalls bool
	}{
		{
			desc:       "no rules",
			totalRules: 0,
			pageSize:   100,
		},
		{
			desc:       "single rule",
			totalRules: 1,
			pageSize:   100,
		},
		{
			desc:       "page size minus one",
			totalRules: 9,
			pageSize:   10,
		},
		{
			desc:       "page size",
			totalRules: 10,
			pageSize:   10,
		},
		{
			desc:       "page size plus one",
			totalRules: 11,
			pageSize:   10,
		},
		{
			desc:       "many pages",
			totalRules: 100,
			pageSize:   10,
		},
		{
			// The name of the last rule is used as the pageToken, test what
			// happens if that rule is deleted. It will be deleted after it is
			// listed, so all seeded rules should still be listed.
			desc:                       "delete last rule",
			totalRules:                 20,
			pageSize:                   5,
			deleteLastRuleBetweenCalls: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			p := newTestPack(t)

			var seededRules []string
			for i := 0; i < tc.totalRules; i++ {
				ruleName := fmt.Sprintf("rule%d", i)
				rule := &loginrulepb.LoginRule{
					Metadata: &types.Metadata{
						Name: ruleName,
					},
					Version:          types.V1,
					Priority:         int32(i),
					TraitsExpression: "external",
				}
				outRule, err := p.s.CreateLoginRule(ctx, rule)
				require.NoError(t, err)
				seededRules = append(seededRules, outRule.String())
			}

			pageCount := 1
			allRules, nextPageToken, err := p.s.ListLoginRules(ctx, tc.pageSize, "")
			require.NoError(t, err, "unexpected error from ListLoginRules")
			for nextPageToken != "" {
				var rules []*loginrulepb.LoginRule
				rules, nextPageToken, err = p.s.ListLoginRules(ctx, tc.pageSize, nextPageToken)
				require.NoError(t, err, "unexpected error from ListLoginRules")
				allRules = append(allRules, rules...)
				pageCount++

				if tc.deleteLastRuleBetweenCalls && len(rules) > 0 {
					lastRule := rules[len(rules)-1]
					err := p.s.DeleteLoginRule(ctx, lastRule.Metadata.Name)
					require.NoError(t, err)
				}
			}

			if !tc.deleteLastRuleBetweenCalls {
				// Expect an optimal number of pages to be required.
				optimalPageCount := tc.totalRules/tc.pageSize + 1
				require.Equal(t, optimalPageCount, pageCount, "expected to fetch the optimal number of pages (%d)", optimalPageCount)
			}

			var allRulesStrings []string
			for _, rule := range allRules {
				allRulesStrings = append(allRulesStrings, rule.String())
			}

			require.ElementsMatch(t, seededRules, allRulesStrings, "listed login rules do not match the expected")
		})
	}
}

func TestDeleteLoginRule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	seededRules := make(map[string]*loginrulepb.LoginRule)
	for i := 0; i < 3; i++ {
		ruleName := fmt.Sprintf("rule%d", i)
		rule := &loginrulepb.LoginRule{
			Metadata: &types.Metadata{
				Name: ruleName,
			},
			Version:          types.V1,
			Priority:         int32(i),
			TraitsExpression: "external",
		}
		_, err := p.s.CreateLoginRule(ctx, rule)
		require.NoError(t, err)
		seededRules[ruleName] = rule
	}

	rulesToDelete := map[string]bool{
		"rule1":  true,
		"rule99": false,
	}

	// Delete an existing and non-existing rule.
	for ruleName, success := range rulesToDelete {
		err := p.s.DeleteLoginRule(ctx, ruleName)
		if success {
			require.NoError(t, err, "unexpected error from DeleteLoginRule")
		} else {
			require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
		}
	}

	// Make sure non-deleted rules are still there, and deleted rules are gone.
	for ruleName := range seededRules {
		rule, err := p.s.GetLoginRule(ctx, ruleName)
		if _, deleted := rulesToDelete[ruleName]; deleted {
			require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
			return
		}
		require.NoError(t, err)
		require.Equal(t, seededRules[ruleName].String(), rule.String())
	}
}
