package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

var adjectives = []string{
	"brave", "calm", "eager", "fair", "glad",
	"keen", "bold", "warm", "wise", "swift",
	"bright", "cool", "gentle", "happy", "kind",
	"lively", "noble", "quick", "sharp", "steady",
}

var nouns = []string{
	"falcon", "river", "maple", "summit", "coral",
	"ember", "cedar", "brook", "crane", "dune",
	"flint", "grove", "heron", "jade", "lark",
	"moss", "oak", "pearl", "ridge", "stone",
}

// humanIDGenerator produces unique human-readable identifiers like "brave-falcon".
type humanIDGenerator struct {
	used map[string]struct{}
}

// newHumanIDGenerator creates a generator that tracks used IDs to ensure uniqueness.
func newHumanIDGenerator() *humanIDGenerator {
	return &humanIDGenerator{used: make(map[string]struct{})}
}

// Generate returns a unique human-readable identifier. On collision, it
// appends a numeric suffix (e.g. "brave-falcon-2").
func (g *humanIDGenerator) Generate() string {
	id := generateHumanID()
	if _, ok := g.used[id]; !ok {
		g.used[id] = struct{}{}

		return id
	}

	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", id, i)
		if _, ok := g.used[candidate]; !ok {
			g.used[candidate] = struct{}{}

			return candidate
		}
	}
}

func generateHumanID() string {
	adj := adjectives[randInt(len(adjectives))]
	noun := nouns[randInt(len(nouns))]

	return fmt.Sprintf("%s-%s", adj, noun)
}

func randInt(n int) int {
	max := big.NewInt(int64(n))

	v, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}

	return int(v.Int64())
}
