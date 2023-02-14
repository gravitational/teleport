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

package pagination

import "github.com/gravitational/trace"

// IteratePages will iterate through pages of a paging function and call the given function to each element of the results.
// The map function can return false to short circuit and return from the function early.
func Iterate[T any](initialToken string, pageFn func(string) ([]T, string, error), mapFn func(T) (bool, error)) error {
	pageToken := initialToken
	for {
		var resources []T
		var err error
		resources, pageToken, err = pageFn(pageToken)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, resource := range resources {
			ok, err := mapFn(resource)
			if err != nil {
				return trace.Wrap(err)
			}

			if !ok {
				return nil
			}
		}

		if pageToken == "" {
			break
		}
	}

	return nil
}
