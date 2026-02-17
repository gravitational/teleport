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
	"encoding/binary"
	"hash/fnv"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"
)

// clusterOptions controls the clustering pipeline.
type clusterOptions struct {
	// Drain parameters.
	drainMaxDepth     int
	drainSimThreshold float64

	// Shingling parameters.
	shingleSize int // n-gram size for shingling (default 3)

	// MinHash parameters.
	numHashes int // signature size (default 128)

	// LSH parameters.
	lshBands int // number of bands (default 16)
	lshRows  int // rows per band (default 8); bands*rows must equal numHashes
}

// clusterDefaults returns sensible defaults.
func clusterDefaults() clusterOptions {
	return clusterOptions{
		drainMaxDepth:     4,
		drainSimThreshold: 0.4,
		shingleSize:       3,
		numHashes:         128,
		lshBands:          16,
		lshRows:           8,
	}
}

// textCluster represents a group of similar text documents.
type textCluster struct {
	// ID is a unique cluster identifier (0-indexed).
	ID int `json:"id"`

	// Template is the Drain-normalized representative text for the cluster.
	Template string `json:"template"`

	// Members contains the indices into the original input slice.
	Members []int `json:"-"`

	// Size is len(Members).
	Size int `json:"size"`
}

// clusterTexts groups a set of text documents by similarity using
// Drain normalization → shingling → MinHash → LSH.
//
// Each input text is a multi-line string (e.g., SSM run stdout/stderr).
// Returns clusters sorted by size descending.
func clusterTexts(texts []string, opts clusterOptions) []textCluster {
	if len(texts) == 0 {
		return nil
	}
	if opts.shingleSize == 0 {
		opts = clusterDefaults()
	}

	pipelineStart := time.Now()

	// Step 1: Normalize each text through Drain.
	// Two global phases: train all lines first so templates fully converge,
	// then normalize each text against the converged model.
	d := newDrain(drainConfig{
		maxDepth:     opts.drainMaxDepth,
		simThreshold: opts.drainSimThreshold,
	})
	// Phase 1: Train Drain on all lines from all texts.
	for _, text := range texts {
		trainText(d, text)
	}
	slog.Debug("Clustering: Drain training complete", "texts", len(texts), "templates", len(d.Clusters()), "elapsed", time.Since(pipelineStart).Round(time.Millisecond))

	// Phase 2: Normalize each text using converged templates.
	stepStart := time.Now()
	normalized := make([]string, len(texts))
	for i, text := range texts {
		normalized[i] = matchText(d, text)
	}
	// Log distribution of unique normalized texts.
	uniqueCounts := make(map[string]int, len(normalized))
	for _, n := range normalized {
		uniqueCounts[n]++
	}
	// Show top buckets by size.
	type bucket struct {
		preview string
		count   int
	}
	buckets := make([]bucket, 0, len(uniqueCounts))
	for text, count := range uniqueCounts {
		preview := text
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		preview = strings.ReplaceAll(preview, "\n", " | ")
		buckets = append(buckets, bucket{preview: preview, count: count})
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].count > buckets[j].count })
	topN := min(len(buckets), 5)
	for i := 0; i < topN; i++ {
		slog.Debug("Clustering: normalized text bucket", "rank", i+1, "count", buckets[i].count, "preview", buckets[i].preview)
	}
	slog.Debug("Clustering: Drain normalization complete", "unique_texts", len(uniqueCounts), "total_texts", len(normalized), "elapsed", time.Since(stepStart).Round(time.Millisecond))

	// Deduplicate: run Shingle/MinHash/LSH only on unique normalized texts,
	// then expand cluster memberships back to original indices.
	// This avoids O(N²) blowup in LSH when many texts normalize identically.
	type uniqueEntry struct {
		text    string
		members []int // original indices with this normalized text
	}
	uniqueMap := make(map[string]int) // normalized text → index in uniqueTexts
	var uniqueTexts []uniqueEntry
	for i, n := range normalized {
		if idx, ok := uniqueMap[n]; ok {
			uniqueTexts[idx].members = append(uniqueTexts[idx].members, i)
		} else {
			uniqueMap[n] = len(uniqueTexts)
			uniqueTexts = append(uniqueTexts, uniqueEntry{text: n, members: []int{i}})
		}
	}
	U := len(uniqueTexts)
	slog.Debug("Clustering: deduplicated for pipeline", "unique", U, "total", len(texts))

	// Step 2: Shingle each unique normalized text.
	stepStart = time.Now()
	shingleSets := make([]map[uint64]struct{}, U)
	for i, entry := range uniqueTexts {
		shingleSets[i] = shingle(entry.text, opts.shingleSize)
	}
	slog.Debug("Clustering: Shingling complete", "elapsed", time.Since(stepStart).Round(time.Millisecond))

	// Step 3: MinHash signatures.
	stepStart = time.Now()
	signatures := make([][]uint64, U)
	for i, set := range shingleSets {
		signatures[i] = minHashSignature(set, opts.numHashes)
	}
	slog.Debug("Clustering: MinHash complete", "elapsed", time.Since(stepStart).Round(time.Millisecond))

	// Step 4: LSH to find candidate pairs (among unique texts only).
	stepStart = time.Now()
	candidates := lshCandidates(signatures, opts.lshBands, opts.lshRows)
	slog.Debug("Clustering: LSH complete", "candidates", len(candidates), "elapsed", time.Since(stepStart).Round(time.Millisecond))

	// Step 5: Union-Find to merge candidates into clusters.
	stepStart = time.Now()
	uf := newUnionFind(U)
	for _, pair := range candidates {
		uf.union(pair[0], pair[1])
	}
	slog.Debug("Clustering: Union-Find complete", "elapsed", time.Since(stepStart).Round(time.Millisecond))

	// Step 6: Build cluster output, expanding unique-text clusters back
	// to original indices.
	groups := make(map[int][]int)
	for i := range uniqueTexts {
		root := uf.find(i)
		groups[root] = append(groups[root], uniqueTexts[i].members...)
	}

	clusters := make([]textCluster, 0, len(groups))
	clusterID := 0
	for root, members := range groups {
		template := uniqueTexts[root].text
		clusters = append(clusters, textCluster{
			ID:       clusterID,
			Template: template,
			Members:  members,
			Size:     len(members),
		})
		clusterID++
	}

	// Sort by size descending.
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Size > clusters[j].Size
	})

	// Re-assign IDs after sorting.
	for i := range clusters {
		clusters[i].ID = i
	}

	slog.Debug("Clustering: pipeline complete", "input_texts", len(texts), "clusters", len(clusters), "total_elapsed", time.Since(pipelineStart).Round(time.Millisecond))
	return clusters
}

// trainText feeds each line of a multi-line text through Drain for training.
// Call this for all texts before calling matchText.
func trainText(d *drain, text string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !isNoiseLine(line) {
			d.Train(line)
		}
	}
}

// matchText normalizes a text by matching each line against the trained
// Drain model, returning concatenated template lines.
func matchText(d *drain, text string) string {
	var templates []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isNoiseLine(line) {
			continue
		}
		tokens := tokenize(line)
		if len(tokens) == 0 {
			continue
		}
		best, _ := d.matchTokens(tokens)
		if best != nil {
			templates = append(templates, best.Template())
		} else {
			templates = append(templates, line)
		}
	}
	return strings.Join(templates, "\n")
}

// normalizeText is a convenience wrapper that trains and matches in one call.
// Used in tests.
func normalizeText(d *drain, text string) string {
	trainText(d, text)
	return matchText(d, text)
}

// isNoiseLine returns true for lines that are predominantly numeric noise
// (e.g., curl progress bars, download stats) which hurt clustering quality.
func isNoiseLine(line string) bool {
	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		return true
	}
	// Lines where >70% of tokens contain digits are noise.
	digitTokens := 0
	for _, tok := range tokens {
		if hasDigit(tok) {
			digitTokens++
		}
	}
	return float64(digitTokens)/float64(len(tokens)) > 0.7
}

// shingle produces a set of word-level n-gram hashes from the text.
func shingle(text string, n int) map[uint64]struct{} {
	words := strings.Fields(text)
	if len(words) < n {
		// If fewer words than shingle size, use the whole text as one shingle.
		set := make(map[uint64]struct{}, 1)
		set[hashString(text)] = struct{}{}
		return set
	}
	set := make(map[uint64]struct{}, len(words)-n+1)
	for i := 0; i <= len(words)-n; i++ {
		gram := strings.Join(words[i:i+n], " ")
		set[hashString(gram)] = struct{}{}
	}
	return set
}

// hashString returns a 64-bit FNV-1a hash of the string.
func hashString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// minHashSignature computes a MinHash signature of the given size for
// a shingle set. Each hash function is simulated by XOR-ing with a
// different seed.
func minHashSignature(set map[uint64]struct{}, numHashes int) []uint64 {
	sig := make([]uint64, numHashes)
	for i := range sig {
		sig[i] = math.MaxUint64
	}
	if len(set) == 0 {
		return sig
	}
	seeds := makeSeeds(numHashes)
	for hash := range set {
		for i, seed := range seeds {
			val := hash ^ seed
			if val < sig[i] {
				sig[i] = val
			}
		}
	}
	return sig
}

// makeSeeds generates deterministic seed values for MinHash.
func makeSeeds(n int) []uint64 {
	seeds := make([]uint64, n)
	for i := range seeds {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(i*0x517cc1b727220a95+0x6c62272e07bb0142))
		seeds[i] = binary.LittleEndian.Uint64(b)
	}
	return seeds
}

// lshCandidates uses Locality-Sensitive Hashing to find candidate pairs
// of similar documents from their MinHash signatures.
func lshCandidates(signatures [][]uint64, bands, rows int) [][2]int {
	if len(signatures) == 0 {
		return nil
	}

	seen := make(map[[2]int]struct{})
	var pairs [][2]int

	for b := range bands {
		buckets := make(map[uint64][]int)
		for docIdx, sig := range signatures {
			start := b * rows
			end := min(start+rows, len(sig))
			bandHash := hashBand(sig[start:end])
			buckets[bandHash] = append(buckets[bandHash], docIdx)
		}
		// Log largest bucket size per band.
		maxBucket := 0
		for _, docs := range buckets {
			if len(docs) > maxBucket {
				maxBucket = len(docs)
			}
		}
		if b == 0 || maxBucket > 100 {
			slog.Debug("LSH band stats", "band", b, "buckets", len(buckets), "max_bucket_size", maxBucket, "pairs_so_far", len(pairs))
		}
		for _, docs := range buckets {
			for i := range len(docs) {
				for j := i + 1; j < len(docs); j++ {
					pair := [2]int{docs[i], docs[j]}
					if _, ok := seen[pair]; !ok {
						seen[pair] = struct{}{}
						pairs = append(pairs, pair)
					}
				}
			}
		}
	}
	return pairs
}

// hashBand hashes a band (slice of signature values) into a single uint64.
func hashBand(band []uint64) uint64 {
	h := fnv.New64a()
	b := make([]byte, 8)
	for _, v := range band {
		binary.LittleEndian.PutUint64(b, v)
		h.Write(b)
	}
	return h.Sum64()
}

// unionFind implements a disjoint-set data structure with path compression
// and union by rank.
type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(n int) *unionFind {
	parent := make([]int, n)
	rank := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	return &unionFind{parent: parent, rank: rank}
}

func (uf *unionFind) find(x int) int {
	for uf.parent[x] != x {
		uf.parent[x] = uf.parent[uf.parent[x]] // path compression
		x = uf.parent[x]
	}
	return x
}

func (uf *unionFind) union(x, y int) {
	rx, ry := uf.find(x), uf.find(y)
	if rx == ry {
		return
	}
	if uf.rank[rx] < uf.rank[ry] {
		rx, ry = ry, rx
	}
	uf.parent[ry] = rx
	if uf.rank[rx] == uf.rank[ry] {
		uf.rank[rx]++
	}
}
