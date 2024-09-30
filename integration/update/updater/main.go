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
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/tool/common/update"
)

var (
	version  = "development"
	baseUrl  = "http://localhost"
	toolsDir = ""
)

func main() {
	updater := update.NewUpdater(
		toolsDir,
		version,
		update.WithBaseURL(baseUrl),
	)
	toolsVersion, reExec := updater.CheckLocal()
	if reExec {
		// Download the version of client tools required by the cluster. This
		// is required if the user passed in the TELEPORT_TOOLS_VERSION
		// explicitly.
		err := updater.Download(toolsVersion)
		if errors.Is(err, update.ErrCanceled) {
			os.Exit(0)
			return
		}
		if err != nil {
			log.Fatalf("Failed to download version (%v): %v", toolsVersion, err)
			return
		}

		// Re-execute client tools with the correct version of client tools.
		code, err := updater.Exec()
		if err != nil {
			log.Fatalf("Failed to re-exec client tool: %v", err)
		} else {
			os.Exit(code)
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("Teleport v%v git\n", version)
	}
}
