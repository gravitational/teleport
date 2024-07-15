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

package services

import (
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
	"github.com/gravitational/teleport/lib/utils/typical"
)

type labelExpression typical.Expression[labelExpressionEnv, bool]

type labelExpressionEnv struct {
	resourceLabelGetter LabelGetter
	userTraits          map[string][]string
}

var labelExpressionParser = mustNewLabelExpressionParser()

func parseLabelExpression(expr string) (labelExpression, error) {
	parsedExpr, err := labelExpressionParser.Parse(expr)
	if err != nil {
		return nil, trace.Wrap(err, "parsing label expression")
	}
	return parsedExpr, nil

}

func mustNewLabelExpressionParser() *typical.CachedParser[labelExpressionEnv, bool] {
	parser, err := newLabelExpressionParser()
	if err != nil {
		panic(trace.Wrap(err, "failed to create label expression parser (this is a bug)"))
	}
	return parser
}

func newLabelExpressionParser() (*typical.CachedParser[labelExpressionEnv, bool], error) {
	parser, err := typical.NewCachedParser[labelExpressionEnv, bool](typical.ParserSpec[labelExpressionEnv]{
		Variables: map[string]typical.Variable{
			"user.spec.traits": typical.DynamicVariable(
				func(env labelExpressionEnv) (map[string][]string, error) {
					return env.userTraits, nil
				}),
			"labels": typical.DynamicMapFunction(
				func(env labelExpressionEnv, key string) (string, error) {
					label, _ := env.resourceLabelGetter.GetLabel(key)
					return label, nil
				}),
		},
		Functions: map[string]typical.Function{
			"labels_matching": typical.UnaryFunctionWithEnv(labelsMatching),
			"contains": typical.BinaryFunction[labelExpressionEnv](
				func(list []string, item string) (bool, error) {
					return slices.Contains(list, item), nil
				}),
			"contains_any": typical.BinaryFunction[labelExpressionEnv](containsAny),
			"contains_all": typical.BinaryFunction[labelExpressionEnv](containsAll),
			"regexp.match": typical.BinaryFunction[labelExpressionEnv](
				func(list []string, re string) (bool, error) {
					match, err := utils.RegexMatchesAny(list, re)
					if err != nil {
						return false, trace.Wrap(err, "invalid regular expression %q", re)
					}
					return match, nil
				}),
			// Use regexp.replace and email.local from lib/utils/parse to get behavior identical
			// to role templates.
			"regexp.replace": typical.TernaryFunction[labelExpressionEnv](parse.RegexpReplace),
			"email.local":    typical.UnaryFunction[labelExpressionEnv](parse.EmailLocal),
			"strings.upper": typical.UnaryFunction[labelExpressionEnv](
				func(list []string) ([]string, error) {
					out := make([]string, len(list))
					for i, s := range list {
						out[i] = strings.ToUpper(s)
					}
					return out, nil
				}),
			"strings.lower": typical.UnaryFunction[labelExpressionEnv](
				func(list []string) ([]string, error) {
					out := make([]string, len(list))
					for i, s := range list {
						out[i] = strings.ToLower(s)
					}
					return out, nil
				}),
		},
	})
	return parser, trace.Wrap(err)
}

// labelsMatching returns the aggregate of all label values for all keys that
// match keyExpr. It supports globs or full regular expressions and must find
// a complete match for the key.
func labelsMatching(env labelExpressionEnv, keyExpr string) ([]string, error) {
	allLabels := env.resourceLabelGetter.GetAllLabels()
	var matchingLabelValues []string
	for key, value := range allLabels {
		match, err := utils.MatchString(key, keyExpr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if match {
			matchingLabelValues = append(matchingLabelValues, value)
		}
	}
	return matchingLabelValues, nil
}

// containsAny returns true if [list] contains any element of [items].
func containsAny(list []string, items []string) (bool, error) {
	s := set(list)
	for _, item := range items {
		if _, ok := s[item]; ok {
			return true, nil
		}
	}
	return false, nil
}

// containsAny returns true if [list] contains every element of [items]. If
// [items] is empty, it returns false, to avoid matching resources that
// otherwise appear unrelated to the expression.
func containsAll(list []string, items []string) (bool, error) {
	if len(items) == 0 {
		return false, nil
	}
	s := set(list)
	for _, item := range items {
		if _, ok := s[item]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func set(list []string) map[string]struct{} {
	m := make(map[string]struct{}, len(list))
	for _, l := range list {
		m[l] = struct{}{}
	}
	return m
}
