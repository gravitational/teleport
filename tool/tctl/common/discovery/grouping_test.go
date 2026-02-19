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
	"fmt"
	"strings"
	"testing"

	minhashlsh "github.com/ekzhu/minhash-lsh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMinhashSig computes a MinHash signature for text using word-level n-gram shingling.
func testMinhashSig(text string, n, numHashes int) []uint64 {
	m := minhashlsh.NewMinhash(1, numHashes)
	words := strings.Fields(text)
	if len(words) < n {
		m.Push([]byte(text))
	} else {
		for i := 0; i <= len(words)-n; i++ {
			gram := strings.Join(words[i:i+n], " ")
			m.Push([]byte(gram))
		}
	}
	return m.Signature()
}

func TestMinHashSimilarTexts(t *testing.T) {
	// Two similar texts should have similar signatures.
	text1 := "error failed to install teleport on instance disk full on /dev/xvda1"
	text2 := "error failed to install teleport on instance disk full on /dev/xvdb2"

	sig1 := testMinhashSig(text1, 3, 128)
	sig2 := testMinhashSig(text2, 3, 128)

	similarity := estimateJaccard(sig1, sig2)
	t.Logf("Jaccard similarity: %.3f", similarity)
	assert.Greater(t, similarity, 0.5, "similar texts should have high MinHash similarity")
}

func TestEstimateJaccardMultiLine(t *testing.T) {
	// Simulate the real-world case: two multi-line templates that differ on one line.
	base := `Offloading the installation part to the generic Teleport install script
Downloading from https://cdn.teleport.dev/teleport-ent-v18.6.5-linux-amd64-bin.tar.gz
> <*> enable --proxy discover-dev-5.cloud.gravitational.io:443
The install script %s returned a non-zero exit code
INFO [UPDATER] Initiating installation target_version:18.6.5
INFO [UPDATER] Downloading Teleport tarball size:217917539
ERRO [UPDATER] Command failed error:
ERROR REPORT:
Original Error: size of download exceeds available disk space <*> bytes
User Message: failed to install
failed to run commands: exit status <*>`

	text1 := fmt.Sprintf(base, "(/tmp/tmp.lKTtCLNGKQ)")
	text2 := fmt.Sprintf(base, "(/tmp/tmp.GPmSRPxXFj)")
	text3 := fmt.Sprintf(base, "<*>")

	sig1 := testMinhashSig(text1, 3, 128)
	sig2 := testMinhashSig(text2, 3, 128)
	sig3 := testMinhashSig(text3, 3, 128)

	sim12 := estimateJaccard(sig1, sig2)
	sim13 := estimateJaccard(sig1, sig3)
	sim23 := estimateJaccard(sig2, sig3)

	t.Logf("Jaccard(text1, text2) = %.3f (different temp paths)", sim12)
	t.Logf("Jaccard(text1, text3) = %.3f (literal vs wildcard)", sim13)
	t.Logf("Jaccard(text2, text3) = %.3f (literal vs wildcard)", sim23)

	assert.Greater(t, sim12, 0.5, "texts differing only in temp path should be similar")
	assert.Greater(t, sim13, 0.5, "text with literal vs wildcard temp path should be similar")
}

func TestMinHashDissimilarTexts(t *testing.T) {
	text1 := "error failed to install teleport disk full"
	text2 := "curl could not resolve host dns failure timeout"

	sig1 := testMinhashSig(text1, 3, 128)
	sig2 := testMinhashSig(text2, 3, 128)

	similarity := estimateJaccard(sig1, sig2)
	assert.Less(t, similarity, 0.3, "dissimilar texts should have low MinHash similarity")
}

func TestLSHCandidates(t *testing.T) {
	// Create 3 similar and 2 different texts.
	texts := []string{
		"error installing teleport on server disk space full",
		"error installing teleport on server disk space full",
		"error installing teleport on host disk space full",
		"curl dns resolution failed cannot reach repository",
		"curl dns resolution failed cannot reach repository",
	}

	lsh := minhashlsh.NewMinhashLSH64(128, 0.5, len(texts))
	for i, text := range texts {
		sig := testMinhashSig(text, 3, 128)
		lsh.Add(i, sig)
	}
	lsh.Index()

	// Query the first text — should find at least (0,1) as identical.
	sig0 := testMinhashSig(texts[0], 3, 128)
	results := lsh.Query(sig0)

	hasResult := func(target int) bool {
		for _, r := range results {
			if r.(int) == target {
				return true
			}
		}
		return false
	}
	assert.True(t, hasResult(1), "identical texts should be candidates")

	// Query a DNS text — should find other DNS text.
	sig3 := testMinhashSig(texts[3], 3, 128)
	results3 := lsh.Query(sig3)
	hasResult3 := func(target int) bool {
		for _, r := range results3 {
			if r.(int) == target {
				return true
			}
		}
		return false
	}
	assert.True(t, hasResult3(4), "identical texts should be candidates")
}

func TestUnionFind(t *testing.T) {
	uf := newUnionFind(5)

	// Initially all separate.
	for i := range 5 {
		assert.Equal(t, i, uf.find(i))
	}

	// Union 0-1, 2-3.
	uf.union(0, 1)
	uf.union(2, 3)
	assert.Equal(t, uf.find(0), uf.find(1))
	assert.Equal(t, uf.find(2), uf.find(3))
	assert.NotEqual(t, uf.find(0), uf.find(2))

	// Union 1-3 should merge both groups.
	uf.union(1, 3)
	assert.Equal(t, uf.find(0), uf.find(3))

	// 4 is still separate.
	assert.NotEqual(t, uf.find(0), uf.find(4))
}

func TestGroupTextsEndToEnd(t *testing.T) {
	// Simulate SSM run outputs with three failure types.
	diskFullTemplate := func(instanceID, device string, pct int) string {
		return fmt.Sprintf(`Installing Teleport v16.4.12 on %s...
Downloading package from https://cdn.teleport.dev/teleport-v16.4.12.tar.gz
Extracting to /tmp/teleport-install
No space left on device %s (%d%% used)
Installation failed: disk full`, instanceID, device, pct)
	}

	dnsFailTemplate := func(instanceID, host string) string {
		return fmt.Sprintf(`Installing Teleport v16.4.12 on %s...
Downloading package from https://cdn.teleport.dev/teleport-v16.4.12.tar.gz
curl: (6) Could not resolve host: %s
Download failed: DNS resolution error`, instanceID, host)
	}

	tokenExpiredTemplate := func(instanceID, token string) string {
		return fmt.Sprintf(`Installing Teleport v16.4.12 on %s...
Downloaded successfully
Configuring Teleport with token %s
teleport: error: join token has expired
Configuration failed: invalid token`, instanceID, token)
	}

	var texts []string
	// 10 disk full errors.
	for i := range 10 {
		texts = append(texts, diskFullTemplate(
			fmt.Sprintf("i-%012d", i),
			fmt.Sprintf("/dev/xvd%c", 'a'+byte(i%3)),
			90+i%10,
		))
	}
	// 5 DNS failures.
	for i := range 5 {
		texts = append(texts, dnsFailTemplate(
			fmt.Sprintf("i-%012d", 100+i),
			fmt.Sprintf("cdn%d.teleport.dev", i),
		))
	}
	// 3 token expired.
	for i := range 3 {
		texts = append(texts, tokenExpiredTemplate(
			fmt.Sprintf("i-%012d", 200+i),
			fmt.Sprintf("tok-%d-expired", i),
		))
	}

	groups, _ := groupTexts(texts, groupingDefaults())
	require.NotEmpty(t, groups, "should produce groups")

	t.Logf("Got %d groups:", len(groups))
	for _, g := range groups {
		t.Logf("  Group %d: %d members", g.ID, g.Size)
		lines := strings.Split(g.Template, "\n")
		for i, line := range lines {
			if i >= 3 {
				t.Logf("    ...")
				break
			}
			t.Logf("    %s", line)
		}
	}

	// Verify the largest group has the disk-full errors.
	assert.GreaterOrEqual(t, groups[0].Size, 8,
		"largest group should contain most disk-full errors")

	// Verify total members equals input size.
	total := 0
	for _, g := range groups {
		total += g.Size
	}
	assert.Equal(t, len(texts), total, "all texts should be assigned to a group")
}

func TestGroupTextsEmpty(t *testing.T) {
	groups, _ := groupTexts(nil, groupingDefaults())
	assert.Nil(t, groups)

	groups, _ = groupTexts([]string{}, groupingDefaults())
	assert.Nil(t, groups)
}

func TestGroupTextsSingleItem(t *testing.T) {
	groups, _ := groupTexts([]string{"single error message"}, groupingDefaults())
	require.Len(t, groups, 1)
	assert.Equal(t, 1, groups[0].Size)
	assert.Equal(t, []int{0}, groups[0].Members)
}

func TestGroupTextsIdentical(t *testing.T) {
	texts := make([]string, 20)
	for i := range texts {
		texts[i] = "exact same error message on every host"
	}

	groups, _ := groupTexts(texts, groupingDefaults())
	require.Len(t, groups, 1)
	assert.Equal(t, 20, groups[0].Size)
}

func TestGroupTextsRealisticSSMOutput(t *testing.T) {
	// Simulate realistic SSM run outputs with:
	// - Same failure type (disk full) but different paths, download stats
	// - A different failure type (DNS resolution)
	// - Curl progress bars (numeric noise that should be filtered)
	diskFull := func(i int) string {
		return fmt.Sprintf(`Offloading the installation part to the generic Teleport install script
Downloading from https://cdn.teleport.dev/teleport-ent-v18.6.5-linux-amd64-bin.tar.gz and extracting teleport to /root/tmp.%08d ...
> /root/tmp.%08d/bin/teleport-update enable --proxy cluster.example.com:443
The install script (/tmp/tmp.%08d) returned a non-zero exit code
  %% Total    %% Received  Average Speed   Time    Time     Time  Current
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0
  %d  207M   %d 5580k    0     0  16.1M      0  0:00:12 --:--:--  0:00:12 16.0M
2026-02-16T19:37:40.725Z INFO [UPDATER]   Initiating installation. target_version:18.6.5
2026-02-16T19:37:42.496Z INFO [UPDATER]   Downloading Teleport tarball. size:217917539
2026-02-16T19:37:42.640Z ERRO [UPDATER]   Command failed. error:
ERROR REPORT:
Original Error: size of download (217917539 bytes) exceeds available disk space (%d bytes)
User Message: failed to install
failed to run commands: exit status 1`, i, i, i+1000, i%100, i%100, 100000+i*1000)
	}

	dnsFailure := func(i int) string {
		return fmt.Sprintf(`Offloading the installation part to the generic Teleport install script
Downloading from https://cdn.teleport.dev/teleport-ent-v18.6.5-linux-amd64-bin.tar.gz
curl: (6) Could not resolve host: cdn.teleport.dev
  %% Total    %% Received  Average Speed   Time    Time     Time  Current
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0
Download failed with exit code %d
failed to run commands: exit status 1`, 6+i%3)
	}

	var texts []string
	// 50 disk-full errors with varying paths and download stats.
	for i := range 50 {
		texts = append(texts, diskFull(i))
	}
	// 20 DNS failures.
	for i := range 20 {
		texts = append(texts, dnsFailure(i))
	}

	groups, _ := groupTexts(texts, groupingDefaults())
	require.NotEmpty(t, groups)

	t.Logf("Got %d groups from 70 outputs:", len(groups))
	for _, g := range groups {
		t.Logf("  Group %d: %d members", g.ID, g.Size)
	}

	// Should produce 2 main groups (disk-full and DNS).
	// Allow up to 4 total groups for edge cases.
	assert.LessOrEqual(t, len(groups), 4,
		"should produce a small number of groups for 2 failure types")

	// The largest group should be the disk-full group.
	assert.GreaterOrEqual(t, groups[0].Size, 40,
		"largest group should contain most disk-full errors")

	// Total should equal input size.
	total := 0
	for _, g := range groups {
		total += g.Size
	}
	assert.Equal(t, 70, total)
}

func TestIsNoiseLine(t *testing.T) {
	// Curl progress bar lines are noise.
	assert.True(t, isNoiseLine("  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0"))
	assert.True(t, isNoiseLine("  2  207M    2 5580k    0     0  16.1M      0  0:00:12 --:--:--  0:00:12 16.0M"))

	// Regular log lines are not noise.
	assert.False(t, isNoiseLine("Downloading from https://cdn.teleport.dev/teleport.tar.gz"))
	assert.False(t, isNoiseLine("failed to run commands: exit status 1"))
	assert.False(t, isNoiseLine("ERROR REPORT: disk full"))
}

func TestGroupingOptionsWithDefaults(t *testing.T) {
	t.Parallel()
	defaults := groupingDefaults()

	t.Run("zero value gets all defaults", func(t *testing.T) {
		got := groupingOptions{}.withDefaults()
		require.Equal(t, defaults, got)
	})

	t.Run("partial override preserves set values", func(t *testing.T) {
		got := groupingOptions{shingleSize: 5}.withDefaults()
		require.Equal(t, 5, got.shingleSize)
		require.Equal(t, defaults.drainMaxDepth, got.drainMaxDepth)
		require.Equal(t, defaults.numHashes, got.numHashes)
	})

	t.Run("full override keeps everything", func(t *testing.T) {
		custom := groupingOptions{
			drainMaxDepth:     10,
			drainSimThreshold: 0.8,
			shingleSize:       7,
			numHashes:         256,
			lshThreshold:      0.9,
		}
		got := custom.withDefaults()
		require.Equal(t, custom, got)
	})
}

func TestNormalizeText(t *testing.T) {
	d := newDrain(drainDefaults())

	text := `error on host i-abc123: disk full
error on host i-def456: disk full
error on host i-ghi789: disk full`

	normalized := normalizeText(d, text)

	// After Drain, all three lines converge to the same template.
	lines := strings.Split(normalized, "\n")
	require.Len(t, lines, 3)
	assert.Contains(t, lines[2], "<*>", "variable tokens should become wildcards")
	assert.Contains(t, lines[2], "error on host")
	assert.Contains(t, lines[2], "disk full")
	assert.Equal(t, lines[1], lines[2])
}
