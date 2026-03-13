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
	"slices"
	"strings"
)

var sshNode = registerFixture("ssh-node", "start and connect a Teleport SSH node, runs in Docker")
var connect = registerFixture("connect", "build Teleport Connect")

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

var validTeleportLogLevels = []string{"DEBUG", "INFO", "WARN", "ERROR"}

func parseFlags(repoRoot string) (*e2eFlags, runMode, error) {
	var f e2eFlags

	modes := modeSet{
		defaultMode: modeTest,
	}

	modes.register("ui", "open Playwright UI mode", modeUI)
	modes.register("codegen", "open Playwright codegen against running Teleport (not available for Connect)", modeCodegen)
	modes.register("debug", "run tests with Playwright inspector (PWDEBUG=1)", modeDebug)
	modes.register("browse", "open a signed-in browser for manual web testing", modeBrowse)
	modes.register("browse-connect", "open a signed-in Teleport Connect app for manual testing", modeBrowseConnect)

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

	f.teleportLogLevel = strings.ToUpper(f.teleportLogLevel)
	if !slices.Contains(validTeleportLogLevels, f.teleportLogLevel) {
		return nil, 0, fmt.Errorf("invalid --teleport-log-level %q, must be one of: %s", f.teleportLogLevel, strings.Join(validTeleportLogLevels, ", "))
	}

	e2eDir := filepath.Join(repoRoot, "e2e")

	var err error
	f.testFiles, err = normalizeTestFiles(e2eDir, flag.Args())
	if err != nil {
		return nil, 0, err
	}

	mode, err := modes.resolve()
	if err != nil {
		return nil, 0, err
	}

	// Auto-enable Connect if intent is explicit via mode or selected test paths.
	if mode == modeBrowseConnect {
		connect.enabled = true
	}
	for _, file := range f.testFiles {
		slashPath := filepath.ToSlash(file)
		if slashPath == "tests/connect" || strings.HasPrefix(slashPath, "tests/connect/") {
			connect.enabled = true
			break
		}
	}

	return &f, mode, nil
}

func normalizeTestFiles(e2eDir string, args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}

	callerDir := os.Getenv("E2E_CALLER_DIR")
	if callerDir == "" {
		var err error
		callerDir, err = os.Getwd()

		if err != nil {
			return nil, fmt.Errorf("getting current working directory: %w", err)
		}
	}

	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		abs := arg
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(callerDir, abs)
		}

		rel, err := filepath.Rel(e2eDir, abs)
		if err != nil {
			return nil, fmt.Errorf("making %q relative to e2e dir: %w", arg, err)
		}

		normalized = append(normalized, rel)
	}

	return normalized, nil
}

func stringFlagWithEnv(fs *flag.FlagSet, p *string, name, env, fallback, usage string) {
	if v := os.Getenv(env); v != "" {
		fallback = v
	}
	fs.StringVar(p, name, fallback, fmt.Sprintf("%s (env: %s)", usage, env))
}
