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

package typical

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCachedParser(t *testing.T) {
	// A simple cached parser with no environment that always returns an int.
	p, err := NewCachedParser[struct{}, int](ParserSpec[struct{}]{
		Functions: map[string]Function{
			"inc": UnaryFunction[struct{}](func(i int) (int, error) {
				return i + 1, nil
			}),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, p)

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
	}

	// Parse one more expression
	expr := fmt.Sprintf("inc(%d)", defaultCacheSize+logAfterEvictions)
	_, err = p.Parse(expr)
	require.NoError(t, err)

	// Parse another $logAfterEvictions unique expressions to cause
	// another $logAfterEvictions cache evictions and one more log
	for i := 0; i < logAfterEvictions; i++ {
		expr := fmt.Sprintf("inc(%d)", defaultCacheSize+logAfterEvictions+i+1)
		_, err := p.Parse(expr)
		require.NoError(t, err)
	}
}
