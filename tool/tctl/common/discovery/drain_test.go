// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDrainBasicTemplateExtraction(t *testing.T) {
	d := newDrain(drainDefaults())

	// Train on similar lines that differ only in a numeric token.
	d.Train("Connection from 10.0.1.5 failed timeout after 30s")
	d.Train("Connection from 10.0.2.8 failed timeout after 45s")
	d.Train("Connection from 192.168.1.1 failed timeout after 60s")

	clusters := d.Clusters()
	require.Len(t, clusters, 1, "similar lines should merge into one cluster")
	assert.Equal(t, 3, clusters[0].count)
	// IPs and durations contain digits, so they become wildcards.
	assert.Equal(t, "Connection from <*> failed timeout after <*>", clusters[0].Template())
}

func TestDrainDistinctTemplates(t *testing.T) {
	d := newDrain(drainDefaults())

	// Two very different log lines should produce separate clusters.
	d.Train("Starting service on port 8080")
	d.Train("Starting service on port 9090")
	d.Train("Disk usage exceeded threshold at /dev/sda1")
	d.Train("Disk usage exceeded threshold at /dev/sdb2")

	clusters := d.Clusters()
	require.Len(t, clusters, 2, "distinct log patterns should be separate clusters")

	templates := make(map[string]int)
	for _, c := range clusters {
		templates[c.Template()] = c.count
	}
	assert.Equal(t, 2, templates["Starting service on port <*>"])
	assert.Equal(t, 2, templates["Disk usage exceeded threshold at <*>"])
}

func TestDrainEmptyAndSingleToken(t *testing.T) {
	d := newDrain(drainDefaults())

	// Empty line should return nil.
	assert.Nil(t, d.Train(""))
	assert.Nil(t, d.Train("   "))

	// Single token.
	c := d.Train("ERROR")
	require.NotNil(t, c)
	assert.Equal(t, "ERROR", c.Template())
}

func TestDrainWildcardOnDigits(t *testing.T) {
	d := newDrain(drainDefaults())

	d.Train("request id abc123 completed in 50ms")
	d.Train("request id def456 completed in 120ms")

	clusters := d.Clusters()
	require.Len(t, clusters, 1)
	// "abc123", "def456", "50ms", "120ms" all contain digits.
	assert.Equal(t, "request id <*> completed in <*>", clusters[0].Template())
}

func TestDrainDifferentLengths(t *testing.T) {
	d := newDrain(drainDefaults())

	// Lines of different token lengths should never merge.
	d.Train("short line")
	d.Train("this is a much longer line with more tokens")

	clusters := d.Clusters()
	require.Len(t, clusters, 2)
}

func TestDrainSimThreshold(t *testing.T) {
	// High similarity threshold — only very similar lines merge.
	d := newDrain(drainConfig{maxDepth: 4, simThreshold: 0.9})

	d.Train("error processing request for user alpha in region west")
	d.Train("error processing request for user beta in region east")

	clusters := d.Clusters()
	// With 0.9 threshold, "alpha"/"beta" and "west"/"east" differ →
	// 7/9 match = 0.78 < 0.9, so they should NOT merge.
	require.Len(t, clusters, 2)
}

func TestDrainMultipleClustersCount(t *testing.T) {
	d := newDrain(drainDefaults())

	for range 10 {
		d.Train("GET /api/v1/users returned 200 in 50ms")
	}
	for range 5 {
		d.Train("POST /api/v1/orders returned 500 in 200ms")
	}

	clusters := d.Clusters()
	require.Len(t, clusters, 2)

	counts := make(map[string]int)
	for _, c := range clusters {
		counts[c.Template()] = c.count
	}
	// Both templates have digits in paths and numbers, so they get wildcards.
	// The key structural tokens (GET vs POST, users vs orders) keep them separate.
	total := 0
	for _, count := range counts {
		total += count
	}
	assert.Equal(t, 15, total)
}

func TestTokenSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		template []string
		tokens   []string
		expected float64
	}{
		{
			name:     "identical",
			template: []string{"hello", "world"},
			tokens:   []string{"hello", "world"},
			expected: 1.0,
		},
		{
			name:     "half match",
			template: []string{"hello", "world"},
			tokens:   []string{"hello", "there"},
			expected: 0.5,
		},
		{
			name:     "wildcard ignored",
			template: []string{"hello", drainWildcard, "world"},
			tokens:   []string{"hello", "foo", "world"},
			expected: 2.0 / 3.0,
		},
		{
			name:     "empty",
			template: []string{},
			tokens:   []string{},
			expected: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim := tokenSimilarity(tt.template, tt.tokens)
			assert.InDelta(t, tt.expected, sim, 0.001)
		})
	}
}

func TestHasDigit(t *testing.T) {
	assert.True(t, hasDigit("abc123"))
	assert.True(t, hasDigit("0"))
	assert.False(t, hasDigit("hello"))
	assert.False(t, hasDigit(""))
}
