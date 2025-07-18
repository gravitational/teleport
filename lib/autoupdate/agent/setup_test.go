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

package agent

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/autoupdate/agent/internal"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestNewNamespace(t *testing.T) {
	for _, p := range []struct {
		name       string
		namespace  string
		installDir string
		errMatch   string
		ns         *Namespace
	}{
		{
			name: "no namespace",
			ns: &Namespace{
				dataDir:               "/var/lib/teleport",
				installDir:            "/opt/teleport",
				defaultPathDir:        "/usr/local/bin",
				teleportServiceFile:   "/lib/systemd/system/teleport.service",
				teleportConfigFile:    "/etc/teleport.yaml",
				teleportPIDFile:       "/run/teleport.pid",
				needrestartConfigFile: "/etc/needrestart/conf.d/teleport-update.conf",
				teleportDropInFile:    "/etc/systemd/system/teleport.service.d/teleport-update.conf",
				updaterIDFile:         "/TMP/teleport-update.id",
				updaterServiceFile:    "/etc/systemd/system/teleport-update.service",
				updaterTimerFile:      "/etc/systemd/system/teleport-update.timer",
				deprecatedDropInFile:  "/etc/systemd/system/teleport-upgrade.service.d/teleport-update.conf",
				tbotServiceFile:       "/etc/systemd/system/tbot.service",
				tbotConfigFile:        "/etc/tbot.yaml",
				tbotPIDFile:           "/run/tbot.pid",
			},
		},
		{
			name:       "no namespace with dirs",
			installDir: "/install",
			ns: &Namespace{
				dataDir:               "/var/lib/teleport",
				installDir:            "/install",
				defaultPathDir:        "/usr/local/bin",
				teleportServiceFile:   "/lib/systemd/system/teleport.service",
				teleportConfigFile:    "/etc/teleport.yaml",
				teleportPIDFile:       "/run/teleport.pid",
				updaterIDFile:         "/TMP/teleport-update.id",
				updaterServiceFile:    "/etc/systemd/system/teleport-update.service",
				updaterTimerFile:      "/etc/systemd/system/teleport-update.timer",
				teleportDropInFile:    "/etc/systemd/system/teleport.service.d/teleport-update.conf",
				deprecatedDropInFile:  "/etc/systemd/system/teleport-upgrade.service.d/teleport-update.conf",
				tbotServiceFile:       "/etc/systemd/system/tbot.service",
				tbotConfigFile:        "/etc/tbot.yaml",
				tbotPIDFile:           "/run/tbot.pid",
				needrestartConfigFile: "/etc/needrestart/conf.d/teleport-update.conf",
			},
		},
		{
			name:      "test namespace",
			namespace: "test",
			ns: &Namespace{
				name:                  "test",
				dataDir:               "/var/lib/teleport_test",
				installDir:            "/opt/teleport",
				defaultPathDir:        "/opt/teleport/test/bin",
				teleportServiceFile:   "/etc/systemd/system/teleport_test.service",
				teleportConfigFile:    "/etc/teleport_test.yaml",
				teleportPIDFile:       "/run/teleport_test.pid",
				updaterIDFile:         "/TMP/teleport-update_test.id",
				updaterServiceFile:    "/etc/systemd/system/teleport-update_test.service",
				updaterTimerFile:      "/etc/systemd/system/teleport-update_test.timer",
				teleportDropInFile:    "/etc/systemd/system/teleport_test.service.d/teleport-update_test.conf",
				tbotServiceFile:       "/etc/systemd/system/tbot_test.service",
				tbotConfigFile:        "/etc/tbot_test.yaml",
				tbotPIDFile:           "/run/tbot_test.pid",
				needrestartConfigFile: "/etc/needrestart/conf.d/teleport-update_test.conf",
			},
		},
		{
			name:       "test namespace with dirs",
			namespace:  "test",
			installDir: "/install",
			ns: &Namespace{
				name:                  "test",
				dataDir:               "/var/lib/teleport_test",
				installDir:            "/install",
				defaultPathDir:        "/install/test/bin",
				teleportConfigFile:    "/etc/teleport_test.yaml",
				teleportPIDFile:       "/run/teleport_test.pid",
				teleportServiceFile:   "/etc/systemd/system/teleport_test.service",
				updaterIDFile:         "/TMP/teleport-update_test.id",
				updaterServiceFile:    "/etc/systemd/system/teleport-update_test.service",
				updaterTimerFile:      "/etc/systemd/system/teleport-update_test.timer",
				teleportDropInFile:    "/etc/systemd/system/teleport_test.service.d/teleport-update_test.conf",
				tbotServiceFile:       "/etc/systemd/system/tbot_test.service",
				tbotConfigFile:        "/etc/tbot_test.yaml",
				tbotPIDFile:           "/run/tbot_test.pid",
				needrestartConfigFile: "/etc/needrestart/conf.d/teleport-update_test.conf",
			},
		},
		{
			name:      "reserved default",
			namespace: defaultNamespace,
			errMatch:  "reserved",
		},
		{
			name:      "reserved system",
			namespace: systemNamespace,
			errMatch:  "reserved",
		},
	} {
		t.Run(p.name, func(t *testing.T) {
			log := slog.Default()
			ctx := context.Background()
			ns, err := NewNamespace(ctx, log, p.namespace, p.installDir)
			if p.errMatch != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), p.errMatch)
				return
			}
			require.NoError(t, err)
			ns.log = nil
			ns.updaterIDFile = strings.Replace(ns.updaterIDFile,
				strings.TrimSuffix(os.TempDir(), "/"), "/TMP", 1,
			)
			require.Equal(t, p.ns, ns)
		})
	}
}

func TestWriteConfigFiles(t *testing.T) {
	for _, p := range []struct {
		name       string
		namespace  string
		customTbot bool
	}{
		{
			name: "no namespace",
		},
		{
			name:      "test namespace",
			namespace: "test",
		},
		{
			name:       "test with custom tbot",
			customTbot: true,
		},
		{
			name:       "test namespace with custom tbot",
			customTbot: true,
		},
	} {
		t.Run(p.name, func(t *testing.T) {
			log := slog.Default()
			linkDir := t.TempDir()
			ctx := context.Background()
			ns, err := NewNamespace(ctx, log, p.namespace, "")
			require.NoError(t, err)
			ns.updaterServiceFile = rebasePath(filepath.Join(linkDir, serviceDir), ns.updaterServiceFile)
			ns.updaterTimerFile = rebasePath(filepath.Join(linkDir, serviceDir), ns.updaterTimerFile)
			ns.teleportDropInFile = rebasePath(filepath.Join(linkDir, serviceDir, filepath.Base(filepath.Dir(ns.teleportDropInFile))), ns.teleportDropInFile)
			ns.deprecatedDropInFile = rebasePath(filepath.Join(linkDir, serviceDir, filepath.Base(filepath.Dir(ns.deprecatedDropInFile))), ns.deprecatedDropInFile)
			ns.needrestartConfigFile = rebasePath(linkDir, filepath.Base(ns.needrestartConfigFile))
			if p.customTbot {
				ns.tbotServiceFile = rebasePath(filepath.Join(linkDir, serviceDir), ns.tbotServiceFile)
				err := os.MkdirAll(filepath.Dir(ns.tbotServiceFile), os.ModePerm)
				require.NoError(t, err)
				err = os.WriteFile(ns.tbotServiceFile, []byte("custom"), os.ModePerm)
				require.NoError(t, err)
			}
			err = ns.writeConfigFiles(ctx, linkDir, NewRevision("version", 0))
			require.NoError(t, err)

			for _, tt := range []struct {
				name string
				path string
			}{
				{name: "service", path: ns.updaterServiceFile},
				{name: "timer", path: ns.updaterTimerFile},
				{name: "dropin", path: ns.teleportDropInFile},
				{name: "deprecated", path: ns.deprecatedDropInFile},
				{name: "needrestart", path: ns.needrestartConfigFile},
			} {
				if tt.path == "" {
					continue
				}
				t.Run(tt.name, func(t *testing.T) {
					data, err := os.ReadFile(tt.path)
					require.NoError(t, err)
					data = replaceValues(data, map[string]string{
						defaultPathDir: linkDir,
					})
					if golden.ShouldSet() {
						golden.Set(t, data)
					}
					require.Equal(t, string(golden.Get(t)), string(data))
				})
			}
		})
	}
}

func TestHasCustomTbot(t *testing.T) {
	for _, tt := range []struct {
		name    string
		present bool
		header  bool

		result bool
	}{
		{
			name: "does not exist",
		},
		{
			name:    "exists",
			present: true,
			result:  true,
		},
		{
			name:    "exists with header",
			present: true,
			header:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tempdir := t.TempDir()
			ns := &Namespace{
				log:             slog.Default(),
				tbotServiceFile: filepath.Join(tempdir, "tbot.service"),
			}
			err := os.MkdirAll(filepath.Dir(ns.tbotServiceFile), os.ModePerm)
			require.NoError(t, err)
			header := "custom"
			if tt.header {
				header = markerPrefix
			}
			if tt.present {
				err = os.WriteFile(ns.tbotServiceFile, []byte(header), os.ModePerm)
				require.NoError(t, err)
			}
			ctx := context.Background()
			res, err := ns.HasCustomTbot(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.result, res)
		})
	}
}

func rebasePath(newBase, oldPath string) string {
	if oldPath == "" {
		return ""
	}
	return filepath.Join(newBase, filepath.Base(oldPath))
}

func replaceValues(data []byte, m map[string]string) []byte {
	for k, v := range m {
		data = bytes.ReplaceAll(data, []byte(v), []byte(k))
	}
	return data
}

func TestNamespace_overrideFromConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		teleportConfig *internal.UnversionedConfig
		tbotConfig     *internal.UnversionedConfig
		customTbot     bool
		want           Namespace
	}{
		{
			name: "default",
			teleportConfig: &internal.UnversionedConfig{
				ProxyServer: "example.com",
				DataDir:     "/data",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:3080",
				dataDir:          "/data",
			},
		},
		{
			name:           "empty",
			teleportConfig: &internal.UnversionedConfig{},
			want: Namespace{
				defaultProxyAddr: "default.example.com",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "full proxy",
			teleportConfig: &internal.UnversionedConfig{
				ProxyServer: "https://example.com:8080",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:8080",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "protocol and host",
			teleportConfig: &internal.UnversionedConfig{
				ProxyServer: "https://example.com",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:3080",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "host and port",
			teleportConfig: &internal.UnversionedConfig{
				ProxyServer: "example.com:443",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:443",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "host",
			teleportConfig: &internal.UnversionedConfig{
				ProxyServer: "example.com",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:3080",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "auth server (v3)",
			teleportConfig: &internal.UnversionedConfig{
				AuthServer: "example.com",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:3025",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "auth server (v1/2)",
			teleportConfig: &internal.UnversionedConfig{
				AuthServers: []string{
					"one.example.com",
					"two.example.com",
				},
			},
			want: Namespace{
				defaultProxyAddr: "one.example.com:3025",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "proxy priority",
			teleportConfig: &internal.UnversionedConfig{
				ProxyServer: "one.example.com",
				AuthServer:  "two.example.com",
				AuthServers: []string{"three.example.com"},
			},
			want: Namespace{
				defaultProxyAddr: "one.example.com:3080",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "auth priority",
			teleportConfig: &internal.UnversionedConfig{
				AuthServer:  "two.example.com",
				AuthServers: []string{"three.example.com"},
			},
			want: Namespace{
				defaultProxyAddr: "two.example.com:3025",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "missing",
			want: Namespace{
				defaultProxyAddr: "default.example.com",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "tbot managed",
			tbotConfig: &internal.UnversionedConfig{
				ProxyServer: "example.com",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:3080",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "tbot unmanaged",
			tbotConfig: &internal.UnversionedConfig{
				ProxyServer: "example.com",
			},
			customTbot: true,
			want: Namespace{
				defaultProxyAddr: "default.example.com",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "teleport overrides tbot",
			teleportConfig: &internal.UnversionedConfig{
				ProxyServer: "example.com",
				DataDir:     "/data",
			},
			tbotConfig: &internal.UnversionedConfig{
				ProxyServer: "other.example.com",
				DataDir:     "/other-data",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:3080",
				dataDir:          "/data",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &Namespace{
				log:              slog.Default(),
				defaultProxyAddr: "default.example.com",
				dataDir:          "/var/lib/teleport",
			}
			if tt.customTbot {
				ns.tbotServiceFile = filepath.Join(t.TempDir(), "tbot.service")
				err := os.WriteFile(ns.tbotServiceFile, []byte("custom"), os.ModePerm)
				require.NoError(t, err)
			}
			if tt.teleportConfig != nil {
				ns.teleportConfigFile = filepath.Join(t.TempDir(), "teleport.yaml")
				out, err := yaml.Marshal(internal.UnversionedTeleport{Teleport: *tt.teleportConfig})
				require.NoError(t, err)
				err = os.WriteFile(ns.teleportConfigFile, out, os.ModePerm)
				require.NoError(t, err)
			}
			if tt.tbotConfig != nil {
				ns.tbotConfigFile = filepath.Join(t.TempDir(), "tbot.yaml")
				out, err := yaml.Marshal(tt.tbotConfig)
				require.NoError(t, err)
				err = os.WriteFile(ns.tbotConfigFile, out, os.ModePerm)
				require.NoError(t, err)
			}
			ctx := context.Background()
			ns.overrideFromConfig(ctx)
			ns.teleportConfigFile = ""
			ns.tbotConfigFile = ""
			ns.tbotServiceFile = ""
			ns.log = nil
			require.Equal(t, &tt.want, ns)
		})
	}
}

func TestWriteTeleportService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		pidFile    string
		configFile string
		pathDir    string
		flags      autoupdate.InstallFlags
	}{
		{
			name:       "default",
			pidFile:    "/var/run/teleport.pid",
			configFile: "/etc/teleport.yaml",
			pathDir:    "/usr/local/bin",
		},
		{
			name:       "custom",
			pidFile:    "/some/path/teleport.pid",
			configFile: "/some/path/teleport.yaml",
			pathDir:    "/some/path/bin",
		},
		{
			name:       "FIPS",
			pidFile:    "/var/run/teleport.pid",
			configFile: "/etc/teleport.yaml",
			pathDir:    "/usr/local/bin",
			flags:      autoupdate.FlagFIPS,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceFile := filepath.Join(t.TempDir(), "file")
			ns := &Namespace{
				log:                 slog.Default(),
				teleportConfigFile:  tt.configFile,
				teleportServiceFile: serviceFile,
				teleportPIDFile:     tt.pidFile,
			}
			err := ns.WriteTeleportService(context.Background(), tt.pathDir, NewRevision("version", tt.flags))
			require.NoError(t, err)
			data, err := os.ReadFile(serviceFile)
			require.NoError(t, err)
			if golden.ShouldSet() {
				golden.Set(t, data)
			}
			require.Equal(t, string(golden.Get(t)), string(data))
		})
	}
}

func TestWriteTbotService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		suffix     string
		pidFile    string
		configFile string
		pathDir    string
		dataDir    string
		err        bool
	}{
		{
			name:       "default",
			pidFile:    "/run/tbot.pid",
			configFile: "/etc/tbot.yaml",
			pathDir:    "/usr/local/bin",
			dataDir:    "/var/lib/teleport",
		},
		{
			name:       "custom",
			pidFile:    "/run/tbot.pid",
			configFile: "/some/path/tbot.yaml",
			pathDir:    "/some/path/bin",
			dataDir:    "/some/path",
		},
		{
			name:       "custom suffix",
			suffix:     "suffix",
			pidFile:    "/run/tbot_suffix.pid",
			configFile: "/some/path/tbot.yaml",
			pathDir:    "/some/path/bin",
			dataDir:    "/some/path",
		},
		{
			name:       "bad pid",
			pidFile:    "/some/path/tbot.pid",
			configFile: "/some/path/tbot.yaml",
			pathDir:    "/some/path/bin",
			dataDir:    "/some/path",
			err:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceFile := filepath.Join(t.TempDir(), "file")
			ns := &Namespace{
				log:             slog.Default(),
				name:            tt.suffix,
				tbotConfigFile:  tt.configFile,
				tbotServiceFile: serviceFile,
				tbotPIDFile:     tt.pidFile,
				dataDir:         tt.dataDir,
			}
			err := ns.WriteTbotService(context.Background(), tt.pathDir, NewRevision("version", 0))
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			data, err := os.ReadFile(serviceFile)
			require.NoError(t, err)
			if golden.ShouldSet() {
				golden.Set(t, data)
			}
			require.Equal(t, string(golden.Get(t)), string(data))
		})
	}
}

func TestReplaceTeleportService(t *testing.T) {
	t.Parallel()

	const defaultService = `
[Unit]
Description=Teleport Service
After=network.target

[Service]
Type=simple
Restart=always
RestartSec=5
EnvironmentFile=-/etc/default/teleport
ExecStart=/usr/local/bin/teleport start --config /etc/teleport.yaml --pid-file=/run/teleport.pid
# systemd before 239 needs an absolute path
ExecReload=/bin/sh -c "exec pkill -HUP -L -F /run/teleport.pid"
PIDFile=/run/teleport.pid
LimitNOFILE=524288

[Install]
WantedBy=multi-user.target
`

	tests := []struct {
		name string
		in   string

		pidFile    string
		configFile string
		pathDir    string
		flags      autoupdate.InstallFlags
	}{
		{
			name:       "default",
			in:         defaultService,
			pidFile:    "/var/run/teleport.pid",
			configFile: "/etc/teleport.yaml",
			pathDir:    "/usr/local/bin",
		},
		{
			name:       "custom",
			in:         defaultService,
			pidFile:    "/some/path/teleport.pid",
			configFile: "/some/path/teleport.yaml",
			pathDir:    "/some/path/bin",
		},
		{
			name:       "FIPS",
			in:         defaultService,
			pidFile:    "/var/run/teleport.pid",
			configFile: "/etc/teleport.yaml",
			pathDir:    "/usr/local/bin",
			flags:      autoupdate.FlagFIPS,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &Namespace{
				log:                slog.Default(),
				teleportConfigFile: tt.configFile,
				teleportPIDFile:    tt.pidFile,
			}
			data := ns.ReplaceTeleportService([]byte(tt.in), tt.pathDir, tt.flags)
			if golden.ShouldSet() {
				golden.Set(t, data)
			}
			require.Equal(t, string(golden.Get(t)), string(data))
		})
	}
}
