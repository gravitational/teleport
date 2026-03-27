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
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/e2e/runner/fixtures"
)

var logLevel = new(slog.LevelVar)

func main() {
	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      logLevel,
		TimeFormat: time.Kitchen,
	}))
	slog.SetDefault(logger)

	e2eDir, err := resolveE2EDir()
	if err != nil {
		slog.Error("failed to resolve e2e directory", "error", err)
		os.Exit(1)
	}

	resultsPath := filepath.Join(e2eDir, "test-results", "results.json")

	flags, mode, err := parseFlags(filepath.Dir(e2eDir))
	if err != nil {
		slog.Error("failed to parse flags", "error", err)
		os.Exit(1)
	}

	if mode == modeGitHubReport {
		if err := writeGitHubReport(resultsPath); err != nil {
			slog.Error("failed to write GitHub report", "error", err)
			os.Exit(1)
		}
		return
	}

	if mode == modeReport || mode == modeTestResults {
		repo := flags.reportRepo
		if repo == "" {
			repo = detectRepo(e2eDir)
		}

		cfg := &reportConfig{
			prNumber:  flags.reportPR,
			repo:      repo,
			sha:       flags.reportSHA,
			e2eDir:    e2eDir,
			tracePath: flags.tracePath,
		}

		var runErr error
		if mode == modeReport {
			runErr = runReport(cfg)
		} else {
			runErr = runTestResults(cfg)
		}

		if runErr != nil {
			slog.Error("runner exited with error", "error", runErr)
			os.Exit(1)
		}
		return
	}

	_ = os.Remove(resultsPath)

	isCI := os.Getenv("CI") != ""
	runErr := run(flags, mode, e2eDir, isCI)

	// Reset the terminal before exiting to ensure we aren't left with a messed up terminal if interrupted
	if tty, err := os.Open("/dev/tty"); err == nil {
		reset := exec.Command("stty", "sane")
		reset.Stdin = tty
		reset.Stdout = os.Stdout
		reset.Stderr = os.Stderr
		reset.Run()
		tty.Close()
	}

	if !flags.quiet {
		printTestSummary(e2eDir, resultsPath)
	}

	if runErr != nil {
		slog.Error("runner exited with error", "error", runErr)
		os.Exit(1)
	}
}

type e2eConfig struct {
	e2eFlags
	isCI     bool
	repoRoot string
	e2eDir   string
	certsDir string

	nodeConfigTemplate     string
	teleportConfigTemplate string
	stateTemplate          string

	// teleportBuildDir is the directory in which to run `make build/teleport`.
	// Empty when the teleport binary is overridden and no build is needed.
	teleportBuildDir string

	connectAppDir     string
	connectTshBinPath string

	creds *credentials

	instances       []*browserInstance
	connectInstance *browserInstance
}

// run sets up the test environment (ports, certs, credentials, teleport instance)
// and hands off to the Playwright runner in whatever mode was requested.
func run(flags *e2eFlags, mode runMode, e2eDir string, isCI bool) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	config := &e2eConfig{
		e2eFlags:               *flags,
		isCI:                   isCI,
		repoRoot:               filepath.Dir(e2eDir),
		e2eDir:                 e2eDir,
		certsDir:               filepath.Join(e2eDir, "certs"),
		stateTemplate:          filepath.Join(e2eDir, "config", "state.yaml.tmpl"),
		teleportConfigTemplate: filepath.Join(e2eDir, "config", "teleport.yaml.tmpl"),
		nodeConfigTemplate:     filepath.Join(e2eDir, "node", "node.yaml.tmpl"),
		connectAppDir:          filepath.Join(filepath.Dir(e2eDir), "web", "packages", "teleterm"),
		connectTshBinPath:      filepath.Join(filepath.Dir(e2eDir), "build", "tsh-e2e-webauthnmock"),
	}

	switch config.teleportBin {
	case filepath.Join(config.repoRoot, "build", "teleport"):
		config.teleportBuildDir = config.repoRoot
	case filepath.Join(config.repoRoot, "e", "build", "teleport"):
		config.teleportBuildDir = filepath.Join(config.repoRoot, "e")
	}

	if flags.browsers == nil {
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

	for _, browser := range config.browsers {
		inst := &browserInstance{
			browser: browser,
			log:     newBrowserLogger(browser),
			dataDir: filepath.Join(e2eDir, "data", browser),
		}
		config.instances = append(config.instances, inst)
	}

	if fixtures.Connect.Enabled {
		config.connectInstance = &browserInstance{
			browser: "connect",
			log:     newBrowserLogger("connect"),
			dataDir: filepath.Join(e2eDir, "data", "connect"),
		}
	}

	// Allocate all ports at once to minimize race windows.
	var portTargets []*int
	for _, inst := range config.instances {
		portTargets = append(portTargets, &inst.proxyPort, &inst.authPort)
		if fixtures.SSHNode.Enabled {
			portTargets = append(portTargets, &inst.sshPort)
		}
	}
	if ci := config.connectInstance; ci != nil {
		portTargets = append(portTargets, &ci.proxyPort, &ci.authPort)
	}

	if err := allocatePorts(portTargets...); err != nil {
		return fmt.Errorf("failed to allocate ports: %w", err)
	}

	for _, inst := range config.instances {
		inst.log.Debug("allocated ports", "proxy", inst.proxyPort, "auth", inst.authPort, "ssh", inst.sshPort)
	}
	if ci := config.connectInstance; ci != nil {
		ci.log.Debug("allocated ports", "proxy", ci.proxyPort, "auth", ci.authPort)
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

		if err := generateSelfSignedCert(config.certsDir, fixtures.SSHNode.Enabled); err != nil {
			return fmt.Errorf("failed to generate TLS certificates: %w", err)
		}
	}

	if config.teleportURL == "" {
		allInstances := config.instances
		if config.connectInstance != nil {
			allInstances = append(allInstances, config.connectInstance)
		}

		for _, inst := range allInstances {
			inst.log.Debug("cleaning data directory", "path", inst.dataDir)
			if err := os.RemoveAll(inst.dataDir); err != nil {
				return fmt.Errorf("failed to clean data directory for %s: %w", inst.browser, err)
			}
		}

		creds, err := generateUserCredentials()
		if err != nil {
			return fmt.Errorf("failed to generate credentials: %w", err)
		}
		config.creds = creds

		// One shared state file used by all instances.
		stateFile, err := generateStateFile(config.stateTemplate, creds)
		if err != nil {
			return fmt.Errorf("failed to generate state file: %w", err)
		}
		slog.Debug("generated bootstrap state", "path", stateFile)

		for _, inst := range allInstances {
			outPath := filepath.Join(e2eDir, "config", inst.browser+"-teleport.yaml")
			tcfg, err := generateTeleportConfig(config.teleportConfigTemplate, outPath, &TeleportConfig{
				DataDir:        inst.dataDir,
				AuthServerPort: inst.authPort,
				ProxyPort:      inst.proxyPort,
				KeyFilePath:    filepath.Join(config.certsDir, keyFileName),
				CertFilePath:   filepath.Join(config.certsDir, certFileName),
				LicenseFile:    config.licenseFile,
				LogLevel:       config.teleportLogLevel,
			})
			if err != nil {
				return fmt.Errorf("failed to generate Teleport config for %s: %w", inst.browser, err)
			}
			inst.teleportConfigPath = tcfg
			inst.log.Debug("generated Teleport config", "path", tcfg)
		}

		g, gctx := errgroup.WithContext(ctx)
		for _, inst := range allInstances {
			teleport := &teleportInstance{
				log:         inst.log,
				teleportBin: config.teleportBin,
				proxyPort:   inst.proxyPort,
				configPath:  inst.teleportConfigPath,
				stateFile:   stateFile,
			}
			if config.isCI || config.quiet {
				teleport.logFile = filepath.Join(config.e2eDir, "teleport-"+inst.browser+".log")
				inst.log.Debug("redirecting Teleport logs to file", "path", teleport.logFile)
			}
			inst.teleport = teleport

			g.Go(func() error {
				if err := teleport.start(ctx); err != nil {
					return fmt.Errorf("failed to start Teleport for %s: %w", inst.browser, err)
				}
				if err := teleport.waitReady(gctx, 30*time.Second); err != nil {
					return fmt.Errorf("Teleport for %s failed to become ready: %w", inst.browser, err)
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			for _, inst := range allInstances {
				if inst.teleport != nil {
					inst.teleport.stop()
				}
			}
			return err
		}

		defer func() {
			for _, inst := range allInstances {
				if inst.node != nil {
					inst.node.stop(context.Background())
				}
			}
			for _, inst := range allInstances {
				if inst.teleport != nil {
					inst.teleport.stop()
				}
			}
		}()

		if fixtures.SSHNode.Enabled {
			slog.Info("running with SSH node fixture enabled")

			nodeBin := config.teleportBin
			if runtime.GOOS != "linux" {
				buildDir := config.teleportBuildDir
				if buildDir == "" {
					buildDir = config.repoRoot
				}
				nodeBin = filepath.Join(buildDir, "build", "teleport-node")
			}

			dockerHost, err := resolveDockerHost()
			if err != nil {
				return fmt.Errorf("resolving docker host: %w", err)
			}

			for _, inst := range config.instances {
				outPath := filepath.Join(e2eDir, "node", inst.browser+"-node.yaml")
				nodeConfigPath, err := generateTeleportNodeConfig(config.nodeConfigTemplate, outPath, &TeleportNodeConfig{
					AuthServerHost: dockerHost,
					AuthServerPort: inst.authPort,
					SSHServerPort:  inst.sshPort,
				})
				if err != nil {
					return fmt.Errorf("failed to generate node config for %s: %w", inst.browser, err)
				}
				inst.log.Debug("generated Teleport node config", "path", nodeConfigPath)

				inst.node = &dockerNode{
					log:                inst.log,
					sshPort:            inst.sshPort,
					tctlBin:            config.tctlBin,
					teleportConfigPath: inst.teleportConfigPath,
					logFilePath:        filepath.Join(config.e2eDir, "docker-node-"+inst.browser+".log"),
					imageName:          nodeImage,
					containerName:      "teleport-e2e-node-" + inst.browser,
					configPath:         nodeConfigPath,
					teleportBin:        nodeBin,
				}
			}

			if err := pullImage(ctx, nodeImage); err != nil {
				return fmt.Errorf("pulling docker image: %w", err)
			}

			g, gctx := errgroup.WithContext(ctx)
			for _, inst := range config.instances {
				g.Go(func() error {
					if err := inst.node.start(gctx); err != nil {
						return fmt.Errorf("failed to start docker node for %s: %w", inst.browser, err)
					}
					if err := inst.node.waitJoined(gctx, 30*time.Second); err != nil {
						return fmt.Errorf("docker node for %s failed to join cluster: %w", inst.browser, err)
					}
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				return err
			}
		}
	}

	pw := &playwrightRunner{
		config: config,
	}

	return pw.run(ctx, mode)
}
