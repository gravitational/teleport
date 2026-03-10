/**
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
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

var sshNode = registerFixture("ssh-node", "start and connect a Teleport SSH node, runs in Docker")

type e2eFlags struct {
	noBuild          bool
	full             bool
	quiet            bool
	verbose          bool
	replaceCerts     bool
	updateSnapshots  bool
	licenseFile      string
	teleportBin      string
	tctlBin          string
	teleportURL      string
	teleportLogLevel string
	testFiles        []string
}

func parseFlags(repoRoot string) (*e2eFlags, runMode, error) {
	var f e2eFlags

	modes := modeSet{
		defaultMode: modeTest,
	}

	modes.register("ui", "open Playwright UI mode", modeUI)
	modes.register("codegen", "open Playwright codegen against running Teleport", modeCodegen)
	modes.register("debug", "run tests with Playwright inspector (PWDEBUG=1)", modeDebug)
	modes.register("browse", "open a signed-in browser for manual testing", modeBrowse)

	flag.BoolVar(&f.verbose, "v", false, "enable debug logging")
	flag.BoolVar(&f.noBuild, "no-build", false, "skip make binaries")                          // useful for running during development to avoid rebuilding Teleport every time
	flag.BoolVar(&f.quiet, "quiet", false, "redirect Teleport logs to file instead of stdout") // used in CI to avoid flooding logs with Teleport logs
	flag.BoolVar(&f.full, "full", false, "enable all optional fixtures")
	flag.BoolVar(&f.replaceCerts, "replace-certs", false, "generate new self-signed certificates")
	flag.BoolVar(&f.updateSnapshots, "update-snapshots", false, "update Playwright snapshot baselines")
	flag.StringVar(&f.teleportLogLevel, "teleport-log-level", "INFO", "Teleport log severity (DEBUG, INFO, WARN, ERROR)")
	flag.StringVar(&f.licenseFile, "license-file", "", "path to Teleport license file (required for Enterprise features)")

	stringFlagWithEnv(flag.CommandLine, &f.teleportBin, "teleport-bin", "TELEPORT_BIN",
		filepath.Join(repoRoot, "build", "teleport"), "override teleport binary path")
	stringFlagWithEnv(flag.CommandLine, &f.tctlBin, "tctl-bin", "TCTL_BIN",
		filepath.Join(repoRoot, "build", "tctl"), "override tctl binary path")
	stringFlagWithEnv(flag.CommandLine, &f.teleportURL, "teleport-url", "TELEPORT_URL", "",
		"override teleport URL for Playwright tests (e.g. https://localhost:3080), if set the runner will skip starting Teleport")

	bindFixtureFlags(flag.CommandLine)
	modes.bindFlags(flag.CommandLine)

	flag.Parse()

	if f.verbose {
		logLevel.Set(slog.LevelDebug)
	}

	if f.full {
		enableAllFixtures()
	}

	f.testFiles = flag.Args()

	mode, err := modes.resolve()

	return &f, mode, err
}

func stringFlagWithEnv(fs *flag.FlagSet, p *string, name, env, fallback, usage string) {
	if v := os.Getenv(env); v != "" {
		fallback = v
	}
	fs.StringVar(p, name, fallback, fmt.Sprintf("%s (env: %s)", usage, env))
}
