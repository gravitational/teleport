/*
Copyright 2015-2021 Gravitational, Inc.

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

package lib

// SliceToAnonymousMap converts string slice to map of anonymous structs
func SliceToAnonymousMap(keys []string) map[string]struct{} {
	var result = make(map[string]struct{})

	for _, v := range keys {
		result[v] = struct{}{}
	}

	return result
}
