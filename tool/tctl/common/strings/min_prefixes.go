// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package strings

// FindMinPrefixes finds the smallest, non-conflicting prefix of vals,
// starting from a pre-determined min length.
//
// Useful to trim a sequence of hashes to unique prefixes.
//
// Returns the minimal prefixes, all with the same length, or the original slice
// if a min prefix can't be found.
func FindMinPrefixes(vals []string) []string {
	if len(vals) == 0 {
		return nil
	}

	minHashes := make([]string, len(vals))

	const startLen = 8
	for minLen := startLen; true; minLen++ {
		seenHashes := make(map[string]struct{})
		trimmed := false

		// Attempt to trim all entries to minLen.
		for i, h := range vals {
			if minLen < len(h) {
				minHashes[i] = h[:minLen]
				trimmed = true
			} else {
				minHashes[i] = h
			}
		}

		// If no hashes could be trimmed stop and return original slice.
		if !trimmed {
			break
		}

		// Look for a repeated hash. If there is none, return.
		collision := false
		for _, mh := range minHashes {
			if _, seen := seenHashes[mh]; seen {
				collision = true
				break
			}
			seenHashes[mh] = struct{}{}
		}
		if !collision {
			return minHashes
		}
	}

	return vals
}
