// Copied verbatim from github.com/oapi-codegen/runtime v1.4.0 (jsonmerge.go),
// with one edit: the `github.com/apapsch/go-jsonmerge/v2` import is dropped
// because the `Merger` type lives in this same package now (see merge.go).
// Everything else — including the `JSONMerge` body — is byte-identical.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oapiruntime

import (
	"encoding/json"
)

// JsonMerge merges two JSON representation into a single object. `data` is the
// existing representation and `patch` is the new data to be merged in
//
// Deprecated: Use JSONMerge instead.
func JsonMerge(data, patch json.RawMessage) (json.RawMessage, error) {
	return JSONMerge(data, patch)
}

// JSONMerge merges two JSON representation into a single object. `data` is the
// existing representation and `patch` is the new data to be merged in
func JSONMerge(data, patch json.RawMessage) (json.RawMessage, error) {
	merger := Merger{
		CopyNonexistent: true,
	}
	if data == nil {
		data = []byte(`{}`)
	}
	if patch == nil {
		patch = []byte(`{}`)
	}
	merged, err := merger.MergeBytes(data, patch)
	if err != nil {
		return nil, err
	}
	return merged, nil
}
