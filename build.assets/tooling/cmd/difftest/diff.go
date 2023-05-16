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

package main

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gravitational/trace"
	"github.com/waigani/diffparser"
)

const (
	testFileSuffix = "_test.go"
)

// getChangedTestFilesFromDiff returns a list of changed files + segments from git diff string
func getChangedTestFilesFromDiff(diff string, exclude []string, include []string) ([]string, error) {
	d, err := diffparser.Parse(diff)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	files := make([]string, 0)

Outer:
	for _, f := range d.Files {
		if f.Mode == diffparser.DELETED {
			continue
		}

		if !strings.HasSuffix(f.NewName, testFileSuffix) {
			continue
		}

		for _, p := range exclude {
			if p == "" {
				continue
			}

			ok, err := doublestar.PathMatch(p, f.NewName)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if ok {
				continue Outer
			}
		}

		for _, p := range include {
			if p == "" {
				continue
			}

			ok, err := doublestar.PathMatch(p, f.NewName)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if !ok {
				continue Outer
			}
		}

		files = append(files, f.NewName)
	}

	return files, nil
}
