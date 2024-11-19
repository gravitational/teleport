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
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gravitational/teleport/lib/autoupdate/tools"
)

var (
	version  = "development"
	baseURL  = "http://localhost"
	toolsDir = ""
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx, _ = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)

	updater := tools.NewUpdater(
		toolsDir,
		version,
		tools.WithBaseURL(baseURL),
	)
	toolsVersion, reExec, err := updater.CheckLocal()
	if err != nil {
		log.Fatal(err)
	}
	if reExec {
		// Download and update the version of client tools required by the cluster.
		// This is required if the user passed in the TELEPORT_TOOLS_VERSION explicitly.
		err := updater.UpdateWithLock(ctx, toolsVersion)
		if errors.Is(err, context.Canceled) {
			os.Exit(0)
			return
		}
		if err != nil {
			log.Fatalf("failed to download version (%v): %v\n", toolsVersion, err)
			return
		}

		// Re-execute client tools with the correct version of client tools.
		code, err := updater.Exec()
		if err != nil {
			log.Fatalf("Failed to re-exec client tool: %v\n", err)
		} else {
			os.Exit(code)
		}
	}
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("Teleport v%v git\n", version)
	}
}
