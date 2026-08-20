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
