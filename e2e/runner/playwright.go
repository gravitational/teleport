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
	"regexp"
)

type playwrightRunner struct {
	config        *e2eConfig
	extraProjects []string
}

func (p *playwrightRunner) startURL() string {
	if p.config.teleportURL != "" {
		return p.config.teleportURL + "/web"
	}

	return fmt.Sprintf("https://localhost:%d/web", p.config.proxyPort)
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
		return p.browse(ctx)
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
}

func (p *playwrightRunner) test(ctx context.Context, debug bool) error {
	env, err := p.startEnv(ctx)
	if err != nil {
		return err
	}
	if debug {
		env = append(env, "PWDEBUG=1")
	}

	var extraArgs []string
	if p.config.updateSnapshots {
		extraArgs = append(extraArgs, "--update-snapshots")
	}

	if len(p.config.testFiles) > 0 {
		args := append([]string{"exec", "playwright", "test"}, extraArgs...)
		args = append(args, p.config.testFiles...)
		slog.Info("running e2e tests", "files", p.config.testFiles)
		return p.pnpm(ctx, args, env)
	}

	projects := append([]string{"authenticated", "unauthenticated"}, p.extraProjects...)
	args := append([]string{"exec", "playwright", "test"}, extraArgs...)
	for _, project := range projects {
		args = append(args, "--project="+project)
	}

	slog.Info("running e2e tests", "projects", projects)

	return p.pnpm(ctx, args, env)
}

func (p *playwrightRunner) ui(ctx context.Context) error {
	slog.Info("starting playwright in UI mode")

	env, err := p.startEnv(ctx)
	if err != nil {
		return err
	}

	return p.pnpm(ctx, []string{"exec", "playwright", "test", "--ui"}, env)
}

func (p *playwrightRunner) codegen(ctx context.Context) error {
	return p.openAuthenticated(ctx, "codegen")
}

func (p *playwrightRunner) browse(ctx context.Context) error {
	return p.openAuthenticated(ctx, "open")
}

// openAuthenticated runs the setup project to generate auth state, then opens
// a Chromium browser with a virtual WebAuthn authenticator pre-loaded so that
// MFA challenges resolve automatically.
func (p *playwrightRunner) openAuthenticated(ctx context.Context, playwrightCmd string) error {
	env, err := p.startEnv(ctx)
	if err != nil {
		return err
	}

	slog.Debug("running setup project to generate auth state")
	if err := p.pnpm(ctx, []string{"exec", "playwright", "test", "--project=setup"}, env); err != nil {
		return err
	}

	slog.Info("opening playwright " + playwrightCmd + " (with auth and WebAuthn)")

	return p.pnpm(ctx, []string{
		"exec", "tsx", "scripts/open-with-webauthn.ts",
		playwrightCmd,
		p.startURL(),
	}, env)
}

// generateInviteURL runs tctl to create a new user and extracts the invite link from its output.
func (p *playwrightRunner) generateInviteURL(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, p.config.tctlBin, "users", "add", "testuser",
		"--roles=access,editor", "-c", p.config.teleportConfigPath)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			slog.Error("tctl users add failed", "stderr", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("tctl users add: %w", err)
	}

	inviteURL := parseInviteURL(string(out))
	if inviteURL == "" {
		return "", fmt.Errorf("failed to parse invite URL from tctl output: %s", string(out))
	}

	slog.Debug("generated invite URL", "url", inviteURL)

	return inviteURL, nil
}

// startEnv builds the environment variables that Playwright tests need,
// including START_URL, credentials, and the invite URL for signup tests.
func (p *playwrightRunner) startEnv(ctx context.Context) ([]string, error) {
	env := os.Environ()
	// Force color output since Playwright's TTY detection won't work
	// when stdout/stderr are wrapped by the rewrite writer.
	env = append(env, "FORCE_COLOR=1")
	if os.Getenv("START_URL") == "" {
		env = append(env, "START_URL="+p.startURL())
	}

	if creds := p.config.creds; creds != nil {
		env = append(env,
			"E2E_PASSWORD="+creds.password,
			"E2E_WEBAUTHN_PRIVATE_KEY="+creds.privateKeyPKCS8Base64,
			"E2E_WEBAUTHN_CREDENTIAL_ID="+creds.credentialIDBase64,
		)
	}

	inviteURL, err := p.generateInviteURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("generating invite URL: %w", err)
	}
	env = append(env, "E2E_INVITE_URL="+inviteURL)

	return env, nil
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
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
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

	showReportCmd := "pnpm show-report"
	if pathPrefix != "" {
		showReportCmd = fmt.Sprintf("(cd %s && pnpm show-report)", pathPrefix[:len(pathPrefix)-1])
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

var inviteURLRe = regexp.MustCompile(`https?://\S+/web/invite/[0-9a-f]+`)

func parseInviteURL(output string) string {
	return inviteURLRe.FindString(output)
}
