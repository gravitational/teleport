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

//go:generate go run ./cmd/gen-ts-fixtures

package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/lmittmann/tint"

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
	isCI      bool
	repoRoot  string
	e2eDir    string
	sharedDir string // shared resource dir (templates, scripts); defaults to e2eDir
	certsDir  string

	nodeConfigTemplate     string
	teleportConfigTemplate string
	stateTemplate          string

	// teleportBuildDir is the directory in which to run `make build/teleport`.
	// Empty when the teleport binary is overridden and no build is needed.
	teleportBuildDir string

	// nodeArch is the GOARCH the docker node is built and run for.
	nodeArch nodeArch

	// skipEnhancedRecording is set when the docker daemon's kernel cannot run Enhanced Session
	// Recording, so the tests that need it skip themselves instead of failing.
	skipEnhancedRecording bool

	// browserRestrictions maps a spec path to the browsers it declared it should run against.
	browserRestrictions map[string][]string

	connectAppDir     string
	connectTshBinPath string

	creds map[string]*credentials

	instances       []*testInstance
	connectInstance *testInstance

	// teleportConfigs are the unique custom Teleport configs declared by tests.
	teleportConfigs []uniqueTeleportConfig
	// defaultTestFiles are the selectors for the tests that run against the base config.
	defaultTestFiles []string
}

// run sets up the test environment (ports, certs, credentials, teleport instance)
// and hands off to the Playwright runner in whatever mode was requested.
func run(flags *e2eFlags, mode runMode, e2eDir string, isCI bool) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	sharedDir := e2eDir
	if v := os.Getenv("E2E_SHARED_DIR"); v != "" {
		sharedDir = v
	}

	repoRoot := filepath.Dir(e2eDir)

	config := &e2eConfig{
		e2eFlags:               *flags,
		isCI:                   isCI,
		repoRoot:               repoRoot,
		e2eDir:                 e2eDir,
		sharedDir:              sharedDir,
		certsDir:               filepath.Join(e2eDir, "certs"),
		stateTemplate:          filepath.Join(sharedDir, "config", "state.yaml.tmpl"),
		teleportConfigTemplate: filepath.Join(sharedDir, "config", "teleport.yaml.tmpl"),
		nodeConfigTemplate:     filepath.Join(sharedDir, "node", "node.yaml.tmpl"),
		connectAppDir:          filepath.Join(repoRoot, "web", "packages", "teleterm"),
		connectTshBinPath:      filepath.Join(repoRoot, "build", "tsh-e2e-webauthnmock"),
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
		inst := &testInstance{
			browser: browser,
			log:     newBrowserLogger(browser),
			e2eDir:  e2eDir,
			dataDir: filepath.Join(e2eDir, "data", browser),
			tctlBin: flags.tctlBin,
		}
		config.instances = append(config.instances, inst)
	}

	if fixtures.Connect.Enabled {
		config.connectInstance = &testInstance{
			browser: "connect",
			log:     newBrowserLogger("connect"),
			e2eDir:  e2eDir,
			dataDir: filepath.Join(e2eDir, "data", "connect"),
			tctlBin: flags.tctlBin,
		}
	}

	// Allocate all ports at once to minimize race windows.
	var portTargets []*int
	for _, inst := range config.instances {
		portTargets = append(portTargets, &inst.proxyPort, &inst.authPort)
		if variants := nodeVariants(); len(variants) > 0 {
			inst.sshPorts = make([]int, len(variants))
			for i := range inst.sshPorts {
				portTargets = append(portTargets, &inst.sshPorts[i])
			}
		}
	}
	if ci := config.connectInstance; ci != nil {
		portTargets = append(portTargets, &ci.proxyPort, &ci.authPort)
	}

	if err := allocatePorts(portTargets...); err != nil {
		return fmt.Errorf("failed to allocate ports: %w", err)
	}

	for _, inst := range config.instances {
		inst.log.Debug("allocated ports", "proxy", inst.proxyPort, "auth", inst.authPort, "ssh", inst.sshPorts)
	}
	if ci := config.connectInstance; ci != nil {
		ci.log.Debug("allocated ports", "proxy", ci.proxyPort, "auth", ci.authPort)
	}

	if sshNodeEnabled() {
		nodeArch, err := resolveNodeArch(ctx)
		if err != nil {
			return err
		}

		config.nodeArch = nodeArch
		slog.Debug("resolved docker node architecture", "arch", config.nodeArch)
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

		if err := generateSelfSignedCert(config.certsDir); err != nil {
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

		targets := flags.scanTargets
		if targets == nil {
			var err error
			targets, err = resolveTargetsWithHelpers(e2eDir, flags.testFiles)
			if err != nil {
				return fmt.Errorf("failed to resolve scan targets: %w", err)
			}
		}

		// Left alone when nothing resolved, since an empty list would mean "run everything" instead of
		// letting Playwright reject the selection.
		if expanded := expandedTestFiles(targets); len(config.testFiles) > 0 && len(expanded) > 0 {
			config.testFiles = expanded
		}

		scannedUsers, err := scanUsersFromTargets(targets)
		if err != nil {
			return fmt.Errorf("failed to scan users: %w", err)
		}
		slog.Debug("discovered bootstrap users", "count", len(scannedUsers))

		config.teleportConfigs, config.defaultTestFiles, err = scanTeleportConfigs(targets)
		if err != nil {
			return fmt.Errorf("failed to scan teleport configs: %w", err)
		}

		config.browserRestrictions, err = scanBrowserRestrictions(targets)
		if err != nil {
			return fmt.Errorf("failed to scan browser restrictions: %w", err)
		}
		if len(config.browserRestrictions) > 0 {
			slog.Debug("discovered browser restrictions", "specs", config.browserRestrictions)
		}

		bootstrap, err := buildBootstrapState(e2eDir, scannedUsers)
		if err != nil {
			return fmt.Errorf("failed to build bootstrap state: %w", err)
		}
		config.creds = bootstrap.creds

		userMappingPath := filepath.Join(e2eDir, ".auth", "user-mapping.json")
		if err := writeUserMapping(userMappingPath, bootstrap.userMapping); err != nil {
			return fmt.Errorf("failed to write user mapping: %w", err)
		}
		slog.Debug("wrote user mapping", "path", userMappingPath, "users", len(bootstrap.userMapping))

		credsPath := filepath.Join(e2eDir, ".auth", "user-credentials.json")
		if err := writeCredentialsFile(credsPath, bootstrap.creds); err != nil {
			return fmt.Errorf("failed to write user credentials: %w", err)
		}
		slog.Debug("wrote user credentials", "path", credsPath, "users", len(bootstrap.creds))

		recMappingPath := filepath.Join(e2eDir, ".auth", "recording-mapping.json")
		if err := writeRecordingMapping(recMappingPath, bootstrap.recordingMapping); err != nil {
			return fmt.Errorf("failed to write recording mapping: %w", err)
		}
		slog.Debug("wrote recording mapping", "path", recMappingPath, "users", len(bootstrap.recordingMapping))

		// One shared state file used by all instances.
		stateFile, err := generateStateFile(config.stateTemplate, bootstrap.state)
		if err != nil {
			return fmt.Errorf("failed to generate state file: %w", err)
		}
		slog.Debug("generated bootstrap state", "path", stateFile)

		for _, inst := range allInstances {
			outPath := filepath.Join(e2eDir, "config", inst.browser+"-teleport.yaml")
			tcfg, err := generateTeleportConfig(config.teleportConfigTemplate, outPath, &TeleportConfig{
				ClusterName:    clusterName,
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

		// Create teleport instances (started lazily by the playwright runner so that at most 2 run concurrently).
		for _, inst := range allInstances {
			teleport := &teleportInstance{
				log:             inst.log,
				teleportBin:     config.teleportBin,
				proxyPort:       inst.proxyPort,
				configPath:      inst.teleportConfigPath,
				stateFile:       stateFile,
				recordingOwners: bootstrap.recordingOwners,
			}

			if config.isCI || config.quiet {
				teleport.logFile = filepath.Join(config.e2eDir, "teleport-"+inst.browser+".log")
				inst.log.Debug("redirecting Teleport logs to file", "path", teleport.logFile)
			}

			inst.teleport = teleport
		}

		if sshNodeEnabled() {
			slog.Info("running with SSH node fixture enabled", "nodes", nodeVariantNames())

			nodeBin := config.teleportBin
			if needsNodeBuild(config) {
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

			sshPublicHost, err := nodePublicHost()
			if err != nil {
				return fmt.Errorf("resolving node public host: %w", err)
			}

			if err := pullImage(ctx, nodeImage, config.nodeArch); err != nil {
				return fmt.Errorf("pulling docker image: %w", err)
			}

			enhancedRecording := fixtures.SSHNodeBPF.Enabled
			if enhancedRecording {
				supported, probeErr := daemonSupportsBPF(ctx, nodeImage, config.nodeArch)
				if probeErr != nil {
					return fmt.Errorf("checking enhanced recording support: %w", probeErr)
				}

				if !supported {
					if config.isCI {
						return fmt.Errorf("the docker daemon's kernel cannot run enhanced session recording: " +
							"it needs BTF at /sys/kernel/btf/vmlinux and CONFIG_AUDIT for task_struct.sessionid")
					}

					slog.Warn("docker daemon's kernel cannot run enhanced session recording, skipping those tests",
						"needs", "BTF and CONFIG_AUDIT")

					enhancedRecording = false
					config.skipEnhancedRecording = true
				}
			}

			for _, inst := range config.instances {
				for i, variant := range nodeVariants() {
					// Only the BPF variant carries enhanced recording, and it loses it when the daemon's
					// kernel cannot run the programs.
					nodeEnhancedRecording := variant.enhancedRecording && enhancedRecording

					outPath := filepath.Join(e2eDir, "node", inst.browser+"-"+variant.name+".yaml")
					nodeConfigPath, err := generateTeleportNodeConfig(config.nodeConfigTemplate, outPath, &TeleportNodeConfig{
						NodeName:          variant.name,
						AuthServerHost:    dockerHost,
						AuthServerPort:    inst.authPort,
						SSHServerPort:     inst.sshPorts[i],
						SSHPublicHost:     sshPublicHost,
						EnhancedRecording: nodeEnhancedRecording,
					})
					if err != nil {
						return fmt.Errorf("failed to generate node config for %s: %w", inst.browser, err)
					}
					inst.log.Debug("generated Teleport node config", "path", nodeConfigPath)

					inst.nodes = append(inst.nodes, &dockerNode{
						log:                inst.log,
						nodeName:           variant.name,
						sshPort:            inst.sshPorts[i],
						tctlBin:            config.tctlBin,
						teleportConfigPath: inst.teleportConfigPath,
						logFilePath:        filepath.Join(config.e2eDir, variant.name+"-"+inst.browser+".log"),
						imageName:          nodeImage,
						containerName:      "teleport-e2e-" + variant.name + "-" + inst.browser,
						configPath:         nodeConfigPath,
						teleportBin:        nodeBin,
						arch:               config.nodeArch,
						enhancedRecording:  nodeEnhancedRecording,
					})
				}
			}
		}
	}

	pw := &playwrightRunner{
		config: config,
	}

	return pw.run(ctx, mode)
}

// applyResources applies all YAML resource files from e2eDir/config/resources/ via tctl create.
// If the directory does not exist, this is a no-op.
func applyResources(ctx context.Context, e2eDir, tctlBin, teleportConfig string) error {
	resourcesDir := filepath.Join(e2eDir, "config", "resources")
	files, err := filepath.Glob(filepath.Join(resourcesDir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("globbing resources: %w", err)
	}

	if len(files) == 0 {
		return nil
	}

	sort.Strings(files)

	for _, f := range files {
		slog.Info("applying resource", "file", filepath.Base(f))
		cmd := exec.CommandContext(ctx, tctlBin, "create", "-c", teleportConfig, "-f", f)

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tctl create %s: %w\n%s", filepath.Base(f), err, stderr.String())
		}
	}

	return nil
}
