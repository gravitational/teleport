// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services

import (
	"os"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/parse"
	"github.com/gravitational/teleport/lib/utils/typical"
)

type labelExpression typical.Expression[labelExpressionEnv, bool]

type labelExpressionEnv struct {
	resourceLabelGetter LabelGetter
	userTraits          map[string][]string
}

func parseLabelExpression(expr string) (labelExpression, error) {
	if parsedExpr, ok := labelExpressionCache.Get(expr); ok {
		return parsedExpr, nil
	}
	parsedExpr, err := labelExpressionParser.Parse(expr)
	if err != nil {
		return nil, trace.Wrap(err, "parsing label expression")
	}
	if evicted := labelExpressionCache.Add(expr, parsedExpr); evicted {
		log.Info("Evicting entry from label expression cache")
	}
	return parsedExpr, nil

}

var (
	labelExpressionCache  = mustNewLabelExpressionCache()
	labelExpressionParser = mustNewLabelExpressionParser()
)

const (
	cacheSizeEnvVar  = "TELEPORT_EXPRESSION_CACHE_SIZE"
	defaultCacheSize = 1000
)

func mustNewLabelExpressionCache() *lru.Cache[string, labelExpression] {
	cache, err := newLabelExpressionCache()
	if err != nil {
		panic(trace.Wrap(err, "initializing label expression cache"))
	}
	return cache
}

func newLabelExpressionCache() (*lru.Cache[string, labelExpression], error) {
	cacheSize := defaultCacheSize
	if env := os.Getenv(cacheSizeEnvVar); env != "" {
		if envCacheSize, err := strconv.ParseUint(env, 10, 31); err != nil {
			log.WithError(err).Warn("Parsing " + cacheSizeEnvVar)
		} else {
			cacheSize = int(envCacheSize)
		}
	}
	cache, err := lru.New[string, labelExpression](cacheSize)
	return cache, trace.Wrap(err)
}

func mustNewLabelExpressionParser() *typical.Parser[labelExpressionEnv, bool] {
	parser, err := newLabelExpressionParser()
	if err != nil {
		panic(trace.Wrap(err, "failed to create label expression parser (this is a bug)"))
	}
	return parser
}

func newLabelExpressionParser() (*typical.Parser[labelExpressionEnv, bool], error) {
	parser, err := typical.NewParser[labelExpressionEnv, bool](typical.ParserSpec{
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
			"contains": typical.BinaryFunction[labelExpressionEnv](
				func(list []string, item string) (bool, error) {
					return slices.Contains(list, item), nil
				}),
			"regexp.match": typical.BinaryFunction[labelExpressionEnv](
				func(list []string, re string) (bool, error) {
					match, err := utils.RegexMatchesAny(list, re)
					if err != nil {
						return false, trace.Wrap(err, "invalid regular expression %q", re)
					}
					return match, nil
				}),
			// Use regexp.replace from lib/utils/parse to get behavior identical
			// to role templates.
			"regexp.replace": typical.TernaryFunction[labelExpressionEnv](parse.RegexpReplace),
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
