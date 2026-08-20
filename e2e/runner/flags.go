package main

import (
	"context"
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
	noResourceSetup  bool
	enterprise       bool
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
	scanTargets      []scanTarget // resolved scan targets, computed once during flag parsing
	reportPR         int
	reportRepo       string
	reportSHA        string
	tracePath        string
}

var validTeleportLogLevels = []string{"DEBUG", "INFO", "WARN", "ERROR"}

// inferEnterpriseFromArgs switches the run over when a named path lives in the enterprise suite, so
// a path tab-completed out of the tree runs without also having to pass --enterprise. It reads the
// raw arguments because the suite directory the rest of the run resolves against depends on it.
func (f *e2eFlags) inferEnterpriseFromArgs(repoRoot string, args []string) error {
	if f.enterprise {
		return nil
	}

	entDir := filepath.Join(repoRoot, "e", "e2e")
	for _, arg := range args {
		abs := arg
		if err := resolveAbsPaths(&abs); err != nil {
			return err
		}

		if rel, err := filepath.Rel(entDir, abs); err == nil && !strings.HasPrefix(rel, "..") {
			f.enterprise = true
			return nil
		}
	}

	return nil
}

// applyEnterpriseDefaults points an enterprise run at the enterprise build and a license, leaving
// anything the caller set explicitly alone. tctl is deliberately not switched: e/Makefile delegates
// it back to the OSS target, so the two builds are identical.
func (f *e2eFlags) applyEnterpriseDefaults(repoRoot string) {
	if !f.enterprise {
		return
	}

	if f.teleportBin == filepath.Join(repoRoot, "build", "teleport") {
		f.teleportBin = filepath.Join(repoRoot, "e", "build", "teleport")
	}

	if f.licenseFile == "" {
		f.licenseFile = filepath.Join(repoRoot, "e", "fixtures", "license-all-features.pem")
	}
}

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
	modes.register("github-report", "publish test results as GitHub annotations and a job summary (CI only)", modeGitHubReport)

	var testResultsPR int
	flag.IntVar(&f.reportPR, "report", 0, "download and open a Playwright report for a given PR number")
	flag.IntVar(&testResultsPR, "test-results", 0, "download test results and open a trace for a given PR number (pass trace path as argument)")

	flag.BoolVar(&f.verbose, "v", false, "enable debug logging")
	flag.BoolVar(&f.noBuild, "no-build", false, "skip make binaries") // useful for running during development to avoid rebuilding Teleport every time
	flag.BoolVar(&f.noResourceSetup, "no-resource-setup", false, "skip applying resources to Teleport instance in advance of tests")
	enterpriseDefault := os.Getenv("E2E_ENTERPRISE") != ""
	flag.BoolVar(&f.enterprise, "enterprise", enterpriseDefault, "run the enterprise tests (tests/**/e/) against the enterprise Teleport build")
	flag.BoolVar(&f.enterprise, "e", enterpriseDefault, "shorthand for --enterprise")
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
		if err := f.inferEnterpriseFromArgs(repoRoot, flag.Args()); err != nil {
			return nil, 0, err
		}

		dirs := newSuiteDirs(repoRoot, f.enterprise)

		f.testFiles, err = normalizeTestFiles(dirs.suite, flag.Args())
		if err != nil {
			return nil, 0, err
		}

		if len(f.testFiles) > 0 || mode != modeUI {
			targets, resolveErr := resolveTargetsWithHelpers(dirs, f.testFiles)
			if resolveErr != nil {
				slog.WarnContext(context.Background(), "scan: error resolving files", "error", resolveErr)
			} else {
				f.scanTargets = targets
				for _, fix := range scanFixturesFromTargets(targets) {
					fix.Enabled = true
				}
			}
		}
	}

	f.applyEnterpriseDefaults(repoRoot)

	if err := resolveAbsPaths(&f.teleportBin, &f.tctlBin, &f.licenseFile); err != nil {
		return nil, 0, err
	}

	// Auto-enable Connect if intent is explicit via mode or selected test paths.
	if mode == modeBrowseConnect {
		fixtures.Connect.Enabled = true
		f.browsers = []string{}
	}

	if enabled := fixtures.Enabled(); len(enabled) > 0 {
		slog.InfoContext(context.Background(), "enabled fixtures", "fixtures", enabled)
	}

	// If every specified test file targets connect, skip browser instances.
	if len(f.testFiles) > 0 {
		allConnect := true
		anyConnect := false

		for _, file := range f.testFiles {
			slashPath := filepath.ToSlash(file)
			if slashPath == "tests/connect" || strings.HasPrefix(slashPath, "tests/connect/") {
				anyConnect = true
			} else {
				allConnect = false
			}
		}

		if anyConnect && mode == modeUI {
			return nil, 0, fmt.Errorf("--ui is not supported for Connect tests (Connect runs in Electron, not a browser)")
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
