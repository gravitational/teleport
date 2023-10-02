/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

// GetTypes gets the types that the matcher can match.
func (m GCPMatcher) GetTypes() []string {
	return m.Types
}

// CopyWithTypes copies the matcher with new types.
func (m GCPMatcher) CopyWithTypes(t []string) Matcher {
	newMatcher := m
	newMatcher.Types = t
	return newMatcher
}

// GetLabels gets the matcher's labels.
func (m GCPMatcher) GetLabels() Labels {
	if len(m.Labels) != 0 {
		return m.Labels
	}
	// Check Tags as well for backwards compatibility.
	return m.Tags
}

// CheckAndSetDefaults that the matcher is correct and adds default values.
func (m GCPMatcher) CheckAndSetDefaults() error {
	// TODO(marco): implement
	return nil
}
