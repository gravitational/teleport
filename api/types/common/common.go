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

package common

// IsValidLabelKey checks if the supplied string is a valid label key.
func IsValidLabelKey(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, b := range []byte(s) {
		if !isValidLabelKeyByte(b) {
			return false
		}
	}
	return true
}

// isValidLabelKeyByte checks if the byte is a valid character for a label key.
// Valid label keys can consist of alphanumeric characters, forward slashes,
// periods, underscores, colons, stars, and dashes. Some valid label examples:
//
// label
// teleport.dev/fine-grained-access
// teleport.dev/managed:internal_access
// all-objects*
func isValidLabelKeyByte(b byte) bool {
	switch b {
	case
		// Digits
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',

		// Lowercase letters
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
		'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',

		// Uppercase letters
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
		'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',

		// Allowed symbols
		'/', '.', '_', ':', '*', '-':
		return true
	default:
		return false
	}
}
