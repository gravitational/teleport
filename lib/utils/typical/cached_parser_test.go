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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type testLogger struct {
	loggedCount int
}

func (t *testLogger) Infof(msg string, args ...any) {
	t.loggedCount++
}

func TestCachedParser(t *testing.T) {
	// A simple cached parser with no environment that always returns an int.
	p, err := NewCachedParser[struct{}, int](ParserSpec{
		Functions: map[string]Function{
			"inc": UnaryFunction[struct{}](func(i int) (int, error) {
				return i + 1, nil
			}),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

	var logger testLogger
	p.logger = &logger

	// Sanity check we still get errors.
	_, err = p.Parse(`"hello"`)
	require.Error(t, err)
	require.ErrorContains(t, err, "expected type int, got value (hello) with type (string)")

	// Parse $defaultCacheSize unique expressions to fill the cache.
	for i := 0; i < defaultCacheSize; i++ {
		expr := fmt.Sprintf("inc(%d)", i)

		parsed, err := p.Parse(expr)
		require.NoError(t, err)

		// Sanity check the expression evaluates correctly.
		result, err := parsed.Evaluate(struct{}{})
		require.NoError(t, err)
		require.Equal(t, i+1, result)

		// Make sure the cache len matches what we expect.
		require.Equal(t, i+1, p.cache.Len())

		// Make sure the cache len doesn't increase after parsing the same
		// expression again.
		_, err = p.Parse(expr)
		require.NoError(t, err)
		require.Equal(t, i+1, p.cache.Len())
	}

	// Parse $logAfterEvictions-1 unique expressions to cause
	// $logAfterEvictions-1 cache evictions
	for i := 0; i < logAfterEvictions-1; i++ {
		expr := fmt.Sprintf("inc(%d)", defaultCacheSize+i)

		parsed, err := p.Parse(expr)
		require.NoError(t, err)

		// Sanity check the expression evaluates correctly.
		result, err := parsed.Evaluate(struct{}{})
		require.NoError(t, err)
		require.Equal(t, defaultCacheSize+i+1, result)

		// Make sure the cache len matches what we expect.
		require.Equal(t, defaultCacheSize, p.cache.Len())

		// Check that each new parsed expression results in an eviction.
		require.Equal(t, uint32(i+1), p.evictedCount.Load())

		// Check that no log was printed
		require.Equal(t, 0, logger.loggedCount)
	}

	// Parse one more expression
	expr := fmt.Sprintf("inc(%d)", defaultCacheSize+logAfterEvictions)
	_, err = p.Parse(expr)
	require.NoError(t, err)

	// Check a log was printed once after $logAfterEvictions evictions.
	require.Equal(t, 1, logger.loggedCount)

	// Parse another $logAfterEvictions unique expressions to cause
	// another $logAfterEvictions cache evictions and one more log
	for i := 0; i < logAfterEvictions; i++ {
		expr := fmt.Sprintf("inc(%d)", defaultCacheSize+logAfterEvictions+i+1)
		_, err := p.Parse(expr)
		require.NoError(t, err)
	}

	// Check a log was printed twice after 2*$logAfterEvictions evictions.
	require.Equal(t, 2, logger.loggedCount)
}
