//go:build debug

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"log/slog"
	"os"

	crdgen "github.com/gravitational/teleport/integrations/operator/crdgen"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func main() {
	slog.SetDefault(slog.New(logutils.NewSlogTextHandler(os.Stderr,
		logutils.SlogTextHandlerConfig{
			Level: slog.LevelDebug,
		},
	)))

	ctx := context.Background()
	inputPath := os.Getenv(crdgen.PluginInputPathEnvironment)
	if inputPath == "" {
		slog.ErrorContext(ctx, "When built with the 'debug' tag, the input path must be set through the TELEPORT_PROTOC_READ_FILE environment variable")
		os.Exit(-1)
	}
	slog.InfoContext(ctx, "This is a debug build, the protoc request is read from the file", "input_path", inputPath)

	req, err := crdgen.ReadRequestFromFile(inputPath)
	if err != nil {
		slog.ErrorContext(ctx, "error reading request from file", "error", err)
		os.Exit(-1)
	}

	if err := crdgen.HandleDocsRequest(req); err != nil {
		slog.ErrorContext(ctx, "Failed to generate docs", "error", err)
		os.Exit(-1)
	}
}
