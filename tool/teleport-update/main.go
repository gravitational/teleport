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
	"github.com/gravitational/teleport/api/client/webclient"
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
	templateEnvVar    = "TELEPORT_URL_TEMPLATE"
	proxyServerEnvVar = "TELEPORT_PROXY"
	updateGroupEnvVar = "TELEPORT_UPDATE_GROUP"

	cdnURITemplate = "https://cdn.teleport.dev/teleport-v{{.Version}}-{{.OS}}-{{.Arch}}-bin.tar.gz"

	versionsDirName = "versions"
	updatesFileName = "updates.yaml"
	lockFileName    = ".lock"
)

var plog = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentUpdater)

func main() {
	if err := Run(os.Args[1:]); err != nil {
		libutils.FatalError(err)
	}
}

type cliConfig struct {
	Debug   bool
	DataDir string

	ProxyServer string
	Group       string
	Template    string

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

	err = validateCLIConfig(&ccfg)
	if err != nil {
		return trace.Wrap(err)
	}

	switch command {
	case enableCmd.FullCommand():
		err = doEnable(ctx, &ccfg)
	case disableCmd.FullCommand():
		err = doDisable(ctx, &ccfg)
	case updateCmd.FullCommand():
		err = doUpdate(ctx, &ccfg)
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
	updatesKind    = "agent_versions"
)

type UpdatesConfig struct {
	Version string      `yaml:"version"`
	Kind    string      `yaml:"kind"`
	Spec    UpdatesSpec `yaml:"spec"`
}

type UpdatesSpec struct {
	Proxy         string `yaml:"proxy"`
	Group         string `yaml:"group"`
	URITemplate   string `yaml:"uri_template"`
	Enabled       bool   `yaml:"enabled"`
	ActiveVersion string `yaml:"active_version"`
}

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

func doDisable(ctx context.Context, ccfg *cliConfig) error {
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

func doEnable(ctx context.Context, ccfg *cliConfig) error {
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
	if ccfg.ProxyServer != "" {
		cfg.Spec.Proxy = ccfg.ProxyServer
	}
	if ccfg.Group != "" {
		cfg.Spec.Group = ccfg.Group
	}
	if ccfg.Template != "" {
		cfg.Spec.URITemplate = ccfg.Template
	}
	cfg.Spec.Enabled = true
	if err := validateUpdatesSpec(&cfg.Spec); err != nil {
		return trace.Wrap(err)
	}
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return trace.Wrap(err)
	}
	addr, err := libutils.ParseAddr(cfg.Spec.Proxy)
	if err != nil {
		return trace.Errorf("failed to parse proxy server address: %w", err)
	}
	resp, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: addr.Addr,
		Timeout:   30 * time.Second,
		Group:     cfg.Spec.Group,
		Pool:      certPool,
	})
	if err != nil {
		return trace.Errorf("failed to request version from proxy: %w", err)
	}
	resp.AgentVersion = "16.1.0"
	if cfg.Spec.ActiveVersion != resp.AgentVersion {
		client, err := newClient(&downloadConfig{
			Pool: certPool,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		defer client.CloseIdleConnections()
		tv := TeleportVersion{
			VersionsDir:    filepath.Join(ccfg.DataDir, "versions"),
			DownloadClient: client,
		}
		template := cfg.Spec.URITemplate
		if template == "" {
			template = cdnURITemplate
		}
		err = tv.Create(ctx, template, resp.AgentVersion)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Spec.ActiveVersion = resp.AgentVersion
	}
	if err := writeUpdatesConfig(updatesYAML, cfg); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func doUpdate(ctx context.Context, ccfg *cliConfig) error {
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
		return trace.Errorf("updates disabled")
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return trace.Wrap(err)
	}
	addr, err := libutils.ParseAddr(cfg.Spec.Proxy)
	if err != nil {
		return trace.Errorf("failed to parse proxy server address: %w", err)
	}
	resp, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: addr.Addr,
		Timeout:   30 * time.Second,
		Group:     cfg.Spec.Group,
		Pool:      certPool,
	})
	if err != nil {
		return trace.Errorf("failed to request version from proxy: %w", err)
	}

	resp.AgentVersion = "16.1.0"
	if cfg.Spec.ActiveVersion != resp.AgentVersion {
		client, err := newClient(&downloadConfig{
			Pool: certPool,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		defer client.CloseIdleConnections()
		tv := TeleportVersion{
			VersionsDir:    filepath.Join(ccfg.DataDir, "versions"),
			DownloadClient: client,
		}
		template := cfg.Spec.URITemplate
		if template == "" {
			template = cdnURITemplate
		}
		err = tv.Create(ctx, template, resp.AgentVersion)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Spec.ActiveVersion = resp.AgentVersion
		if err := writeUpdatesConfig(updatesYAML, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
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

func validateCLIConfig(cfg *cliConfig) error {
	if cfg.DataDir == "" {
		cfg.DataDir = libdefaults.DataDir
	}
	return nil
}

func validateUpdatesSpec(spec *UpdatesSpec) error {
	if spec.Proxy == "" {
		return trace.Errorf("proxy URL must be specified with --proxy or present in updates.yaml")
	}
	return nil
}
