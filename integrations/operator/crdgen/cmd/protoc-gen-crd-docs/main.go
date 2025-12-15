//go:build !debug

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

	"github.com/gogo/protobuf/vanity/command"

	crdgen "github.com/gravitational/teleport/integrations/operator/crdgen"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func main() {
	slog.SetDefault(slog.New(logutils.NewSlogTextHandler(os.Stderr,
		logutils.SlogTextHandlerConfig{
			Level: slog.LevelDebug,
		},
	)))

	req := command.Read()
	if err := crdgen.HandleDocsRequest(req); err != nil {
		slog.ErrorContext(context.Background(), "Failed to generate schema", "error", err)
		os.Exit(-1)
	}
}
