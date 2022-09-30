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

package fp

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func Filter[T any](slice []T, f func(T) bool) []T {
	var n []T
	for _, e := range slice {
		if f(e) {
			n = append(n, e)
		}
	}
	return n
}

func Map[T any, R any](slice []T, f func(T) R) []R {
	n := make([]R, len(slice))

	for i, e := range slice {
		n[i] = f(e)
	}

	return n
}

func Find[T any](slice []T, f func(T) bool) *T {
	for _, e := range slice {
		if f(e) {
			return &e
		}
	}
	return nil
}

func Some[T any](slice []T, f func(T) bool) bool {
	for _, e := range slice {
		if f(e) {
			return true
		}
	}
	return false
}
