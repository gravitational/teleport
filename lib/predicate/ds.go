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

package predicate

func copyMap[K comparable, V any](m map[K]V) map[K]V {
	n := make(map[K]V)
	for k, v := range m {
		n[k] = v
	}
	return n
}

type set[T comparable] map[T]struct{}

func intersect[T comparable](a, b set[T]) set[T] {
	i := make(set[T])
	for k := range a {
		if _, ok := b[k]; ok {
			i[k] = struct{}{}
		}
	}

	return i
}

func union[T comparable](a, b set[T]) set[T] {
	i := make(set[T])
	for k := range a {
		i[k] = struct{}{}
	}

	for k := range b {
		i[k] = struct{}{}
	}

	return i
}

func newSet[T comparable](a []T) set[T] {
	i := make(set[T])
	for _, k := range a {
		i[k] = struct{}{}
	}

	return i
}
