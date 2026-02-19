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
	"log/slog"
	"sort"
	"strings"
	"time"

	minhashlsh "github.com/ekzhu/minhash-lsh"
)

// groupingOptions controls the grouping pipeline.
type groupingOptions struct {
	// Drain parameters.
	drainMaxDepth     int
	drainSimThreshold float64

	// Shingling parameters.
	shingleSize int // n-gram size for shingling (default 3)

	// MinHash parameters.
	numHashes int // signature size (default 128)

	// LSH parameters.
	lshThreshold float64 // Jaccard similarity threshold for LSH candidate detection (default 0.5)
}

// groupingDefaults returns sensible defaults.
func groupingDefaults() groupingOptions {
	return groupingOptions{
		drainMaxDepth:     4,
		drainSimThreshold: 0.4,
		shingleSize:       3,
		numHashes:         128,
		lshThreshold:      0.5,
	}
}

// withDefaults returns a copy of opts where any zero-valued field is
// replaced with the corresponding default from groupingDefaults.
func (opts groupingOptions) withDefaults() groupingOptions {
	defaults := groupingDefaults()
	if opts.drainMaxDepth <= 0 {
		opts.drainMaxDepth = defaults.drainMaxDepth
	}
	if opts.drainSimThreshold <= 0 {
		opts.drainSimThreshold = defaults.drainSimThreshold
	}
	if opts.shingleSize <= 0 {
		opts.shingleSize = defaults.shingleSize
	}
	if opts.numHashes <= 0 {
		opts.numHashes = defaults.numHashes
	}
	if opts.lshThreshold <= 0 {
		opts.lshThreshold = defaults.lshThreshold
	}
	return opts
}

// textGroup represents a group of similar text documents.
type textGroup struct {
	// ID is a unique group identifier (0-indexed).
	ID int `json:"id"`

	// Template is the Drain-normalized representative text for the group.
	Template string `json:"template"`

	// Members contains the indices into the original input slice.
	Members []int `json:"-"`

	// Size is len(Members).
	Size int `json:"size"`

	// Debug fields, populated when debug is requested.
	UniqueTexts int `json:"-"` // distinct Drain-normalized texts merged into this group
	ShingleSize int `json:"-"` // number of shingles for the representative text
}

// groupingStats holds pipeline-level diagnostics.
type groupingStats struct {
	InputTexts     int               `json:"input_texts"`
	DrainTemplates int               `json:"drain_templates"`
	UniqueTexts    int               `json:"unique_texts"`
	LSHCandidates  int               `json:"lsh_candidates"`
	FinalGroups    int               `json:"final_groups"`
	Elapsed        string            `json:"elapsed"`
	Similarities   []groupSimilarity `json:"similarities,omitempty"`
}

// groupSimilarity reports the estimated Jaccard similarity between two groups.
type groupSimilarity struct {
	GroupA     int     `json:"group_a"`
	GroupB     int     `json:"group_b"`
	Similarity float64 `json:"similarity"`
}

// uniqueEntry represents a unique normalized text and the original indices that map to it.
type uniqueEntry struct {
	text    string
	members []int // original indices with this normalized text
}

// bucket is used for debug logging of normalized text distribution.
type bucket struct {
	preview string
	count   int
}

// drainNormalize trains a Drain model on all texts, then normalizes each text
// using the converged templates. Returns normalized texts and the drain instance.
func drainNormalize(texts []string, opts groupingOptions) ([]string, *drain) {
	pipelineStart := time.Now()

	// Train Drain on all lines from all texts.
	d := newDrain(drainConfig{
		maxDepth:     opts.drainMaxDepth,
		simThreshold: opts.drainSimThreshold,
	})
	for _, text := range texts {
		trainText(d, text)
	}
	slog.Debug("Grouping: Drain training complete", "texts", len(texts), "templates", len(d.Clusters()), "elapsed", time.Since(pipelineStart).Round(time.Millisecond))

	// Normalize each text using converged templates.
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
	for i := range topN {
		slog.Debug("Grouping: normalized text bucket", "rank", i+1, "count", buckets[i].count, "preview", buckets[i].preview)
	}
	slog.Debug("Grouping: Drain normalization complete", "unique_texts", len(uniqueCounts), "total_texts", len(normalized), "elapsed", time.Since(stepStart).Round(time.Millisecond))

	return normalized, d
}

// deduplicateTexts deduplicates normalized texts, returning unique entries
// that track which original indices map to each unique text.
func deduplicateTexts(normalized []string) []uniqueEntry {
	uniqueMap := make(map[string]int) // normalized text → index in result
	var result []uniqueEntry
	for i, n := range normalized {
		if idx, ok := uniqueMap[n]; ok {
			result[idx].members = append(result[idx].members, i)
		} else {
			uniqueMap[n] = len(result)
			result = append(result, uniqueEntry{text: n, members: []int{i}})
		}
	}
	slog.Debug("Grouping: deduplicated for pipeline", "unique", len(result), "total", len(normalized))
	return result
}

// computeSignatures computes MinHash signatures for each unique entry using
// word-level n-gram shingling. Returns signatures and shingle counts.
func computeSignatures(entries []uniqueEntry, opts groupingOptions) ([][]uint64, []int) {
	stepStart := time.Now()
	signatures := make([][]uint64, len(entries))
	shingleCounts := make([]int, len(entries))
	for i, entry := range entries {
		m := minhashlsh.NewMinhash(1, opts.numHashes)
		words := strings.Fields(entry.text)
		n := opts.shingleSize
		count := 0
		if len(words) < n {
			m.Push([]byte(entry.text))
			count = 1
		} else {
			for j := 0; j <= len(words)-n; j++ {
				gram := strings.Join(words[j:j+n], " ")
				m.Push([]byte(gram))
				count++
			}
		}
		signatures[i] = m.Signature()
		shingleCounts[i] = count
	}
	slog.Debug("Grouping: Shingle+MinHash complete", "elapsed", time.Since(stepStart).Round(time.Millisecond))
	return signatures, shingleCounts
}

// lshCandidates uses LSH to find candidate pairs of similar documents
// among the unique texts. Returns deduplicated candidate pairs.
func lshCandidates(signatures [][]uint64, opts groupingOptions, n int) [][2]int {
	stepStart := time.Now()
	lsh := minhashlsh.NewMinhashLSH64(opts.numHashes, opts.lshThreshold, n)
	for i, sig := range signatures {
		lsh.Add(i, sig)
	}
	lsh.Index()

	// Query each document to find its similar neighbors.
	seen := make(map[[2]int]struct{})
	var candidates [][2]int
	for i, sig := range signatures {
		neighbors := lsh.Query(sig)
		for _, nb := range neighbors {
			j := nb.(int)
			if j <= i {
				continue
			}
			pair := [2]int{i, j}
			if _, ok := seen[pair]; !ok {
				seen[pair] = struct{}{}
				candidates = append(candidates, pair)
			}
		}
	}
	slog.Debug("Grouping: LSH complete", "candidates", len(candidates), "elapsed", time.Since(stepStart).Round(time.Millisecond))
	return candidates
}

// groupTexts groups a set of text documents by similarity using
// Drain normalization → shingling → MinHash → LSH.
//
// Each input text is a multi-line string (e.g., SSM run stdout/stderr).
// Returns groups sorted by size descending.
func groupTexts(texts []string, opts groupingOptions) ([]textGroup, groupingStats) {
	if len(texts) == 0 {
		return nil, groupingStats{}
	}
	opts = opts.withDefaults()

	pipelineStart := time.Now()

	// Step 1: Drain normalization.
	normalized, d := drainNormalize(texts, opts)

	// Step 2: Deduplicate normalized texts.
	uniqueTexts := deduplicateTexts(normalized)
	U := len(uniqueTexts)

	// Step 3: Shingle + MinHash each unique normalized text.
	signatures, shingleCounts := computeSignatures(uniqueTexts, opts)

	// Step 4: LSH to find candidate pairs.
	candidates := lshCandidates(signatures, opts, U)

	// Step 5: Union-Find to merge candidates into groups.
	stepStart := time.Now()
	uf := newUnionFind(U)
	for _, pair := range candidates {
		uf.union(pair[0], pair[1])
	}
	slog.Debug("Grouping: Union-Find complete", "elapsed", time.Since(stepStart).Round(time.Millisecond))

	// Step 6: Build group output, expanding unique-text groups back
	// to original indices.
	rawGroups := make(map[int][]int)      // root → original indices
	groupUnique := make(map[int][]int)    // root → unique text indices
	for i := range uniqueTexts {
		root := uf.find(i)
		rawGroups[root] = append(rawGroups[root], uniqueTexts[i].members...)
		groupUnique[root] = append(groupUnique[root], i)
	}

	result := make([]textGroup, 0, len(rawGroups))
	groupID := 0
	for root, members := range rawGroups {
		template := uniqueTexts[root].text
		shingleCount := shingleCounts[root]
		result = append(result, textGroup{
			ID:          groupID,
			Template:    template,
			Members:     members,
			Size:        len(members),
			UniqueTexts: len(groupUnique[root]),
			ShingleSize: shingleCount,
		})
		groupID++
	}

	// Sort by size descending, breaking ties by template text for deterministic output.
	// BUG: without the tiebreaker, map iteration order + unstable sort caused
	// non-deterministic group IDs for same-sized groups, making --group-by-account
	// output differ from ungrouped output even with identical input data.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Size != result[j].Size {
			return result[i].Size > result[j].Size
		}
		return result[i].Template < result[j].Template
	})

	// Re-assign IDs after sorting.
	for i := range result {
		result[i].ID = i
	}

	elapsed := time.Since(pipelineStart).Round(time.Millisecond)
	slog.Debug("Grouping: pipeline complete", "input_texts", len(texts), "groups", len(result), "total_elapsed", elapsed)

	stats := groupingStats{
		InputTexts:     len(texts),
		DrainTemplates: len(d.Clusters()),
		UniqueTexts:    U,
		LSHCandidates:  len(candidates),
		FinalGroups:    len(result),
		Elapsed:        elapsed.String(),
	}

	// Compute pairwise Jaccard similarities between group representatives.
	// Only compute for small group counts to avoid O(N²) blowup.
	if len(result) <= 50 {
		// Map group back to its root unique text index.
		rootForTemplate := make(map[string]int)
		for root := range rawGroups {
			rootForTemplate[uniqueTexts[root].text] = root
		}
		groupRoots := make([]int, len(result))
		for i, g := range result {
			groupRoots[i] = rootForTemplate[g.Template]
		}

		for i := 0; i < len(result); i++ {
			for j := i + 1; j < len(result); j++ {
				sim := estimateJaccard(signatures[groupRoots[i]], signatures[groupRoots[j]])
				if sim > 0.01 {
					stats.Similarities = append(stats.Similarities, groupSimilarity{
						GroupA:     result[i].ID,
						GroupB:     result[j].ID,
						Similarity: sim,
					})
				}
			}
		}
		sort.Slice(stats.Similarities, func(i, j int) bool {
			return stats.Similarities[i].Similarity > stats.Similarities[j].Similarity
		})
	}

	return result, stats
}

// trainText feeds each line of a multi-line text through Drain for training.
// Call this for all texts before calling matchText.
func trainText(d *drain, text string) {
	for line := range strings.SplitSeq(text, "\n") {
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
	for line := range strings.SplitSeq(text, "\n") {
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

// noiseDigitRatio is the threshold for isNoiseLine: lines where more than
// this fraction of tokens contain digits are treated as noise.
const noiseDigitRatio = 0.7

// isNoiseLine returns true for lines that are predominantly numeric noise
// (e.g., curl progress bars, download stats) which hurt grouping quality.
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
	return float64(digitTokens)/float64(len(tokens)) > noiseDigitRatio
}

// estimateJaccard estimates the Jaccard similarity between two documents
// from their MinHash signatures. The estimate is the fraction of hash
// positions where the signatures agree.
func estimateJaccard(sigA, sigB []uint64) float64 {
	if len(sigA) != len(sigB) || len(sigA) == 0 {
		return 0
	}
	agree := 0
	for i := range sigA {
		if sigA[i] == sigB[i] {
			agree++
		}
	}
	return float64(agree) / float64(len(sigA))
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
