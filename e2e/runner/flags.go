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

	"github.com/gravitational/teleport/e2e/runner/fixtures"
)

var validBrowsers = []string{"chromium", "firefox", "webkit"}

type e2eFlags struct {
	noBuild          bool
	quiet            bool
	verbose          bool
	replaceCerts     bool
	updateSnapshots  bool
	licenseFile      string
	teleportBin      string
	tctlBin          string
	teleportURL      string
	teleportLogLevel string
	browsers         []string
	testFiles        []string
	reportPR         int
	reportRepo       string
	reportSHA        string
	tracePath        string
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
	modes.register("github-report", "publish test results as GitHub annotations, job summary, and PR comment (CI only)", modeGitHubReport)

	var testResultsPR int
	flag.IntVar(&f.reportPR, "report", 0, "download and open a Playwright report for a given PR number")
	flag.IntVar(&testResultsPR, "test-results", 0, "download test results and open a trace for a given PR number (pass trace path as argument)")

	flag.BoolVar(&f.verbose, "v", false, "enable debug logging")
	flag.BoolVar(&f.noBuild, "no-build", false, "skip make binaries")                          // useful for running during development to avoid rebuilding Teleport every time
	flag.BoolVar(&f.quiet, "quiet", false, "redirect Teleport logs to file instead of stdout") // used in CI to avoid flooding logs with Teleport logs
	flag.BoolVar(&f.replaceCerts, "replace-certs", false, "generate new self-signed certificates")
	flag.BoolVar(&f.updateSnapshots, "update-snapshots", false, "update Playwright snapshot baselines")
	flag.StringVar(&f.teleportLogLevel, "teleport-log-level", "INFO", "Teleport log severity (DEBUG, INFO, WARN, ERROR)")
	flag.StringVar(&f.licenseFile, "license-file", "", "path to Teleport license file (required for Enterprise features)")

	stringArrayFlag(flag.CommandLine, &f.browsers, "browsers", "comma-separated browsers to test: chromium, firefox, webkit (default: chromium locally, all in CI)")

	stringFlagWithEnv(flag.CommandLine, &f.teleportBin, "teleport-bin", "TELEPORT_BIN",
		filepath.Join(repoRoot, "build", "teleport"), "override teleport binary path")
	stringFlagWithEnv(flag.CommandLine, &f.tctlBin, "tctl-bin", "TCTL_BIN",
		filepath.Join(repoRoot, "build", "tctl"), "override tctl binary path")
	stringFlagWithEnv(flag.CommandLine, &f.teleportURL, "teleport-url", "TELEPORT_URL", "",
		"override teleport URL for Playwright tests (e.g. https://localhost:3080), if set the runner will skip starting Teleport")

	flag.StringVar(&f.reportRepo, "repo", "", "GitHub repo name (e.g. teleport.e), auto-detected if omitted")
	flag.StringVar(&f.reportSHA, "sha", "", "commit SHA to download artifacts for (overrides PR head SHA)")

	fixtures.BindFlags(flag.CommandLine)
	modes.bindFlags(flag.CommandLine)

	flag.Parse()

	if err := resolveAbsPaths(&f.teleportBin, &f.tctlBin); err != nil {
		return nil, 0, err
	}

	if f.verbose {
		logLevel.Set(slog.LevelDebug)
	}

	f.teleportLogLevel = strings.ToUpper(f.teleportLogLevel)
	if !slices.Contains(validTeleportLogLevels, f.teleportLogLevel) {
		return nil, 0, fmt.Errorf("invalid --teleport-log-level %q, must be one of: %s", f.teleportLogLevel, strings.Join(validTeleportLogLevels, ", "))
	}

	for _, b := range f.browsers {
		if !slices.Contains(validBrowsers, b) {
			return nil, 0, fmt.Errorf("invalid browser %q, must be one of: %s", b, strings.Join(validBrowsers, ", "))
		}
	}

	mode, err := modes.resolve()
	if err != nil {
		return nil, 0, err
	}

	switch {
	case f.reportPR > 0 && testResultsPR > 0:
		return nil, 0, fmt.Errorf("--report and --test-results are mutually exclusive")
	case f.reportPR > 0:
		if mode != modeTest {
			return nil, 0, fmt.Errorf("--report and --%s are mutually exclusive", mode)
		}
		mode = modeReport
	case testResultsPR > 0:
		if mode != modeTest {
			return nil, 0, fmt.Errorf("--test-results and --%s are mutually exclusive", mode)
		}
		mode = modeTestResults
		f.reportPR = testResultsPR

		args := flag.Args()
		if len(args) < 1 {
			return nil, 0, fmt.Errorf("--test-results requires a trace path as an argument")
		}
		f.tracePath = args[0]
	}

	isTestRun := mode == modeTest || mode == modeUI || mode == modeDebug
	if isTestRun {
		e2eDir := filepath.Join(repoRoot, "e2e")

		f.testFiles, err = normalizeTestFiles(e2eDir, flag.Args())
		if err != nil {
			return nil, 0, err
		}

		detected := scanFixtures(e2eDir, f.testFiles)
		if len(detected) == 0 {
			slog.Debug("fixture scan found no fixture declarations")
		} else {
			names := make([]string, len(detected))
			for i, fix := range detected {
				fix.Enabled = true
				names[i] = fix.Name
			}
			slog.Info("enabled fixtures", "fixtures", names)
		}
	}

	// Auto-enable Connect if intent is explicit via mode or selected test paths.
	if mode == modeBrowseConnect {
		fixtures.Connect.Enabled = true
		f.browsers = []string{}
	}

	// If every specified test file targets connect, skip browser instances.
	if len(f.testFiles) > 0 {
		allConnect := true

		for _, file := range f.testFiles {
			slashPath := filepath.ToSlash(file)
			if slashPath != "tests/connect" && !strings.HasPrefix(slashPath, "tests/connect/") {
				allConnect = false
				break
			}
		}

		if allConnect {
			f.browsers = []string{}
		}
	}

	return &f, mode, nil
}

func normalizeTestFiles(e2eDir string, args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		abs := arg
		if err := resolveAbsPaths(&abs); err != nil {
			return nil, err
		}

		rel, err := filepath.Rel(e2eDir, abs)
		if err != nil {
			return nil, fmt.Errorf("making %q relative to e2e dir: %w", arg, err)
		}

		normalized = append(normalized, rel)
	}

	return normalized, nil
}

func resolveAbsPaths(paths ...*string) error {
	callerDir := os.Getenv("E2E_CALLER_DIR")
	if callerDir == "" {
		var err error
		callerDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current working directory: %w", err)
		}
	}

	for _, p := range paths {
		if *p != "" && !filepath.IsAbs(*p) {
			*p = filepath.Join(callerDir, *p)
		}
	}

	return nil
}

func stringArrayFlag(fs *flag.FlagSet, p *[]string, name, usage string) {
	fs.Func(name, usage, func(s string) error {
		for _, v := range strings.Split(s, ",") {
			if v = strings.TrimSpace(v); v != "" {
				*p = append(*p, v)
			}
		}
		return nil
	})
}

func stringFlagWithEnv(fs *flag.FlagSet, p *string, name, env, fallback, usage string) {
	if v := os.Getenv(env); v != "" {
		fallback = v
	}
	fs.StringVar(p, name, fallback, fmt.Sprintf("%s (env: %s)", usage, env))
}
