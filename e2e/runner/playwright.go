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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type playwrightRunner struct {
	config *e2eConfig
}

var baseProjects = []string{"authenticated", "unauthenticated"}

func (p *playwrightRunner) startURL(inst *testInstance) string {
	if p.config.teleportURL != "" {
		return p.config.teleportURL + "/web"
	}

	return fmt.Sprintf("https://localhost:%d/web", inst.proxyPort)
}

func (p *playwrightRunner) run(ctx context.Context, mode runMode) error {
	switch mode {
	case modeTest:
		return p.test(ctx, false)
	case modeDebug:
		return p.test(ctx, true)
	case modeUI:
		return p.ui(ctx)
	case modeCodegen:
		return p.codegen(ctx)
	case modeBrowse:
		return p.openWebAuthenticated(ctx, "open")
	case modeBrowseConnect:
		return p.openConnectAuthenticated(ctx)
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
}

func (p *playwrightRunner) test(ctx context.Context, debug bool) error {
	// Keep blobs (and the attachments Playwright extracts into
	// blob-reports/resources during merge) under test-results, so they're part
	// of the uploaded test-results artifact and the --test-results trace flow
	// can resolve them. Outside test-results the merged report references a
	// transient sibling dir that's deleted next run and never uploaded.
	blobBaseDir := filepath.Join(p.config.e2eDir, "test-results", "blob-reports")
	if err := os.RemoveAll(blobBaseDir); err != nil {
		return fmt.Errorf("cleaning blob-reports directory: %w", err)
	}

	var extraArgs []string
	if p.config.updateSnapshots {
		extraArgs = append(extraArgs, "--update-snapshots")
	}

	var g errgroup.Group

	g.SetLimit(2)

	for _, inst := range p.config.instances {
		g.Go(func() error {
			return p.runInstance(ctx, inst, blobBaseDir, debug, extraArgs)
		})
	}

	if ci := p.config.connectInstance; ci != nil {
		g.Go(func() error {
			return p.runInstance(ctx, ci, blobBaseDir, debug, extraArgs)
		})
	}

	testErr := g.Wait()

	// Nothing ran, so there is no report to merge and the previous run's results are still on disk.
	// Reporting success here would read as green.
	if blobs, err := filepath.Glob(filepath.Join(blobBaseDir, "*.zip")); err == nil && len(blobs) == 0 {
		if testErr != nil {
			return testErr
		}

		return fmt.Errorf("no specs ran, every selected spec was restricted to other browsers")
	}

	slog.InfoContext(ctx, "merging blob reports")
	mergeArgs := []string{"exec", "playwright", "merge-reports", p.configFlag(), blobBaseDir}
	mergeEnv := os.Environ()
	mergeEnv = append(mergeEnv, "FORCE_COLOR=1")
	if err := p.pnpmQuiet(ctx, mergeArgs, mergeEnv); err != nil {
		slog.WarnContext(ctx, "failed to merge reports", "error", err)
		if testErr == nil {
			return err
		}
	} else {
		// Merge consumed the per-browser blobs; drop them so the test-results
		// artifact carries only the extracted resources/, not the (redundant,
		// larger) raw blob archives with every attachment embedded twice.
		if blobs, err := filepath.Glob(filepath.Join(blobBaseDir, "*.zip")); err == nil {
			for _, b := range blobs {
				_ = os.Remove(b)
			}
		}
	}

	return testErr
}

func (p *playwrightRunner) runInstance(ctx context.Context, inst *testInstance, blobBaseDir string, debug bool, extraArgs []string) error {
	hasConfigs := len(p.config.teleportConfigs) > 0
	selected := p.config.testFiles
	if hasConfigs {
		selected = p.config.defaultTestFiles
	}
	defaultFiles := p.filesForProject(inst, selected)

	// An empty list tells Playwright to run everything, so a selection that filtered down to nothing
	// has to skip the pass rather than widen it.
	runDefault := len(defaultFiles) > 0 || (!hasConfigs && len(selected) == 0)

	configFiles := make([][]string, len(p.config.teleportConfigs))
	anyConfigFiles := false
	for i, cfg := range p.config.teleportConfigs {
		configFiles[i] = p.filesForProject(inst, cfg.files)
		anyConfigFiles = anyConfigFiles || len(configFiles[i]) > 0
	}

	if !runDefault && !anyConfigFiles {
		inst.log.Info("no selected specs run against this browser, skipping")
		return nil
	}

	if err := inst.start(ctx); err != nil {
		return err
	}
	defer inst.stop()

	// Every pass runs even after an earlier one fails. Each re-initializes Teleport from the base
	// config, so they do not depend on each other, and stopping early would report a failing spec as
	// the only result while silently dropping the coverage of the passes behind it.
	var errs []error

	if runDefault {
		blobPath := filepath.Join(blobBaseDir, inst.browser+".zip")
		if err := p.runInstanceTests(ctx, inst, defaultFiles, blobPath, debug, extraArgs); err != nil {
			errs = append(errs, err)
		}
	}

	baseConfigPath := inst.teleportConfigPath
	for i, cfg := range p.config.teleportConfigs {
		files := configFiles[i]
		if len(files) == 0 {
			continue // no tests for this instance's project
		}
		// An interrupted run is the one case worth abandoning, since the remaining passes can only
		// fail on the dead context.
		if ctx.Err() != nil {
			break
		}
		if err := p.runTeleportConfig(ctx, inst, baseConfigPath, cfg, files, i, blobBaseDir, debug, extraArgs); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// filesForProject picks the connect specs or browser specs for a testInstance, dropping any spec
// that restricted itself to other browsers.
func (p *playwrightRunner) filesForProject(inst *testInstance, files []string) []string {
	if len(files) == 0 {
		return files
	}
	var out []string
	for _, f := range files {
		isConnect := strings.HasPrefix(filepath.ToSlash(f), "tests/connect/")
		if isConnect != (inst.browser == "connect") {
			continue
		}

		spec := lineNumberSuffixRe.ReplaceAllString(filepath.ToSlash(f), "")
		if allowed, ok := p.config.browserRestrictions[spec]; ok && !slices.Contains(allowed, inst.browser) {
			continue
		}

		out = append(out, f)
	}
	return out
}

// runInstanceTests runs Playwright against a running Teleport instance for the
// given test files. If `files` is empty, all tests are run.
func (p *playwrightRunner) runInstanceTests(ctx context.Context, inst *testInstance, files []string, blobPath string, debug bool, extraArgs []string) error {
	env, err := p.startEnv(inst)
	if err != nil {
		return fmt.Errorf("building env for %s: %w", inst.browser, err)
	}
	if debug {
		env = append(env, "PWDEBUG=1")
	}
	env = append(env, "PLAYWRIGHT_BLOB_OUTPUT_FILE="+blobPath)

	args := []string{"exec", "playwright", "test", p.configFlag()}
	args = append(args, extraArgs...)
	args = append(args, "--reporter=blob,"+filepath.Join(p.config.sharedDir, "scripts", "dot-progress-reporter.ts"))
	// Avoid `.playwright-artifacts-<n>` collisions across parallel pnpm runs.
	args = append(args, "--output=test-results/"+inst.browser)
	if inst.browser == "connect" {
		args = append(args, "--project=connect")
	} else {
		for _, proj := range baseProjects {
			args = append(args, "--project="+inst.browser+":"+proj)
		}
	}
	args = append(args, files...)

	if len(files) > 0 {
		inst.log.InfoContext(ctx, "running e2e tests", "files", files)
	} else {
		inst.log.InfoContext(ctx, "running e2e tests", "projects", baseProjects)
	}

	if err := p.pnpm(ctx, args, env); err != nil {
		return fmt.Errorf("playwright tests failed for %s: %w", inst.browser, err)
	}
	return nil
}

// runTeleportConfig re-inits the instance's Teleport with a test-declared config.
func (p *playwrightRunner) runTeleportConfig(ctx context.Context, inst *testInstance, baseConfigPath string, cfg uniqueTeleportConfig, files []string, idx int, blobBaseDir string, debug bool, extraArgs []string) error {
	inst.log.InfoContext(ctx, "re-initializing teleport with a test-declared config", "files", files)

	inst.teleport.stop()

	mergedPath := filepath.Join(p.config.e2eDir, "config", fmt.Sprintf("%s-teleport-config-%d.yaml", inst.browser, idx))
	if err := mergeTeleportConfig(baseConfigPath, mergedPath, p.config.e2eDir, cfg.raw); err != nil {
		return fmt.Errorf("merging teleport config for %s: %w", inst.browser, err)
	}
	inst.teleport.configPath = mergedPath
	inst.teleportConfigPath = mergedPath

	if err := inst.teleport.start(ctx); err != nil {
		return fmt.Errorf("re-initializing teleport for %s: %w", inst.browser, err)
	}
	if err := inst.teleport.waitReady(ctx, 30*time.Second); err != nil {
		return fmt.Errorf("teleport for %s not ready after config change: %w", inst.browser, err)
	}

	for _, node := range inst.nodes {
		if err := node.waitJoined(ctx, 30*time.Second); err != nil {
			return fmt.Errorf("node for %s failed to rejoin: %w", inst.browser, err)
		}
	}

	blobPath := filepath.Join(blobBaseDir, fmt.Sprintf("%s-config-%d.zip", inst.browser, idx))
	return p.runInstanceTests(ctx, inst, files, blobPath, debug, extraArgs)
}

func (p *playwrightRunner) ui(ctx context.Context) error {
	slog.InfoContext(ctx, "starting playwright in UI mode")

	if len(p.config.instances) == 0 {
		return fmt.Errorf("no test instances configured")
	}

	inst := p.config.instances[0]
	if err := inst.start(ctx); err != nil {
		return err
	}
	defer inst.stop()

	env, err := p.startEnv(inst)
	if err != nil {
		return err
	}

	args := []string{"exec", "playwright", "test", p.configFlag(), "--ui"}
	if len(p.config.testFiles) > 0 {
		args = append(args, p.config.testFiles...)
	}

	return p.pnpm(ctx, args, env)
}

func (p *playwrightRunner) codegen(ctx context.Context) error {
	return p.openWebAuthenticated(ctx, "codegen")
}

// openWebAuthenticated runs the global setup to generate auth state, then opens
// a Chromium browser with a virtual WebAuthn authenticator pre-loaded so that
// MFA challenges resolve automatically.
func (p *playwrightRunner) openWebAuthenticated(ctx context.Context, playwrightCmd string) error {
	if len(p.config.instances) == 0 {
		return fmt.Errorf("no test instances configured")
	}

	inst := p.config.instances[0]
	if err := inst.start(ctx); err != nil {
		return err
	}
	defer inst.stop()

	env, err := p.startEnv(inst)
	if err != nil {
		return err
	}

	slog.DebugContext(ctx, "running global setup to generate auth state")
	if err := p.pnpm(ctx, []string{"exec", "tsx", filepath.Join(p.config.sharedDir, "global-setup.ts")}, env); err != nil {
		return err
	}

	slog.InfoContext(ctx, "opening playwright with auth and WebAuthn", "command", playwrightCmd)

	return p.pnpm(ctx, []string{
		"exec", "tsx", filepath.Join(p.config.sharedDir, "scripts", "open-with-webauthn.ts"),
		playwrightCmd,
		p.startURL(inst),
	}, env)
}

func (p *playwrightRunner) openConnectAuthenticated(ctx context.Context) error {
	inst := p.config.connectInstance
	if inst == nil {
		return fmt.Errorf("connect instance not configured (run Connect specific tests or use --with-connect)")
	}

	if err := inst.start(ctx); err != nil {
		return err
	}
	defer inst.stop()

	env, err := p.startEnv(inst)
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "opening Teleport Connect (with auth)")

	return p.pnpm(ctx, []string{"exec", "tsx", filepath.Join(p.config.sharedDir, "scripts", "open-connect.ts")}, env)
}

// startEnv builds the environment variables that Playwright tests need,
// including START_URL, credentials, and tctl paths for invite URL generation.
func (p *playwrightRunner) startEnv(inst *testInstance) ([]string, error) {
	env := os.Environ()
	// Force color output since Playwright's TTY detection won't work
	// when stdout/stderr are wrapped by the rewrite writer.
	env = append(env, "FORCE_COLOR=1")
	if os.Getenv("START_URL") == "" {
		env = append(env, "START_URL="+p.startURL(inst))
	}

	env = append(env, "E2E_DIR="+p.config.e2eDir)

	if p.config.creds != nil {
		env = append(env, "E2E_USERS_FILE="+filepath.Join(p.config.e2eDir, ".auth", "user-credentials.json"))
	}

	env = append(env, "E2E_TCTL_BIN="+p.config.tctlBin)
	env = append(env, "E2E_TELEPORT_CONFIG="+inst.teleportConfigPath)
	env = append(env, "E2E_BROWSERS="+strings.Join(p.config.browsers, ","))
	env = append(env, "E2E_BROWSER="+inst.browser)

	env = append(env, "E2E_CONNECT_TSH_BIN="+p.config.connectTshBinPath)
	env = append(env, "E2E_CONNECT_APP_DIR="+p.config.connectAppDir)

	if p.config.skipEnhancedRecording {
		env = append(env, "E2E_SKIP_ENHANCED_RECORDING=1")
	}

	return env, nil
}

func (p *playwrightRunner) pnpmQuiet(ctx context.Context, args []string, env []string) error {
	cmd := exec.CommandContext(ctx, "pnpm", args...)
	cmd.Dir = p.config.sharedDir
	cmd.Env = env
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("command exited with code %d: %w", exitErr.ExitCode(), err)
		}

		return fmt.Errorf("failed to run command: %w", err)
	}

	return nil
}

func (p *playwrightRunner) pnpm(ctx context.Context, args []string, env []string) error {
	cmd := exec.CommandContext(ctx, "pnpm", args...)
	cmd.Dir = p.config.sharedDir
	cmd.Env = env

	stdout, stderr := p.outputWriters()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("command exited with code %d: %w", exitErr.ExitCode(), err)
		}

		return fmt.Errorf("failed to run command: %w", err)
	}

	return nil
}

// outputWriters returns stdout/stderr writers that rewrite Playwright
// output so paths are clickable and commands are runnable from the
// caller's working directory.
func (p *playwrightRunner) outputWriters() (io.Writer, io.Writer) {
	callerDir := os.Getenv("E2E_CALLER_DIR")

	// Always rewrite the show-report command; conditionally prefix paths.
	var pathPrefix string
	if callerDir != "" && callerDir != p.config.e2eDir {
		if rel, err := filepath.Rel(callerDir, p.config.e2eDir); err == nil && rel != "." {
			pathPrefix = rel + "/"
		}
	}

	var showReportCmd string
	if p.config.isCI {
		if pr := ciPRNumber(); pr > 0 {
			showReportCmd = ciReportCmd(pr)
		}
	}
	if showReportCmd == "" {
		showReportCmd = "pnpm show-report"
		if pathPrefix != "" {
			showReportCmd = fmt.Sprintf("(cd %s && pnpm show-report)", pathPrefix[:len(pathPrefix)-1])
		}
	}

	rewrite := func(p []byte) []byte {
		if pathPrefix != "" {
			p = bytes.ReplaceAll(p, []byte("test-results/"), []byte(pathPrefix+"test-results/"))
			p = bytes.ReplaceAll(p, []byte("tests/"), []byte(pathPrefix+"tests/"))
		}
		p = bytes.ReplaceAll(p, []byte("pnpm exec playwright show-report"), []byte(showReportCmd))
		return p
	}

	return rewriteWriter{os.Stdout, rewrite}, rewriteWriter{os.Stderr, rewrite}
}

type rewriteWriter struct {
	w       io.Writer
	rewrite func([]byte) []byte
}

func (rw rewriteWriter) Write(p []byte) (int, error) {
	if _, err := rw.w.Write(rw.rewrite(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (p *playwrightRunner) configFlag() string {
	return "--config=" + filepath.Join(p.config.e2eDir, "playwright.config.ts")
}
