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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"golang.org/x/sync/errgroup"
)

// build compiles teleport binaries and installs playwright dependencies in parallel.
func build(ctx context.Context, config *e2eConfig) error {
	g, ctx := errgroup.WithContext(ctx)

	if !config.noBuild {
		g.Go(func() error {
			slog.Info("building teleport binaries")
			if err := runInDir(ctx, config.repoRoot, "make", "binaries"); err != nil {
				return fmt.Errorf("make binaries: %w", err)
			}

			return nil
		})
	}

	if !config.isCI {
		g.Go(func() error {
			slog.Info("installing e2e dependencies")
			if err := runInDir(ctx, config.e2eDir, "pnpm", "install"); err != nil {
				return fmt.Errorf("pnpm install: %w", err)
			}

			slog.Info("installing playwright browsers")
			if err := runInDir(ctx, config.e2eDir, "pnpm", "exec", "playwright", "install", "--with-deps", "chromium"); err != nil {
				return fmt.Errorf("playwright install: %w", err)
			}

			return nil
		})
	}

	if connect.enabled && !config.noBuild {
		g.Go(func() error {
			slog.Info("building Teleport Connect")
			if err := runInDir(ctx, config.repoRoot, "pnpm", "--filter=@gravitational/teleterm", "build"); err != nil {
				return fmt.Errorf("pnpm --filter=@gravitational/teleterm build: %w", err)
			}

			return nil
		})

		g.Go(func() error {
			slog.Info("building tsh with webauthnmock tag for Teleport Connect e2e")
			if err := runInDir(ctx, config.repoRoot, "go", "build", "-tags", "webauthnmock", "-o", config.connectTshBinPath, "./tool/tsh"); err != nil {
				return fmt.Errorf("go build -tags webauthnmock ./tool/tsh: %w", err)
			}

			return nil
		})
	}

	return g.Wait()
}

func runInDir(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("command exited with code %d: %w", exitErr.ExitCode(), err)
		}

		return fmt.Errorf("failed to run command: %w", err)
	}

	return nil
}
