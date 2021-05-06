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

package ruleset

import (
	"sort"

	. "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/vulcand/predicate"
)

// DefaultImplicitRules provides access to the default set of implicit rules
// assigned to all roles.
var DefaultImplicitRules = []Rule{
	NewRule(KindNode, RO()),
	NewRule(KindProxy, RO()),
	NewRule(KindAuthServer, RO()),
	NewRule(KindReverseTunnel, RO()),
	NewRule(KindCertAuthority, ReadNoSecrets()),
	NewRule(KindClusterAuthPreference, RO()),
	NewRule(KindClusterName, RO()),
	NewRule(KindSSHSession, RO()),
	NewRule(KindAppServer, RO()),
	NewRule(KindRemoteCluster, RO()),
	NewRule(KindKubeService, RO()),
	NewRule(KindDatabaseServer, RO()),
}

// RuleSet maps resource to a set of rules defined for it
type RuleSet map[string][]Rule

// MakeRuleSet creates a new rule set from a list
func MakeRuleSet(rules []Rule) RuleSet {
	set := make(RuleSet)
	for _, rule := range rules {
		for _, resource := range rule.Resources {
			set[resource] = append(set[resource], rule)
		}
	}
	for resource := range set {
		rules := set[resource]
		// sort rules by most specific rule, the rule that has actions
		// is more specific than the one that has no actions
		sort.Slice(rules, func(i, j int) bool {
			return CompareRuleScore(&rules[i], &rules[j])
		})
		set[resource] = rules
	}
	return set
}

// Match tests if the resource name and verb are in a given list of rules.
// More specific rules will be matched first. See Rule.IsMoreSpecificThan
// for exact specs on whether the rule is more or less specific.
//
// Specifying order solves the problem on having multiple rules, e.g. one wildcard
// rule can override more specific rules with 'where' sections that can have
// 'actions' lists with side effects that will not be triggered otherwise.
//
func (set RuleSet) Match(whereParser predicate.Parser, actionsParser predicate.Parser, resource string, verb string) (bool, error) {
	// empty set matches nothing
	if len(set) == 0 {
		return false, nil
	}

	// check for matching resource by name
	// the most specific rule should win
	rules := set[resource]
	for _, rule := range rules {
		match, err := matchesWhere(&rule, whereParser)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if match && (rule.HasVerb(Wildcard) || rule.HasVerb(verb)) {
			if err := processActions(&rule, actionsParser); err != nil {
				return true, trace.Wrap(err)
			}
			return true, nil
		}
	}

	// check for wildcard resource matcher
	for _, rule := range set[Wildcard] {
		match, err := matchesWhere(&rule, whereParser)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if match && (rule.HasVerb(Wildcard) || rule.HasVerb(verb)) {
			if err := processActions(&rule, actionsParser); err != nil {
				return true, trace.Wrap(err)
			}
			return true, nil
		}
	}

	return false, nil
}

// matchesWhere returns true if Where rule matches.
// Empty Where block always matches.
func matchesWhere(r *Rule, parser predicate.Parser) (bool, error) {
	if r.Where == "" {
		return true, nil
	}
	ifn, err := parser.Parse(r.Where)
	if err != nil {
		return false, trace.Wrap(err)
	}
	fn, ok := ifn.(predicate.BoolPredicate)
	if !ok {
		return false, trace.BadParameter("invalid predicate type for where expression: %v", r.Where)
	}
	return fn(), nil
}

// processActions processes actions specified for this rule
func processActions(r *Rule, parser predicate.Parser) error {
	for _, action := range r.Actions {
		ifn, err := parser.Parse(action)
		if err != nil {
			return trace.Wrap(err)
		}
		fn, ok := ifn.(predicate.BoolPredicate)
		if !ok {
			return trace.BadParameter("invalid predicate type for action expression: %v", action)
		}
		fn()
	}
	return nil
}

// Slice returns slice from a set
func (set RuleSet) Slice() []Rule {
	var out []Rule
	for _, rules := range set {
		out = append(out, rules...)
	}
	return out
}

// ruleScore is a sorting score of the rule, the larger the score, the more
// specific the rule is
func ruleScore(r *Rule) int {
	score := 0
	// wildcard rules are less specific
	if utils.SliceContainsStr(r.Resources, Wildcard) {
		score -= 4
	} else if len(r.Resources) == 1 {
		// rules that match specific resource are more specific than
		// fields that match several resources
		score += 2
	}
	// rules that have wildcard verbs are less specific
	if utils.SliceContainsStr(r.Verbs, Wildcard) {
		score -= 2
	}
	// rules that supply 'where' or 'actions' are more specific
	// having 'where' or 'actions' is more important than
	// whether the rules are wildcard or not, so here we have +8 vs
	// -4 and -2 score penalty for wildcards in resources and verbs
	if len(r.Where) > 0 {
		score += 8
	}
	// rules featuring actions are more specific
	if len(r.Actions) > 0 {
		score += 8
	}
	return score
}

// CompareRuleScore returns true if the first rule is more specific than the other.
//
// * nRule matching wildcard resource is less specific
// than same rule matching specific resource.
// * Rule that has wildcard verbs is less specific
// than the same rules matching specific verb.
// * Rule that has where section is more specific
// than the same rule without where section.
// * Rule that has actions list is more specific than
// rule without actions list.
func CompareRuleScore(r *Rule, o *Rule) bool {
	return ruleScore(r) > ruleScore(o)
}

// RW is a shortcut that returns all verbs.
func RW() []string {
	return []string{VerbList, VerbCreate, VerbRead, VerbUpdate, VerbDelete}
}

// RO is a shortcut that returns read only verbs that provide access to secrets.
func RO() []string {
	return []string{VerbList, VerbRead}
}

// ReadNoSecrets is a shortcut that returns read only verbs that do not
// provide access to secrets.
func ReadNoSecrets() []string {
	return []string{VerbList, VerbReadNoSecrets}
}
