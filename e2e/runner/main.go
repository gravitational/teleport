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
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/lmittmann/tint"
)

var logLevel = new(slog.LevelVar)

func main() {
	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      logLevel,
		TimeFormat: time.Kitchen,
	}))
	slog.SetDefault(logger)

	err := run()

	// Reset the terminal before exiting to ensure we aren't left with a messed up terminal if interrupted
	if tty, err := os.Open("/dev/tty"); err == nil {
		reset := exec.Command("stty", "sane")
		reset.Stdin = tty
		reset.Stdout = os.Stdout
		reset.Stderr = os.Stderr
		reset.Run()
		tty.Close()
	}

	if err != nil {
		slog.Error("runner exited with error", "error", err)
		os.Exit(1)
	}
}

type e2eConfig struct {
	e2eFlags
	isCI     bool
	dataDir  string
	repoRoot string
	e2eDir   string
	certsDir string

	nodeConfigTemplate     string
	teleportConfigTemplate string
	stateTemplate          string
	teleportConfigPath     string

	connectAppDir     string
	connectTshBinPath string

	creds *credentials

	proxyPort, authPort, sshPort int
}

// run sets up the test environment (ports, certs, credentials, teleport instance)
// and hands off to the Playwright runner in whatever mode was requested.
func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	e2eDir, err := resolveE2EDir()
	if err != nil {
		return fmt.Errorf("failed to resolve e2e directory: %w", err)
	}

	flags, mode, err := parseFlags(filepath.Dir(e2eDir))
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	config := &e2eConfig{
		e2eFlags:               *flags,
		isCI:                   os.Getenv("CI") != "",
		dataDir:                filepath.Join(e2eDir, "data"),
		repoRoot:               filepath.Dir(e2eDir),
		e2eDir:                 e2eDir,
		certsDir:               filepath.Join(e2eDir, "certs"),
		stateTemplate:          filepath.Join(e2eDir, "config", "state.yaml.tmpl"),
		teleportConfigTemplate: filepath.Join(e2eDir, "config", "teleport.yaml.tmpl"),
		nodeConfigTemplate:     filepath.Join(e2eDir, "node", "node.yaml.tmpl"),
		connectAppDir:          filepath.Join(filepath.Dir(e2eDir), "web", "packages", "teleterm"),
		connectTshBinPath:      filepath.Join(filepath.Dir(e2eDir), "build", "tsh-e2e-webauthnmock"),
	}

	if len(flags.browsers) == 0 {
		if config.isCI {
			config.browsers = []string{"chromium", "firefox", "webkit"}
		} else {
			config.browsers = []string{"chromium"}
		}
	}

	if (mode == modeBrowse || mode == modeCodegen) && len(config.browsers) > 1 {
		return fmt.Errorf("--%s only supports a single browser, got: %v", mode, config.browsers)
	}

	slog.Info("running playwright in mode", "mode", mode, "browsers", config.browsers)

	slog.Debug("using teleport binary", "path", flags.teleportBin)
	slog.Debug("using tctl binary", "path", flags.tctlBin)

	if config.isCI {
		slog.Debug("CI environment detected")
	}

	portTargets := []*int{&config.proxyPort, &config.authPort}

	if sshNode.enabled {
		portTargets = append(portTargets, &config.sshPort)
	}

	if err := allocatePorts(portTargets...); err != nil {
		return fmt.Errorf("failed to allocate ports: %w", err)
	}

	slog.Debug("running proxy", "port", config.proxyPort)
	slog.Debug("running auth server", "port", config.authPort)

	if sshNode.enabled {
		slog.Debug("running SSH service", "port", config.sshPort)
	}

	if err := build(ctx, config); err != nil {
		return fmt.Errorf("failed to build binaries: %w", err)
	}

	_, statErr := os.Stat(config.certsDir)
	switch {
	case statErr != nil && !os.IsNotExist(statErr):
		return fmt.Errorf("failed to check certs directory: %w", statErr)
	case os.IsNotExist(statErr) || config.replaceCerts:
		slog.Info("generating self-signed TLS certificates", "dir", config.certsDir)

		if err := generateSelfSignedCert(config.certsDir, sshNode.enabled); err != nil {
			return fmt.Errorf("failed to generate TLS certificates: %w", err)
		}
	}

	if config.teleportURL == "" {
		slog.Debug("cleaning data directory", "path", config.dataDir)
		if err := os.RemoveAll(config.dataDir); err != nil {
			return fmt.Errorf("failed to clean data directory: %w", err)
		}

		creds, err := generateUserCredentials()
		if err != nil {
			return fmt.Errorf("failed to generate credentials: %w", err)
		}
		config.creds = creds

		stateFile, err := generateStateFile(config.stateTemplate, creds)
		if err != nil {
			return fmt.Errorf("failed to generate state file: %w", err)
		}

		slog.Debug("generated bootstrap state", "path", stateFile)

		teleportConfig, err := generateTeleportConfig(config.teleportConfigTemplate, config)
		if err != nil {
			return fmt.Errorf("failed to generate Teleport config: %w", err)
		}

		config.teleportConfigPath = teleportConfig
		slog.Debug("generated Teleport config", "path", teleportConfig)

		teleport := &teleportInstance{
			config:     config,
			configPath: teleportConfig,
			stateFile:  stateFile,
		}
		if config.isCI || config.quiet {
			teleport.logFile = filepath.Join(config.e2eDir, "teleport.log")
			slog.Debug("redirecting Teleport logs to file", "path", teleport.logFile)
		}

		if err := teleport.start(ctx); err != nil {
			return fmt.Errorf("failed to start Teleport: %w", err)
		}
		defer teleport.stop()

		if err := teleport.waitReady(ctx, 30*time.Second); err != nil {
			return fmt.Errorf("failed to wait for Teleport to be ready: %w", err)
		}

		if sshNode.enabled {
			slog.Info("running with SSH node fixture enabled")

			nodeConfig, err := generateTeleportNodeConfig(config.nodeConfigTemplate, config)
			if err != nil {
				return fmt.Errorf("failed to generate Teleport node config: %w", err)
			}

			slog.Debug("generated Teleport node config", "path", nodeConfig)

			nodeBin := config.teleportBin
			if runtime.GOOS != "linux" {
				buildDir := config.repoRoot
				if config.teleportBin == filepath.Join(config.repoRoot, "e", "build", "teleport") {
					buildDir = filepath.Join(config.repoRoot, "e")
				}
				nodeBin = filepath.Join(buildDir, "build", "teleport-node")
			}

			node := &dockerNode{
				config:        config,
				imageName:     "debian:bookworm-slim",
				containerName: "teleport-e2e-node",
				configPath:    nodeConfig,
				teleportBin:   nodeBin,
			}

			if err := node.start(ctx); err != nil {
				return fmt.Errorf("failed to start docker node: %w", err)
			}
			// using context.Background() here because we want to ensure the node is stopped even if the main context is canceled.
			defer node.stop(context.Background())

			if err := node.waitJoined(ctx, 30*time.Second); err != nil {
				return fmt.Errorf("failed to wait for node to join cluster: %w", err)
			}
		}
	}

	var extraProjects []string
	// Project names from playwright.config.ts.
	if sshNode.enabled {
		extraProjects = append(extraProjects, "with-ssh-node")
	}
	if connect.enabled {
		extraProjects = append(extraProjects, "connect")
	}

	pw := &playwrightRunner{
		config:        config,
		extraProjects: extraProjects,
	}

	return pw.run(ctx, mode)
}
