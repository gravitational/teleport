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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShingle(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		n        int
		expected int // number of shingles
	}{
		{
			name:     "normal text",
			text:     "the quick brown fox jumps",
			n:        3,
			expected: 3, // [the quick brown], [quick brown fox], [brown fox jumps]
		},
		{
			name:     "fewer words than shingle size",
			text:     "hello world",
			n:        3,
			expected: 1, // falls back to whole text as one shingle
		},
		{
			name:     "exact shingle size",
			text:     "one two three",
			n:        3,
			expected: 1,
		},
		{
			name:     "empty text",
			text:     "",
			n:        3,
			expected: 1, // empty text hashes to one shingle
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shingle(tt.text, tt.n)
			assert.Len(t, result, tt.expected)
		})
	}
}

func TestMinHashSignature(t *testing.T) {
	set := shingle("the quick brown fox jumps over the lazy dog", 3)
	sig := minHashSignature(set, 128)
	assert.Len(t, sig, 128)

	// All values should be set (not MaxUint64) since set is non-empty.
	for i, v := range sig {
		assert.NotEqual(t, uint64(0xffffffffffffffff), v, "signature slot %d should be set", i)
	}
}

func TestMinHashSimilarTexts(t *testing.T) {
	// Two similar texts should have similar signatures.
	text1 := "error failed to install teleport on instance disk full on /dev/xvda1"
	text2 := "error failed to install teleport on instance disk full on /dev/xvdb2"

	sig1 := minHashSignature(shingle(text1, 3), 128)
	sig2 := minHashSignature(shingle(text2, 3), 128)

	// Count matching positions.
	matches := 0
	for i := range sig1 {
		if sig1[i] == sig2[i] {
			matches++
		}
	}
	similarity := float64(matches) / float64(len(sig1))
	assert.Greater(t, similarity, 0.5, "similar texts should have high MinHash similarity")
}

func TestMinHashDissimilarTexts(t *testing.T) {
	text1 := "error failed to install teleport disk full"
	text2 := "curl could not resolve host dns failure timeout"

	sig1 := minHashSignature(shingle(text1, 3), 128)
	sig2 := minHashSignature(shingle(text2, 3), 128)

	matches := 0
	for i := range sig1 {
		if sig1[i] == sig2[i] {
			matches++
		}
	}
	similarity := float64(matches) / float64(len(sig1))
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
	sigs := make([][]uint64, len(texts))
	for i, text := range texts {
		sigs[i] = minHashSignature(shingle(text, 3), 128)
	}

	pairs := lshCandidates(sigs, 16, 8)
	require.NotEmpty(t, pairs)

	// Check that (0,1) and (0,2) are candidates (similar).
	hasPair := func(a, b int) bool {
		for _, p := range pairs {
			if (p[0] == a && p[1] == b) || (p[0] == b && p[1] == a) {
				return true
			}
		}
		return false
	}
	assert.True(t, hasPair(0, 1), "identical texts should be candidates")
	assert.True(t, hasPair(3, 4), "identical texts should be candidates")
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

func TestClusterTextsEndToEnd(t *testing.T) {
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

	clusters := clusterTexts(texts, clusterDefaults())
	require.NotEmpty(t, clusters, "should produce clusters")

	// We expect 3 clusters (possibly more if some don't merge, but at least
	// the disk-full group should cluster together).
	t.Logf("Got %d clusters:", len(clusters))
	for _, c := range clusters {
		t.Logf("  Cluster %d: %d members", c.ID, c.Size)
		// Show first few lines of template.
		lines := strings.Split(c.Template, "\n")
		for i, line := range lines {
			if i >= 3 {
				t.Logf("    ...")
				break
			}
			t.Logf("    %s", line)
		}
	}

	// Verify the largest cluster has the disk-full errors.
	assert.GreaterOrEqual(t, clusters[0].Size, 8,
		"largest cluster should contain most disk-full errors")

	// Verify total members equals input size.
	total := 0
	for _, c := range clusters {
		total += c.Size
	}
	assert.Equal(t, len(texts), total, "all texts should be assigned to a cluster")
}

func TestClusterTextsEmpty(t *testing.T) {
	clusters := clusterTexts(nil, clusterDefaults())
	assert.Nil(t, clusters)

	clusters = clusterTexts([]string{}, clusterDefaults())
	assert.Nil(t, clusters)
}

func TestClusterTextsSingleItem(t *testing.T) {
	clusters := clusterTexts([]string{"single error message"}, clusterDefaults())
	require.Len(t, clusters, 1)
	assert.Equal(t, 1, clusters[0].Size)
	assert.Equal(t, []int{0}, clusters[0].Members)
}

func TestClusterTextsIdentical(t *testing.T) {
	texts := make([]string, 20)
	for i := range texts {
		texts[i] = "exact same error message on every host"
	}

	clusters := clusterTexts(texts, clusterDefaults())
	require.Len(t, clusters, 1)
	assert.Equal(t, 20, clusters[0].Size)
}

func TestClusterTextsRealisticSSMOutput(t *testing.T) {
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

	clusters := clusterTexts(texts, clusterDefaults())
	require.NotEmpty(t, clusters)

	t.Logf("Got %d clusters from 70 outputs:", len(clusters))
	for _, c := range clusters {
		t.Logf("  Cluster %d: %d members", c.ID, c.Size)
	}

	// Should produce 2 main clusters (disk-full and DNS).
	// Allow up to 4 total clusters for edge cases.
	assert.LessOrEqual(t, len(clusters), 4,
		"should produce a small number of clusters for 2 failure types")

	// The largest cluster should be the disk-full group.
	assert.GreaterOrEqual(t, clusters[0].Size, 40,
		"largest cluster should contain most disk-full errors")

	// Total should equal input size.
	total := 0
	for _, c := range clusters {
		total += c.Size
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

func TestNormalizeText(t *testing.T) {
	d := newDrain(drainDefaults())

	text := `error on host i-abc123: disk full
error on host i-def456: disk full
error on host i-ghi789: disk full`

	normalized := normalizeText(d, text)

	// After Drain, all three lines converge to the same template.
	// The template evolves as more lines are trained: by the second line,
	// the instance ID token is wildcarded.
	lines := strings.Split(normalized, "\n")
	require.Len(t, lines, 3)
	// After convergence (line 2+), the template should have wildcards.
	assert.Contains(t, lines[2], "<*>", "variable tokens should become wildcards")
	assert.Contains(t, lines[2], "error on host")
	assert.Contains(t, lines[2], "disk full")
	// All lines reference the same cluster, so they all return the
	// current state of that cluster's template.
	assert.Equal(t, lines[1], lines[2])
}
