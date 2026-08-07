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
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/gravitational/teleport/build.assets/tooling/lib/helm"
)

func main() {
	var chartPath string
	var outputPath string
	flag.StringVar(&chartPath, "chart", "", "Path of the chart.")
	flag.StringVar(&outputPath, "output", "-", "Path of the generated markdown reference, '-' means stdout.")
	flag.Parse()

	ctx := context.Background()
	if chartPath == "" {
		slog.ErrorContext(ctx, "chart path must be specified")
		os.Exit(1)
	}

	reference, err := helm.RenderReference(chartPath)
	if err != nil {
		slog.ErrorContext(ctx, "failed parsing chart and rendering reference", "error", err)
		os.Exit(1)
	}

	if outputPath == "-" {
		fmt.Print(string(reference))
		os.Exit(0)
	}
	err = os.WriteFile(outputPath, reference, 0o644)
	if err != nil {
		slog.ErrorContext(ctx, "failed writing file", "error", err)
		os.Exit(1)
	}
	slog.InfoContext(ctx, "File successfully written", "file_path", outputPath)
}
