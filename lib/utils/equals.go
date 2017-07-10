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

// StringMapsEqual returns true if two strings maps are equal
func StringMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if a[key] != b[key] {
			return false
		}
	}
	return true
}

// InterfaceMapsEqual returns true if two interface maps are equal.
func InterfaceMapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if a[key] != b[key] {
			return false
		}
	}
	return true
}

// StringMapSlicesEqual returns true if two maps of string slices are equal
func StringMapSlicesEqual(a, b map[string][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key := range a {
		if !StringSlicesEqual(a[key], b[key]) {
			return false
		}
	}
	return true
}
