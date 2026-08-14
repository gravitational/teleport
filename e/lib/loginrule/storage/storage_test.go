package storage_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/e/lib/loginrule/storage"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

type testPack struct {
	clock clocki.FakeClock
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

	s := storage.New(sanitized)

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
			rule: loginrulepb.LoginRule_builder{
				Metadata: &types.Metadata{
					Name: "expression_rule",
				},
				Version:          "v1",
				TraitsExpression: "external",
			}.Build(),
		},
		{
			desc: "map",
			rule: loginrulepb.LoginRule_builder{
				Metadata: &types.Metadata{
					Name: "map_rule",
				},
				Version: "v1",
				TraitsMap: map[string]*wrappers.StringValues{
					"groups": &wrappers.StringValues{
						Values: []string{"external.groups"},
					},
				},
			}.Build(),
		},
		{
			desc: "duplicate",
			rule: loginrulepb.LoginRule_builder{
				Metadata: &types.Metadata{
					Name: "map_rule",
				},
				Version: "v1",
				TraitsMap: map[string]*wrappers.StringValues{
					"groups": &wrappers.StringValues{
						Values: []string{"external.groups"},
					},
				},
			}.Build(),
			errorContains: `login rule "map_rule" already exists`,
			noUpsertError: true,
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
				require.Empty(t, cmp.Diff(tc.rule, outRule, protocmp.Transform()), "returned rule from CreateLoginRule does not match expected")
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
				require.Empty(t, cmp.Diff(tc.rule, outRule, protocmp.Transform()), "returned rule from UpsertLoginRule does not match expected")
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
		"expression_rule": loginrulepb.LoginRule_builder{
			Metadata: &types.Metadata{
				Name: "expression_rule",
			},
			Priority:         0,
			Version:          "v1",
			TraitsExpression: "external",
		}.Build(),
		"map_rule": loginrulepb.LoginRule_builder{
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
		}.Build(),
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
			Key:   backend.NewKey("login_rules", name),
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
		{
			name:          "nonexistant",
			errorContains: `login rule "nonexistant" is not found`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule, err := p.s.GetLoginRule(ctx, tc.name)
			if tc.errorContains != "" {
				require.ErrorContains(t, err, tc.errorContains, "unexpected error from GetLoginRule")
				return
			}
			require.NoError(t, err, "unexpected error from GetLoginRule")
			require.Empty(t, cmp.Diff(seededRules[tc.name], rule, protocmp.Transform()))
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

			var seededRules []*loginrulepb.LoginRule
			for i := range tc.totalRules {
				ruleName := fmt.Sprintf("rule%d", i)
				rule := loginrulepb.LoginRule_builder{
					Metadata: &types.Metadata{
						Name: ruleName,
					},
					Version:          types.V1,
					Priority:         int32(i),
					TraitsExpression: "external",
				}.Build()
				outRule, err := p.s.CreateLoginRule(ctx, rule)
				require.NoError(t, err)
				seededRules = append(seededRules, outRule)
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
					err := p.s.DeleteLoginRule(ctx, lastRule.GetMetadata().Name)
					require.NoError(t, err)
				}
			}

			if !tc.deleteLastRuleBetweenCalls {
				// Expect an optimal number of pages to be required.
				optimalPageCount := tc.totalRules/tc.pageSize + 1
				require.Equal(t, optimalPageCount, pageCount, "expected to fetch the optimal number of pages (%d)", optimalPageCount)
			}

			if len(allRules) == 0 {
				return
			}
			sort.SliceStable(allRules, func(i, j int) bool {
				return allRules[i].GetMetadata().Name < allRules[j].GetMetadata().Name
			})
			sort.SliceStable(seededRules, func(i, j int) bool {
				return seededRules[i].GetMetadata().Name < seededRules[j].GetMetadata().Name
			})
			require.Empty(t, cmp.Diff(seededRules, allRules, protocmp.Transform()), "listed login rules do not match the expected")
		})
	}
}

func TestDeleteLoginRule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	seededRules := make(map[string]*loginrulepb.LoginRule)
	for i := range 3 {
		ruleName := fmt.Sprintf("rule%d", i)
		rule := loginrulepb.LoginRule_builder{
			Metadata: &types.Metadata{
				Name: ruleName,
			},
			Version:          types.V1,
			Priority:         int32(i),
			TraitsExpression: "external",
		}.Build()
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
			require.ErrorContains(t, err, fmt.Sprintf("login rule %q is not found", ruleName))
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

		require.Empty(t, cmp.Diff(seededRules[ruleName], rule, protocmp.Transform()))
	}
}

// TestLoginRuleRevisions asserts that the metadata.revision field of the login rule changes
// after the rule is updated.
func TestLoginRuleIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	ruleSpec := loginrulepb.LoginRule_builder{
		Metadata: &types.Metadata{
			Name: "rule",
		},
		Version:          types.V1,
		Priority:         1,
		TraitsExpression: "external",
	}.Build()

	_, err := p.s.CreateLoginRule(ctx, ruleSpec)
	require.NoError(t, err)

	ruleBefore, err := p.s.GetLoginRule(ctx, ruleSpec.GetMetadata().Name)
	require.NoError(t, err)

	ruleSpec.SetPriority(2)

	_, err = p.s.UpsertLoginRule(ctx, ruleSpec)
	require.NoError(t, err)

	ruleAfter, err := p.s.GetLoginRule(ctx, ruleSpec.GetMetadata().Name)
	require.NoError(t, err)

	require.NotEqual(t, ruleBefore.GetMetadata().Revision, ruleAfter.GetMetadata().Revision, "expected updated revision not to match original revision")

	rulesAfter, _, err := p.s.ListLoginRules(ctx, 0 /* pageSize */, "" /* pageToken */)
	require.NoError(t, err)
	require.Len(t, rulesAfter, 1)
	ruleAfter = rulesAfter[0]

	require.NotEqual(t, ruleBefore.GetMetadata().Revision, ruleAfter.GetMetadata().Revision, "expected updated revision not to match original revision")
}
