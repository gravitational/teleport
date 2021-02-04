/*
Copyright 2017 Gravitational, Inc.

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

// CopyStringsMap returns a copy of the strings map
func CopyStringsMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, val := range in {
		out[key] = val
	}
	return out
}

// ReplaceInSlice replaces element old with new and returns a new slice.
func ReplaceInSlice(s []string, old string, new string) []string {
	out := make([]string, 0, len(s))

	for _, x := range s {
		if x == old {
			out = append(out, new)
		} else {
			out = append(out, x)
		}
	}

	return out
}
