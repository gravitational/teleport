/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
