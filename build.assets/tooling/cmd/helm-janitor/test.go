/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

func runTest(ctx context.Context, charts []Chart, updateSnapshots bool) error {
	if err := checkDependencies(helmBinName); err != nil {
		return trace.Wrap(err, "preflight checks")
	}

	for _, chart := range charts {
		if chart.IsLibrary {
			continue
		}
		if err := testHelm(ctx, chart, updateSnapshots); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Println(" ✅ All tests succeeded")
	return nil
}

func testHelm(ctx context.Context, chart Chart, updateSnapshots bool) error {
	fmt.Println("Running tests for chart:", chart.Name)
	args := []string{"unittest", "-3", "--with-subchart=false", chart.Path}
	if updateSnapshots {
		args = append(args, "-u")
	}
	// We log the test command so it's easier for a developer to copy it and re-run to target a failing test.
	fmt.Printf(" ▶️ %s %s\n", helmBinName, strings.Join(args, " "))
	stdout, stderr, err := run(ctx, helmBinName, args...)
	if err != nil {
		fmt.Printf(" ❌ Helm unit tests failed for chart %q", chart.Path)
		fmt.Println(string(stdout))
		fmt.Println(string(stderr))
		return trace.Wrap(err, "testing chart %q", chart.Path)
	}
	return nil
}
