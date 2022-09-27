/*
Copyright 2022 Gravitational, Inc.

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

package azure

// StringVal converts a pointer of a string or a string alias to a string value.
func StringVal[T ~string](s *T) string {
	if s != nil {
		return string(*s)
	}
	return ""
}

// ToMapOfString converts a map of pointers of strings or strings aliases to a
// map of strings.
func ToMapOfString[K comparable, V ~string](m map[K]*V) map[K]string {
	result := make(map[K]string, len(m))
	for k, v := range m {
		result[k] = StringVal(v)
	}
	return result
}

// ToMapOfPtrs converts a map of a generic type to a map of the pointers of
// that type.
func ToMapOfPtrs[K comparable, V any](m map[K]V) map[K]*V {
	result := make(map[K]*V, len(m))
	for k, v := range m {
		result[k] = &v
	}
	return result
}
