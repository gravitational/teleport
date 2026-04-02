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
	"strings"

	"golang.org/x/sync/errgroup"
)

type playwrightRunner struct {
	config        *e2eConfig
	extraProjects []string
}

func (p *playwrightRunner) startURL(inst *browserInstance) string {
	if p.config.teleportURL != "" {
		return p.config.teleportURL + "/web"
	}

	return fmt.Sprintf("https://localhost:%d/web", inst.proxyPort)
}

// callerRelativePaths returns paths relative to the caller's working directory
// so that logged paths are cmd+clickable in terminals.
func (p *playwrightRunner) callerRelativePaths(paths []string) []string {
	callerDir := os.Getenv("E2E_CALLER_DIR")
	if callerDir == "" {
		return paths
	}

	out := make([]string, len(paths))
	for i, path := range paths {
		abs := filepath.Join(p.config.e2eDir, path)
		if rel, err := filepath.Rel(callerDir, abs); err == nil {
			out[i] = rel
		} else {
			out[i] = path
		}
	}
	return out
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
	blobBaseDir := filepath.Join(p.config.e2eDir, "blob-reports")
	if err := os.RemoveAll(blobBaseDir); err != nil {
		return fmt.Errorf("cleaning blob-reports directory: %w", err)
	}

	baseProjects := make([]string, 0, 2+len(p.extraProjects))
	baseProjects = append(baseProjects, "authenticated", "unauthenticated")
	baseProjects = append(baseProjects, p.extraProjects...)

	var extraArgs []string
	if p.config.updateSnapshots {
		extraArgs = append(extraArgs, "--update-snapshots")
	}

	var g errgroup.Group
	for _, inst := range p.config.instances {
		g.Go(func() error {
			env, err := p.startEnv(inst)
			if err != nil {
				return fmt.Errorf("building env for %s: %w", inst.browser, err)
			}
			if debug {
				env = append(env, "PWDEBUG=1")
			}

			env = append(env, "PLAYWRIGHT_BLOB_OUTPUT_FILE="+filepath.Join(blobBaseDir, inst.browser+".zip"))

			args := []string{"exec", "playwright", "test"}
			args = append(args, extraArgs...)
			args = append(args, "--reporter=blob,./scripts/dot-progress-reporter.ts")

			for _, proj := range baseProjects {
				args = append(args, "--project="+inst.browser+":"+proj)
			}

			if len(p.config.testFiles) > 0 {
				args = append(args, p.config.testFiles...)
			}

			if len(p.config.testFiles) > 0 {
				inst.log.Info("running e2e tests", "files", p.config.testFiles)
			} else {
				inst.log.Info("running e2e tests", "projects", baseProjects)
			}

			if err := p.pnpm(ctx, args, env); err != nil {
				return fmt.Errorf("playwright tests failed for %s: %w", inst.browser, err)
			}
			return nil
		})
	}

	if ci := p.config.connectInstance; ci != nil {
		g.Go(func() error {
			env, err := p.startEnv(ci)
			if err != nil {
				return fmt.Errorf("building env for connect: %w", err)
			}
			if debug {
				env = append(env, "PWDEBUG=1")
			}

			env = append(env, "PLAYWRIGHT_BLOB_OUTPUT_FILE="+filepath.Join(blobBaseDir, "connect.zip"))

			args := []string{"exec", "playwright", "test"}
			args = append(args, extraArgs...)
			args = append(args, "--reporter=blob,./scripts/dot-progress-reporter.ts", "--project=connect")

			if len(p.config.testFiles) > 0 {
				args = append(args, p.config.testFiles...)
			}

			if len(p.config.testFiles) > 0 {
				ci.log.Info("running e2e tests", "files", p.config.testFiles)
			} else {
				ci.log.Info("running e2e tests", "projects", []string{"connect"})
			}

			if err := p.pnpm(ctx, args, env); err != nil {
				return fmt.Errorf("playwright tests failed for connect: %w", err)
			}
			return nil
		})
	}

	testErr := g.Wait()

	slog.Info("merging blob reports")
	mergeArgs := []string{"exec", "playwright", "merge-reports", "--config=playwright.config.ts", blobBaseDir}
	mergeEnv := os.Environ()
	mergeEnv = append(mergeEnv, "FORCE_COLOR=1")
	if err := p.pnpmQuiet(ctx, mergeArgs, mergeEnv); err != nil {
		slog.Warn("failed to merge reports", "error", err)
		if testErr == nil {
			return err
		}
	}

	return testErr
}

func (p *playwrightRunner) ui(ctx context.Context) error {
	slog.Info("starting playwright in UI mode")

	inst := p.config.instances[0]
	env, err := p.startEnv(inst)
	if err != nil {
		return err
	}

	return p.pnpm(ctx, []string{"exec", "playwright", "test", "--ui"}, env)
}

func (p *playwrightRunner) codegen(ctx context.Context) error {
	return p.openWebAuthenticated(ctx, "codegen")
}

// openWebAuthenticated runs the setup project to generate auth state, then opens
// a Chromium browser with a virtual WebAuthn authenticator pre-loaded so that
// MFA challenges resolve automatically.
func (p *playwrightRunner) openWebAuthenticated(ctx context.Context, playwrightCmd string) error {
	inst := p.config.instances[0]
	env, err := p.startEnv(inst)
	if err != nil {
		return err
	}

	slog.Debug("running setup project to generate auth state")
	if err := p.pnpm(ctx, []string{"exec", "playwright", "test", "--project=" + inst.browser + ":setup"}, env); err != nil {
		return err
	}

	slog.Info("opening playwright " + playwrightCmd + " (with auth and WebAuthn)")

	return p.pnpm(ctx, []string{
		"exec", "tsx", "scripts/open-with-webauthn.ts",
		playwrightCmd,
		p.startURL(inst),
	}, env)
}

func (p *playwrightRunner) openConnectAuthenticated(ctx context.Context) error {
	inst := p.config.connectInstance
	if inst == nil {
		return fmt.Errorf("connect instance not configured (use --with-connect)")
	}
	env, err := p.startEnv(inst)
	if err != nil {
		return err
	}

	slog.Info("opening Teleport Connect (with auth)")

	return p.pnpm(ctx, []string{"exec", "tsx", "scripts/open-connect.ts"}, env)
}

// startEnv builds the environment variables that Playwright tests need,
// including START_URL, credentials, and tctl paths for invite URL generation.
func (p *playwrightRunner) startEnv(inst *browserInstance) ([]string, error) {
	env := os.Environ()
	// Force color output since Playwright's TTY detection won't work
	// when stdout/stderr are wrapped by the rewrite writer.
	env = append(env, "FORCE_COLOR=1")
	if os.Getenv("START_URL") == "" {
		env = append(env, "START_URL="+p.startURL(inst))
	}

	if creds := p.config.creds; creds != nil {
		env = append(env,
			"E2E_PASSWORD="+creds.password,
			"E2E_WEBAUTHN_PRIVATE_KEY="+creds.privateKeyPKCS8Base64,
			"E2E_WEBAUTHN_CREDENTIAL_ID="+creds.credentialIDBase64,
		)
	}

	env = append(env, "E2E_TCTL_BIN="+p.config.tctlBin)
	env = append(env, "E2E_TELEPORT_CONFIG="+inst.teleportConfigPath)
	env = append(env, "E2E_BROWSERS="+strings.Join(p.config.browsers, ","))

	env = append(env, "E2E_CONNECT_TSH_BIN="+p.config.connectTshBinPath)
	env = append(env, "E2E_CONNECT_APP_DIR="+p.config.connectAppDir)

	return env, nil
}

func (p *playwrightRunner) pnpmQuiet(ctx context.Context, args []string, env []string) error {
	cmd := exec.CommandContext(ctx, "pnpm", args...)
	cmd.Dir = p.config.e2eDir
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
	cmd.Dir = p.config.e2eDir
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
