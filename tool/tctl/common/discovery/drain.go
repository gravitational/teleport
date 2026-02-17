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
	"strings"
)

// drainWildcard is the placeholder token used when Drain generalizes a
// variable position in a log template.
const drainWildcard = "<*>"

// drainConfig holds tunable parameters for the Drain algorithm.
type drainConfig struct {
	// maxDepth controls the maximum depth of the prefix tree (excluding
	// the root and length nodes). Deeper trees produce more specific
	// templates; shallower trees generalize more aggressively.
	maxDepth int

	// simThreshold is the fraction of tokens that must match (non-wildcard)
	// for a log line to be assigned to an existing cluster rather than
	// creating a new one. Range [0, 1].
	simThreshold float64
}

// drainDefaults returns a drainConfig with sensible defaults.
func drainDefaults() drainConfig {
	return drainConfig{
		maxDepth:     4,
		simThreshold: 0.4,
	}
}

// logCluster represents a discovered log template and the number of lines
// that matched it.
type logCluster struct {
	tokens []string
	count  int
}

// Template returns the human-readable template string for this cluster.
func (c *logCluster) Template() string {
	return strings.Join(c.tokens, " ")
}

// drainNode is an internal prefix-tree node.
type drainNode struct {
	children map[string]*drainNode
	clusters []*logCluster
}

func newDrainNode() *drainNode {
	return &drainNode{
		children: make(map[string]*drainNode),
	}
}

// drain implements the Drain algorithm for online log template mining.
//
// Reference: He, Pinjia, et al. "Drain: An online log parsing approach with
// fixed depth tree." 2017 IEEE International Conference on Web Services.
type drain struct {
	cfg  drainConfig
	root *drainNode // root → length-bucket → prefix tokens → clusters
}

// newDrain creates a new Drain instance with the given configuration.
func newDrain(cfg drainConfig) *drain {
	return &drain{
		cfg:  cfg,
		root: newDrainNode(),
	}
}

// Train processes a single log line, either matching it to an existing
// cluster or creating a new one. Returns the matched/created cluster.
func (d *drain) Train(line string) *logCluster {
	tokens := tokenize(line)
	if len(tokens) == 0 {
		return nil
	}
	return d.train(tokens)
}

func (d *drain) train(tokens []string) *logCluster {
	// Step 1: Navigate to the length bucket.
	lengthKey := lengthBucketKey(len(tokens))
	lengthNode, ok := d.root.children[lengthKey]
	if !ok {
		lengthNode = newDrainNode()
		d.root.children[lengthKey] = lengthNode
	}

	// Step 2: Walk the prefix tree up to maxDepth.
	node := lengthNode
	for depth := 0; depth < d.cfg.maxDepth && depth < len(tokens); depth++ {
		tok := tokens[depth]
		if hasDigit(tok) {
			tok = drainWildcard
		}
		child, ok := node.children[tok]
		if !ok {
			child = newDrainNode()
			node.children[tok] = child
		}
		node = child
	}

	// Step 3: Search leaf clusters for a match.
	bestCluster, bestSim := d.findBestCluster(node.clusters, tokens)
	if bestCluster != nil && bestSim >= d.cfg.simThreshold {
		bestCluster.count++
		// Update the template: any position where the new line differs
		// from the template gets replaced with a wildcard.
		for i, tok := range tokens {
			if bestCluster.tokens[i] != tok {
				bestCluster.tokens[i] = drainWildcard
			}
		}
		return bestCluster
	}

	// Step 4: No match — create a new cluster.
	newCluster := &logCluster{
		tokens: make([]string, len(tokens)),
		count:  1,
	}
	copy(newCluster.tokens, tokens)
	node.clusters = append(node.clusters, newCluster)
	return newCluster
}

// findBestCluster returns the cluster with the highest token similarity
// to the given tokens, along with the similarity score.
func (d *drain) findBestCluster(clusters []*logCluster, tokens []string) (*logCluster, float64) {
	var best *logCluster
	bestSim := -1.0
	for _, c := range clusters {
		if len(c.tokens) != len(tokens) {
			continue
		}
		sim := tokenSimilarity(c.tokens, tokens)
		if sim > bestSim {
			bestSim = sim
			best = c
		}
	}
	return best, bestSim
}

// matchTokens finds the best matching cluster for the given tokens
// without modifying any state. Used for read-only lookups after training.
func (d *drain) matchTokens(tokens []string) (*logCluster, float64) {
	lengthKey := lengthBucketKey(len(tokens))
	lengthNode, ok := d.root.children[lengthKey]
	if !ok {
		return nil, 0
	}
	node := lengthNode
	for depth := 0; depth < d.cfg.maxDepth && depth < len(tokens); depth++ {
		tok := tokens[depth]
		if hasDigit(tok) {
			tok = drainWildcard
		}
		child, ok := node.children[tok]
		if !ok {
			// Try wildcard path.
			child, ok = node.children[drainWildcard]
			if !ok {
				return nil, 0
			}
		}
		node = child
	}
	return d.findBestCluster(node.clusters, tokens)
}

// Clusters returns all discovered log clusters.
func (d *drain) Clusters() []*logCluster {
	var result []*logCluster
	d.collectClusters(d.root, &result)
	return result
}

func (d *drain) collectClusters(node *drainNode, result *[]*logCluster) {
	*result = append(*result, node.clusters...)
	for _, child := range node.children {
		d.collectClusters(child, result)
	}
}

// tokenSimilarity computes the fraction of positions where the template
// and the candidate line share the same token (ignoring wildcard positions).
func tokenSimilarity(template, tokens []string) float64 {
	if len(template) == 0 {
		return 0
	}
	matches := 0
	for i, t := range template {
		if t == drainWildcard {
			continue
		}
		if t == tokens[i] {
			matches++
		}
	}
	return float64(matches) / float64(len(template))
}

// tokenize splits a log line into tokens on whitespace.
func tokenize(line string) []string {
	return strings.Fields(line)
}

// hasDigit returns true if the string contains at least one digit.
func hasDigit(s string) bool {
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return true
		}
	}
	return false
}

// lengthBucketKey returns the map key for a token-length bucket.
func lengthBucketKey(n int) string {
	return strings.Repeat(".", n) // cheap, unique per length
}
