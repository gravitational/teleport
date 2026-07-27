/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestHumanIDGeneratorCollisionSuffix(t *testing.T) {
	g := newHumanIDGenerator()

	// Saturate all 400 base adj-noun combos so Generate() falls through to the numeric-suffix loop.
	for _, adj := range adjectives {
		for _, noun := range nouns {
			g.used[fmt.Sprintf("%s-%s", adj, noun)] = struct{}{}
		}
	}

	id := g.Generate()
	if !strings.HasSuffix(id, "-2") {
		t.Errorf("expected -2 suffix after base collision, got %q", id)
	}

	if _, ok := g.used[id]; !ok {
		t.Errorf("generated id %q not recorded as used", id)
	}
}

func TestHumanIDGeneratorMultipleCollisions(t *testing.T) {
	g := newHumanIDGenerator()

	// Saturate every base + base-2 so the next Generate() resolves to -3 regardless of base.
	for _, adj := range adjectives {
		for _, noun := range nouns {
			base := fmt.Sprintf("%s-%s", adj, noun)
			g.used[base] = struct{}{}
			g.used[base+"-2"] = struct{}{}
		}
	}

	id := g.Generate()
	if !strings.HasSuffix(id, "-3") {
		t.Errorf("expected -3 suffix after base + -2 collision, got %q", id)
	}
}

func TestHumanIDGeneratorUniqueness(t *testing.T) {
	g := newHumanIDGenerator()

	// Exceed the 400 base namespace to force suffix collisions and verify uniqueness.
	seen := make(map[string]struct{})
	for range 1000 {
		id := g.Generate()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id: %q", id)
		}

		seen[id] = struct{}{}
	}
}
