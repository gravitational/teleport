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

import "regexp"

// LabelPattern is a regexp that describes a valid label key. In this case,
// valid label keys can consist of alphanumeric characters, forward slashes,
// periods, underscores, colons, stars, and dashes. Some valid label examples:
//
// label
// teleport.dev/fine-grained-access
// teleport.dev/managed:internal_access
// all-objects*
const LabelPattern = `^[a-zA-Z/.0-9_:*-]+$`

var validLabelKey = regexp.MustCompile(LabelPattern)

// IsValidLabelKey checks if the supplied string matches the
// label key regexp.
func IsValidLabelKey(s string) bool {
	return validLabelKey.MatchString(s)
}
