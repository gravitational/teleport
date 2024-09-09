/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpproxy"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	apiutils "github.com/gravitational/teleport/api/utils"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	libutils "github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const appHelp = `Teleport Updater

The Teleport Updater updates the version a Teleport agent on a Linux server
that is being used as agent to provide connectivity to Teleport resources.

The Teleport Updater supports upgrade schedules and automated rollbacks. 

Find out more at https://goteleport.com/docs/updater`

const (
	// templateEnvVar allows the template for the Teleport tgz to be specified via env var.
	templateEnvVar = "TELEPORT_URL_TEMPLATE"
	// proxyServerEnvVar allows the proxy server address to be specified via env var.
	proxyServerEnvVar = "TELEPORT_PROXY"
	// updateGroupEnvVar allows the update group to be specified via env var.
	updateGroupEnvVar = "TELEPORT_UPDATE_GROUP"
)

const (
	// cdnURITemplate is the default template for the Teleport tgz download.
	cdnURITemplate = "https://cdn.teleport.dev/teleport-v{{.Version}}-{{.OS}}-{{.Arch}}-bin.tar.gz"
)

const (
	// versionsDirName specifies the name of the subdirectory inside of the Teleport data dir for storing Teleport versions.
	versionsDirName = "versions"
	// updatesFileName specifies the name of the file inside versionsDirName containing configuration for the teleport update
	updatesFileName = "updates.yaml"
	// lockFileName specifies the name of the file inside versionsDirName containing the flock lock preventing concurrent updater execution.
	lockFileName = ".lock"
)

var plog = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentUpdater)

func main() {
	if err := Run(os.Args[1:]); err != nil {
		libutils.FatalError(err)
	}
}

type cliConfig struct {
	// Debug logs enabled
	Debug bool
	// DataDir for Teleport (usually /var/lib/teleport)
	DataDir string

	// ProxyServer address, scheme and port optional
	ProxyServer string
	// Group identifier for updates (e.g., staging)
	Group string
	// Template for the Teleport tgz download URL
	Template string

	// LogFormat controls the format of logging. Can be either `json` or `text`.
	// By default, this is `text`.
	LogFormat string
}

func Run(args []string) error {
	var ccfg cliConfig
	ctx := context.Background()
	ctx, _ = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)

	app := libutils.InitCLIParser("teleport-updater", appHelp).Interspersed(false)
	app.Flag("debug", "Verbose logging to stdout.").Short('d').BoolVar(&ccfg.Debug)
	app.Flag("data-dir", "Directory to store teleport versions. Access to this directory should be limited.").StringVar(&ccfg.DataDir)
	app.Flag("log-format", "Controls the format of output logs. Can be `json` or `text`. Defaults to `text`.").Default(libutils.LogFormatText).
		EnumVar(&ccfg.LogFormat, libutils.LogFormatJSON, libutils.LogFormatText)

	app.HelpFlag.Short('h')

	versionCmd := app.Command("version", "Print the version of your teleport-updater binary.")

	enableCmd := app.Command("enable", "Enable agent auto-updates and perform initial updates.")
	enableCmd.Flag("proxy", "Address of the Teleport Proxy.").Short('p').Envar(proxyServerEnvVar).StringVar(&ccfg.ProxyServer)
	enableCmd.Flag("group", "Update group, for staged updates.").Short('g').Envar(updateGroupEnvVar).StringVar(&ccfg.Group)
	enableCmd.Flag("template", "Go template to override Teleport tgz download URL.").Short('t').Envar(templateEnvVar).StringVar(&ccfg.Template)

	disableCmd := app.Command("disable", "Disable agent auto-updates.")

	updateCmd := app.Command("update", "Update agent to the latest version, if a new version is available.")

	libutils.UpdateAppUsageTemplate(app, args)
	command, err := app.Parse(args)
	if err != nil {
		app.Usage(args)
		return trace.Wrap(err)
	}
	// Logging must be configured as early as possible to ensure all log
	// message are formatted correctly.
	if err := setupLogger(ccfg.Debug, ccfg.LogFormat); err != nil {
		return trace.Errorf("setting up logger")
	}

	if err := ccfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	switch command {
	case enableCmd.FullCommand():
		err = cmdEnable(ctx, &ccfg)
	case disableCmd.FullCommand():
		err = cmdDisable(ctx, &ccfg)
	case updateCmd.FullCommand():
		err = cmdUpdate(ctx, &ccfg)
	case versionCmd.FullCommand():
		modules.GetModules().PrintVersion()
	default:
		// This should only happen when there's a missing switch case above.
		err = trace.Errorf("command %q not configured", command)
	}

	return err
}

func setupLogger(debug bool, format string) error {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	switch format {
	case libutils.LogFormatJSON:
	case libutils.LogFormatText, "":
	default:
		return trace.Errorf("unsupported log format %q", format)
	}

	libutils.InitLogger(libutils.LoggingForDaemon, level, libutils.WithLogFormat(format))
	return nil
}

const (
	updatesVersion = "v1"
	updatesKind    = "updates"
)

// UpdatesConfig describes the updates.yaml file schema.
type UpdatesConfig struct {
	// Version of the configuration file
	Version string `yaml:"version"`
	// Kind of configuration file (always "updates")
	Kind string `yaml:"kind"`
	// Spec contains configuration.
	Spec UpdatesSpec `yaml:"spec"`
}

// UpdatesSpec describes the spec field in updates.yaml.
type UpdatesSpec struct {
	// Proxy address
	Proxy string `yaml:"proxy"`
	// Group update identifier
	Group string `yaml:"group"`
	// URLTemplate for the Teleport tgz download URL.
	URLTemplate string `yaml:"url_template"`
	// Enabled controls whether auto-updates are enabled.
	Enabled bool `yaml:"enabled"`
	// ActiveVersion is the currently active Teleport version.
	ActiveVersion string `yaml:"active_version"`
}

// readUpdatesConfig reads updates.yaml
func readUpdatesConfig(path string) (*UpdatesConfig, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &UpdatesConfig{
			Version: updatesVersion,
			Kind:    updatesKind,
		}, nil
	}
	if err != nil {
		return nil, trace.Errorf("failed to open updates.yaml: %w", err)
	}
	defer f.Close()
	var cfg UpdatesConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, trace.Errorf("failed to parse updates.yaml: %w", err)
	}
	if k := cfg.Kind; k != updatesKind {
		return nil, trace.Errorf("updates.yaml contains invalid kind %q", k)
	}
	if v := cfg.Version; v != updatesVersion {
		return nil, trace.Errorf("updates.yaml contains invalid version %q", v)
	}
	return &cfg, nil
}

// writeUpdatesConfig writes updates.yaml atomically, ensuring the file cannot be corrupted.
func writeUpdatesConfig(filename string, cfg *UpdatesConfig) error {
	opts := append([]renameio.Option{
		renameio.WithPermissions(0755),
		renameio.WithExistingPermissions(),
	})
	t, err := renameio.NewPendingFile(filename, opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	defer t.Cleanup()
	err = yaml.NewEncoder(t).Encode(cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(t.CloseAtomicallyReplace())
}

// cmdDisable disables updates.
func cmdDisable(ctx context.Context, ccfg *cliConfig) error {
	var (
		versionsDir = filepath.Join(ccfg.DataDir, versionsDirName)
		updatesYAML = filepath.Join(versionsDir, updatesFileName)
	)
	unlock, err := lock(filepath.Join(versionsDir, lockFileName))
	if err != nil {
		return trace.Wrap(err)
	}
	defer unlock()
	cfg, err := readUpdatesConfig(updatesYAML)
	if err != nil {
		return trace.Wrap(err)
	}
	if !cfg.Spec.Enabled {
		return nil
	}
	cfg.Spec.Enabled = false
	if err := writeUpdatesConfig(updatesYAML, cfg); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// cmdEnable enables updates and triggers an initial update.
func cmdEnable(ctx context.Context, ccfg *cliConfig) error {
	return trace.NotImplemented("TODO")
}

// cmdUpdate updates Teleport to the version specified by cluster reachable at the proxy address.
func cmdUpdate(ctx context.Context, ccfg *cliConfig) error {
	return trace.NotImplemented("TODO")
}

func lock(lockFile string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(lockFile), 0755); err != nil {
		return nil, trace.Wrap(err)
	}
	lf, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return nil, trace.Wrap(err)
	}

	return func() {
		if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_UN); err != nil {
			plog.Debug("Failed to unlock lock file", "file", lockFile, "error", err)
		}
		if err := lf.Close(); err != nil {
			plog.Debug("Failed to close lock file", "file", lockFile, "error", err)
		}
	}, nil
}

type downloadConfig struct {
	// Insecure turns off TLS certificate verification when enabled.
	Insecure bool
	// Pool defines the set of root CAs to use when verifying server
	// certificates.
	Pool *x509.CertPool
	// Timeout is a timeout for requests.
	Timeout time.Duration
}

func newClient(cfg *downloadConfig) (*http.Client, error) {
	rt := apiutils.NewHTTPRoundTripper(&http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
			RootCAs:            cfg.Pool,
		},
		Proxy: func(req *http.Request) (*url.URL, error) {
			return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
		},
		IdleConnTimeout: apidefaults.DefaultIOTimeout,
	}, nil)

	return &http.Client{
		Transport: tracehttp.NewTransport(rt),
		Timeout:   cfg.Timeout,
	}, nil
}

func (c *cliConfig) CheckAndSetDefaults() error {
	if c.DataDir == "" {
		c.DataDir = libdefaults.DataDir
	}
	return nil
}
