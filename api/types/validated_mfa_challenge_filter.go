/*
 * Copyright 2026 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

// ValidatedMFAChallengeFilter encodes filter params for validated MFA challenges.
type ValidatedMFAChallengeFilter struct {
	TargetCluster string
}

const validatedMFAChallengeFilterKeyTargetCluster = "target_cluster"

// IntoMap copies ValidatedMFAChallengeFilter values into a map.
func (f *ValidatedMFAChallengeFilter) IntoMap() map[string]string {
	m := make(map[string]string)

	if f.TargetCluster != "" {
		m[validatedMFAChallengeFilterKeyTargetCluster] = f.TargetCluster
	}

	return m
}

// FromMap copies values from a map into this ValidatedMFAChallengeFilter value.
func (f *ValidatedMFAChallengeFilter) FromMap(m map[string]string) {
	if val, ok := m[validatedMFAChallengeFilterKeyTargetCluster]; ok {
		f.TargetCluster = val
	}
}

// Match checks whether the target cluster matches the filter.
func (f *ValidatedMFAChallengeFilter) Match(targetCluster string) bool {
	// If no target cluster is specified in the filter, it matches nothing.
	if f.TargetCluster == "" {
		return false
	}

	return f.TargetCluster == targetCluster
}
