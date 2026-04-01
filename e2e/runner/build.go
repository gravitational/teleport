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
	"path/filepath"
	"runtime"

	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/e2e/runner/fixtures"
)

// build compiles teleport binaries and installs playwright dependencies in parallel.
func build(ctx context.Context, config *e2eConfig) error {
	buildTeleport := config.teleportBuildDir != "" &&
		shouldBuild(filepath.Join(config.teleportBuildDir, "build", "teleport"), config.noBuild)
	buildTctl := config.tctlBin == filepath.Join(config.repoRoot, "build", "tctl") &&
		shouldBuild(config.tctlBin, config.noBuild)

	nodeBuildDir := config.teleportBuildDir
	if nodeBuildDir == "" {
		nodeBuildDir = config.repoRoot
	}

	var buildNode bool
	if fixtures.SSHNode.Enabled && runtime.GOOS != "linux" {
		buildNode = shouldBuild(filepath.Join(nodeBuildDir, "build", "teleport-node"), config.noBuild)
	}

	buildConnect := fixtures.Connect.Enabled &&
		shouldBuild(filepath.Join(config.repoRoot, "web", "packages", "teleterm", "build", "app"), config.noBuild)
	buildConnectTsh := fixtures.Connect.Enabled &&
		shouldBuild(config.connectTshBinPath, config.noBuild)

	// Both the teleport build (through make build/teleport -> build-ui) and the Connect build need JS
	// deps installed. Running pnpm install concurrently from multiple goroutines would cause a race,
	// so we ensure JS deps are installed up front before starting concurrent work.
	needsJSDeps := buildTeleport || buildConnect || buildConnectTsh
	if needsJSDeps {
		slog.Info("ensuring JS dependencies are installed")
		if err := runMake(ctx, config.repoRoot, "ensure-js-deps"); err != nil {
			return err
		}
	}

	g, ctx := errgroup.WithContext(ctx)

	if buildTeleport {
		buildDir := config.teleportBuildDir
		g.Go(func() error {
			slog.Info("building teleport", "dir", buildDir)

			return runMake(ctx, buildDir, "build/teleport")
		})
	} else if config.teleportBuildDir == "" {
		slog.Debug("teleport binary overridden, skipping build", "path", config.teleportBin)
	} else if config.noBuild {
		slog.Debug("skipping teleport build (--no-build)", "path", config.teleportBin)
	}

	if buildTctl {
		g.Go(func() error {
			slog.Info("building tctl")

			return runMake(ctx, config.repoRoot, "build/tctl")
		})
	} else if config.tctlBin != filepath.Join(config.repoRoot, "build", "tctl") {
		slog.Debug("tctl binary overridden, skipping build", "path", config.tctlBin)
	} else if config.noBuild {
		slog.Debug("skipping tctl build (--no-build)", "path", config.tctlBin)
	}

	if buildNode {
		g.Go(func() error {
			slog.Info("cross-compiling teleport for linux (docker node)", "dir", nodeBuildDir)

			output := filepath.Join(nodeBuildDir, "build", "teleport-node")
			cmd := exec.CommandContext(ctx, "go", "build",
				"-o", output,
				"-buildvcs=false",
				"./tool/teleport",
			)
			cmd.Dir = nodeBuildDir
			env := append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=1")
			if os.Getenv("CC") == "" {
				env = append(env, "CC=x86_64-unknown-linux-gnu-gcc")
			}
			cmd.Env = env
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("cross-compiling teleport for docker node: %w", err)
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
			args := []string{"exec", "playwright", "install", "--no-shell"}
			args = append(args, config.browsers...)
			if err := runInDir(ctx, config.e2eDir, "pnpm", args...); err != nil {
				return fmt.Errorf("playwright install: %w", err)
			}

			return nil
		})
	}

	if fixtures.Connect.Enabled {
		if buildConnect {
			g.Go(func() error {
				slog.Info("building Teleport Connect")
				if err := runInDir(ctx, config.repoRoot, "pnpm", "--filter=@gravitational/teleterm", "build"); err != nil {
					return fmt.Errorf("pnpm --filter=@gravitational/teleterm build: %w", err)
				}

				return nil
			})
		} else if config.noBuild {
			slog.Debug("skipping Teleport Connect build (--no-build)")
		}

		if buildConnectTsh {
			g.Go(func() error {
				slog.Info("building tsh with webauthnmock tag for Teleport Connect e2e")
				if err := runInDir(ctx, config.repoRoot, "go", "build", "-tags", "webauthnmock", "-o", config.connectTshBinPath, "./tool/tsh"); err != nil {
					return fmt.Errorf("go build -tags webauthnmock ./tool/tsh: %w", err)
				}

				return nil
			})
		} else if config.noBuild {
			slog.Debug("skipping tsh-webauthnmock build (--no-build)", "path", config.connectTshBinPath)
		}
	}

	return g.Wait()
}

func runMake(ctx context.Context, dir string, targets ...string) error {
	if err := runInDir(ctx, dir, "make", targets...); err != nil {
		return fmt.Errorf("make %v: %w", targets, err)
	}

	return nil
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

func shouldBuild(path string, noBuild bool) bool {
	if !noBuild {
		return true
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			slog.Info("binary missing, rebuilding", "path", path)
		} else {
			slog.Warn("error checking binary, rebuilding just in case", "path", path, "error", err)
		}

		return true
	}

	return false
}
