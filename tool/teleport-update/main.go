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
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"text/template"
	"time"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/sys/unix"
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
)

const cdnURITemplate = "https://cdn.teleport.dev/teleport-v{{.Version}}-{{.OS}}-{{.Arch}}-bin.tar.gz"

var plog = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentUpdater)

func main() {
	if err := Run(os.Args[1:]); err != nil {
		libutils.FatalError(err)
	}
}

type cliConfig struct {
	Debug bool

	ProxyServer string
	Group       string
	Template    string

	// LogFormat controls the format of logging. Can be either `json` or `text`.
	// By default, this is `text`.
	LogFormat string
}

func Run(args []string) error {
	var cf cliConfig
	ctx := context.Background()

	app := libutils.InitCLIParser("teleport-updater", appHelp).Interspersed(false)
	app.Flag("debug", "Verbose logging to stdout.").Short('d').BoolVar(&cf.Debug)
	app.HelpFlag.Short('h')

	versionCmd := app.Command("version", "Print the version of your teleport-updater binary.")

	enableCmd := app.Command("enable", "Enable agent auto-updates and perform initial updates.")
	enableCmd.Flag("proxy", "Address of the Teleport Proxy.").Short('p').Envar(proxyServerEnvVar).StringVar(&cf.ProxyServer)
	enableCmd.Flag("group", "Update group, for staged updates.").Short('g').Envar(updateGroupEnvVar).StringVar(&cf.Group)
	enableCmd.Flag("template", "Go template to override Teleport tgz download URL.").Short('t').Envar(templateEnvVar).StringVar(&cf.Template)

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
	if err := setupLogger(cf.Debug, cf.LogFormat); err != nil {
		return trace.Errorf("setting up logger")
	}

	err = validate(&cf)
	if err != nil {
		return trace.Wrap(err)
	}

	switch command {
	case enableCmd.FullCommand():
		err = doEnable(ctx, &cf)
	case disableCmd.FullCommand():
		err = doDisable()
	case updateCmd.FullCommand():
		err = doUpdate(ctx)
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

// TODO: should be configurable?
var (
	versionsDir = filepath.Join(libdefaults.DataDir, "versions")
	updatesYAML = filepath.Join(versionsDir, "updates.yaml")
)

const (
	updatesVersion = "v1"
	updatesKind    = "agent_versions"
)

type UpdatesConfig struct {
	Version string `yaml:"version"`
	Kind    string `yaml:"kind"`
	Spec    struct {
		Proxy         string `yaml:"proxy"`
		Group         string `yaml:"group"`
		URITemplate   string `yaml:"uri_template"`
		Enabled       bool   `yaml:"enabled"`
		ActiveVersion string `yaml:"active_version"`
	} `yaml:"spec"`
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

func doDisable() error {
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

func doEnable(ctx context.Context, cf *cliConfig) error {
	unlock, err := lock()
	if err != nil {
		return trace.Wrap(err)
	}
	defer unlock()

	cfg, err := readUpdatesConfig(updatesYAML)
	if err != nil {
		return trace.Wrap(err)
	}
	if cf.ProxyServer != "" {
		cfg.Spec.Proxy = cf.ProxyServer
	}
	if cf.Group != "" {
		cfg.Spec.Group = cf.Group
	}
	if cf.Template != "" {
		cfg.Spec.URITemplate = cf.Template
	}
	cfg.Spec.Enabled = true

	addr, err := libutils.ParseAddr(cfg.Spec.Proxy)
	if err != nil {
		return trace.Errorf("failed to parse proxy server address: %w", err)
	}
	resp, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: addr.Addr,
		Timeout:   30 * time.Second,
	})
	if err != nil {
		return trace.Errorf("failed to request version from proxy: %w", err)
	}
	if cfg.Spec.ActiveVersion != resp.AgentVersion {
		err = createVersion(ctx, cfg.Spec.URITemplate, resp.AgentVersion)
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

func doUpdate(ctx context.Context) error {
	unlock, err := lock()
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

	addr, err := libutils.ParseAddr(cfg.Spec.Proxy)
	if err != nil {
		return trace.Errorf("failed to parse proxy server address: %w", err)
	}
	resp, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: addr.Addr,
		Timeout:   30 * time.Second,
	})
	if err != nil {
		return trace.Errorf("failed to request version from proxy: %w", err)
	}

	if cfg.Spec.ActiveVersion != resp.AgentVersion {
		err := createVersion(ctx, cfg.Spec.URITemplate, resp.AgentVersion)
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

func download(ctx context.Context, url string) (r io.ReadSeekCloser, sum []byte, err error) {
	f, err := os.CreateTemp("", "teleport-update-")
	if err != nil {
		return nil, nil, trace.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		if err != nil {
			f.Close()
			if err := os.Remove(f.Name()); err != nil {
				plog.WarnContext(ctx, "Failed to cleanup temporary download after error", "error", err)
			}
		}
	}()
	free, err := freeDisk(os.TempDir())
	if err != nil {
		return nil, nil, trace.Errorf("failed to calculate free disk: %w", err)
	}

	client, err := newClient(&downloadConfig{})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.ContentLength < 0 {
		plog.Warn("Content length missing from response, unable to verify Teleport download size")
	} else if resp.ContentLength > free {
		return nil, nil, trace.Errorf("size of download exceeds available disk space")
	}
	shaReader := sha256.New()
	n, err := io.Copy(f, io.TeeReader(resp.Body, shaReader))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return nil, nil, trace.Errorf("mismatch in Teleport download size")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, trace.Errorf("failed seek to start of download: %w", err)
	}
	return rmCloser{f}, shaReader.Sum(nil), nil
}

type rmCloser struct {
	*os.File
}

func (r rmCloser) Close() error {
	err := r.File.Close()
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.Remove(r.File.Name())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getChecksum(ctx context.Context, url string) ([]byte, error) {
	client, err := newClient(&downloadConfig{
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	r := io.LimitReader(resp.Body, 64)
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sum, err := hex.DecodeString(buf.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sum, nil
}

func removeVersion(version string) error {
	versionPath := filepath.Join(versionsDir, version)
	sumPath := filepath.Join(versionPath, "sha256")

	// invalidate checksum first, to protect against partially-removed
	// directory with valid checksum
	if err := os.Remove(sumPath); err != nil {
		return trace.Wrap(err)
	}
	if err := os.RemoveAll(versionPath); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func createVersion(ctx context.Context, uriTmpl, version string) error {
	versionPath := filepath.Join(versionsDir, version)
	sumPath := filepath.Join(versionPath, "sha256")

	tmpl, err := template.New("uri").Parse(uriTmpl)
	if err != nil {
		return trace.Wrap(err)
	}
	var uriBuf bytes.Buffer
	params := struct {
		OS, Version, Arch string
	}{runtime.GOOS, version, runtime.GOARCH}
	err = tmpl.Execute(&uriBuf, params)
	if err != nil {
		return trace.Wrap(err)
	}
	uri := uriBuf.String()

	sum, err := getChecksum(ctx, uri)
	if err != nil {
		return trace.Wrap(err)
	}
	existSum, err := readChecksum(sumPath)
	if err == nil {
		if bytes.Equal(existSum, sum) {
			plog.InfoContext(ctx, "Version already present", "version", version)
			return nil
		}
		plog.WarnContext(ctx, "Removing version that does not match checksum", "version", version)
		if err := removeVersion(version); err != nil {
			return trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err)
	}
	tgz, pathSum, err := download(ctx, uri)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := tgz.Close(); err != nil {
			plog.WarnContext(ctx, "Failed to cleanup temporary download after error", "error", err)
		}
	}()

	if !bytes.Equal(sum, pathSum) {
		return trace.Errorf("mismatched checksum, download possibly corrupt")
	}
	// avoid gzip bomb by validating checksum before decompression
	n, err := uncompressedSize(tgz)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := tgz.Seek(0, io.SeekStart); err != nil {
		return trace.Errorf("failed seek to start: %w", err)
	}
	if err := os.MkdirAll(versionPath, 0755); err != nil {
		return trace.Wrap(err)
	}
	free, err := freeDisk(versionPath)
	if err != nil {
		return trace.Errorf("failed to calculate free disk in %q: %w", versionPath, err)
	}
	if d := free - n; d < 0 {
		return trace.Errorf("%q needs %d additional bytes of disk space for download", versionsDir, -d)
	}
	err = untar(tgz, versionPath)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.WriteFile(sumPath, sum, 0755)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func readChecksum(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(f, 65))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if n != 64 {
		return nil, trace.Errorf("mismatch in checksum size")
	}
	return buf.Bytes(), nil
}

func lock() (func(), error) {
	// Build the path to the lock file that will be used by flock.
	lockFile := filepath.Join(versionsDir, ".lock")

	// Create the advisory lock using flock.
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

const reservedFreeDisk = 10_000_000 // 10 MiB

func freeDisk(dir string) (int64, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(dir, &stat)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if int64(stat.Bavail) < 0 {
		return 0, trace.Errorf("invalid size")
	}
	return int64(stat.Bavail) - reservedFreeDisk, nil
}

func uncompressedSize(f io.Reader) (int64, error) {
	// NOTE: The gzip length trailer is very unreliable,
	//   but we could optimize this in the future if
	//   we are willing to verify that all published
	//   Teleport tarballs have valid trailers.
	r, err := gzip.NewReader(f)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return n, nil
}

func validate(cf *cliConfig) error {
	if cf.Template == "" {
		cf.Template = cdnURITemplate
	}
	return nil
}
