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

package typical

import (
	"os"
	"strconv"
	"sync/atomic"

	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/sirupsen/logrus"
)

const (
	cacheSizeEnvVar   = "TELEPORT_EXPRESSION_CACHE_SIZE"
	defaultCacheSize  = 1000
	logAfterEvictions = 100
)

// newExpressionCache returns a new LRU cache meant to hold parsed expressions.
// The size of the cache defaults to 1000 but can be overridden with the
// TELEPORT_EXPRESSION_CACHE_SIZE environment variable. Each expression type
// will have its own unique cache with its own size.
func newExpressionCache[TExpr any]() (*lru.Cache[string, TExpr], error) {
	cacheSize := defaultCacheSize
	if env := os.Getenv(cacheSizeEnvVar); env != "" {
		if envCacheSize, err := strconv.ParseUint(env, 10, 31); err != nil {
			return nil, trace.Wrap(err)
		} else {
			cacheSize = int(envCacheSize)
		}
	}
	cache, err := lru.New[string, TExpr](cacheSize)
	return cache, trace.Wrap(err)
}

// CachedParser is a Parser that caches each parsed expression.
type CachedParser[TEnv, TResult any] struct {
	Parser[TEnv, TResult]
	cache        *lru.Cache[string, Expression[TEnv, TResult]]
	evictedCount atomic.Uint32
	logger       logger
}

// NewCachedParser creates a cached predicate expression parser with the given specification.
func NewCachedParser[TEnv, TResult any](spec ParserSpec, opts ...ParserOption) (*CachedParser[TEnv, TResult], error) {
	parser, err := NewParser[TEnv, TResult](spec, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cache, err := newExpressionCache[Expression[TEnv, TResult]]()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CachedParser[TEnv, TResult]{
		Parser: *parser,
		cache:  cache,
		logger: logrus.StandardLogger(),
	}, nil
}

// Parse checks if [expression] is already present in the cache and returns the
// cached version if present, or else parses the expression to produce an
// Expression[TEnv, TResult] which is stored in the cache and returned.
func (c *CachedParser[TEnv, TResult]) Parse(expression string) (Expression[TEnv, TResult], error) {
	if parsedExpr, ok := c.cache.Get(expression); ok {
		return parsedExpr, nil
	}
	parsedExpr, err := c.Parser.Parse(expression)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if evicted := c.cache.Add(expression, parsedExpr); evicted && c.evictedCount.Add(1)%logAfterEvictions == 0 {
		c.logger.Infof("%d entries have been evicted from the predicate expression cache, consider increasing TELEPORT_EXPRESSION_CACHE_SIZE",
			logAfterEvictions)
	}
	return parsedExpr, nil
}

type logger interface {
	Infof(fmt string, args ...any)
}
