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
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/build.assets/tooling/lib/helm"
)

func runReference(ctx context.Context, charts []Chart, check bool) error {
	for _, chart := range charts {
		if chart.ReferencePath == "" {
			// teleport-cluster's reference is not yet rendered
			continue
		}
		fmt.Printf("Rendering reference for chart %s\n", chart.Path)
		ref, err := helm.RenderReference(chart.Path)
		if err != nil {
			return trace.Wrap(err, "rendering chart reference for %q", chart.Path)
		}
		if check {
			existing, err := os.ReadFile(chart.ReferencePath)
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			if !bytes.Equal(existing, ref) {
				fmt.Printf(" ❌ Out-of-sync reference for chart %q.\n", chart.Name)
				fmt.Println("Please run `make -C example/chart render-chart-ref`")
				fmt.Println()
				fmt.Println(cmp.Diff(string(existing), string(ref)))
				return trace.CompareFailed("reference is out of date for chart %q", chart.Path)
			}
			continue
		}
		if err := os.WriteFile(chart.ReferencePath, ref, 0644); err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	if check {
		fmt.Println(" ✅ All references are up-to-date")
	} else {
		fmt.Println(" ✅ All references rendered")
	}
	return nil
}
