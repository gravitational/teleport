/*
Copyright 2021 Gravitational, Inc.

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

package utils

// CopyByteSlice returns a copy of the byte slice.
func CopyByteSlice(in []byte) []byte {
	if in == nil {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}

// CopyByteSlices returns a copy of the byte slices.
func CopyByteSlices(in [][]byte) [][]byte {
	if in == nil {
		return nil
	}
	out := make([][]byte, len(in))
	for i := range in {
		out[i] = CopyByteSlice(in[i])
	}
	return out
}

// StringSlicesEqual returns true if string slices equal
func StringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SliceContainsStr returns 'true' if the slice contains the given value
func SliceContainsStr(slice []string, value string) bool {
	for i := range slice {
		if slice[i] == value {
			return true
		}
	}
	return false
}

// Deduplicate deduplicates list of strings
func Deduplicate(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, val := range in {
		if _, ok := seen[val]; !ok {
			out = append(out, val)
			seen[val] = true
		}
	}
	return out
}
