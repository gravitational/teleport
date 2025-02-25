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
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/config"
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
				dataDir:             "/var/lib/teleport",
				installDir:          "/opt/teleport",
				defaultPathDir:      "/usr/local/bin",
				serviceFile:         "/lib/systemd/system/teleport.service",
				configFile:          "/etc/teleport.yaml",
				pidFile:             "/run/teleport.pid",
				updaterServiceFile:  "/etc/systemd/system/teleport-update.service",
				updaterTimerFile:    "/etc/systemd/system/teleport-update.timer",
				dropInFile:          "/etc/systemd/system/teleport.service.d/teleport-update.conf",
				needrestartConfFile: "/etc/needrestart/conf.d/teleport-update.conf",
			},
		},
		{
			name:       "no namespace with dirs",
			installDir: "/install",
			ns: &Namespace{
				dataDir:             "/var/lib/teleport",
				installDir:          "/install",
				defaultPathDir:      "/usr/local/bin",
				serviceFile:         "/lib/systemd/system/teleport.service",
				configFile:          "/etc/teleport.yaml",
				pidFile:             "/run/teleport.pid",
				updaterServiceFile:  "/etc/systemd/system/teleport-update.service",
				updaterTimerFile:    "/etc/systemd/system/teleport-update.timer",
				dropInFile:          "/etc/systemd/system/teleport.service.d/teleport-update.conf",
				needrestartConfFile: "/etc/needrestart/conf.d/teleport-update.conf",
			},
		},
		{
			name:      "test namespace",
			namespace: "test",
			ns: &Namespace{
				name:                "test",
				dataDir:             "/var/lib/teleport_test",
				installDir:          "/opt/teleport",
				defaultPathDir:      "/opt/teleport/test/bin",
				serviceFile:         "/etc/systemd/system/teleport_test.service",
				configFile:          "/etc/teleport_test.yaml",
				pidFile:             "/run/teleport_test.pid",
				updaterServiceFile:  "/etc/systemd/system/teleport-update_test.service",
				updaterTimerFile:    "/etc/systemd/system/teleport-update_test.timer",
				dropInFile:          "/etc/systemd/system/teleport_test.service.d/teleport-update_test.conf",
				needrestartConfFile: "/etc/needrestart/conf.d/teleport-update_test.conf",
			},
		},
		{
			name:       "test namespace with dirs",
			namespace:  "test",
			installDir: "/install",
			ns: &Namespace{
				name:                "test",
				dataDir:             "/var/lib/teleport_test",
				installDir:          "/install",
				defaultPathDir:      "/install/test/bin",
				configFile:          "/etc/teleport_test.yaml",
				pidFile:             "/run/teleport_test.pid",
				serviceFile:         "/etc/systemd/system/teleport_test.service",
				updaterServiceFile:  "/etc/systemd/system/teleport-update_test.service",
				updaterTimerFile:    "/etc/systemd/system/teleport-update_test.timer",
				dropInFile:          "/etc/systemd/system/teleport_test.service.d/teleport-update_test.conf",
				needrestartConfFile: "/etc/needrestart/conf.d/teleport-update_test.conf",
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
			require.Equal(t, p.ns, ns)
		})
	}
}

func TestWriteConfigFiles(t *testing.T) {
	for _, p := range []struct {
		name      string
		namespace string
	}{
		{
			name: "no namespace",
		},
		{
			name:      "test namespace",
			namespace: "test",
		},
	} {
		t.Run(p.name, func(t *testing.T) {
			log := slog.Default()
			linkDir := t.TempDir()
			ctx := context.Background()
			ns, err := NewNamespace(ctx, log, p.namespace, "")
			require.NoError(t, err)
			ns.updaterServiceFile = filepath.Join(linkDir, serviceDir, filepath.Base(ns.updaterServiceFile))
			ns.updaterTimerFile = filepath.Join(linkDir, serviceDir, filepath.Base(ns.updaterTimerFile))
			ns.dropInFile = filepath.Join(linkDir, serviceDir, filepath.Base(filepath.Dir(ns.dropInFile)), filepath.Base(ns.dropInFile))
			ns.needrestartConfFile = filepath.Join(linkDir, filepath.Base(ns.dropInFile))
			err = ns.writeConfigFiles(ctx, linkDir)
			require.NoError(t, err)

			for _, tt := range []struct {
				name string
				path string
			}{
				{name: "service", path: ns.updaterServiceFile},
				{name: "timer", path: ns.updaterTimerFile},
				{name: "dropin", path: ns.dropInFile},
				{name: "needrestart", path: ns.needrestartConfFile},
			} {
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

func replaceValues(data []byte, m map[string]string) []byte {
	for k, v := range m {
		data = bytes.ReplaceAll(data, []byte(v), []byte(k))
	}
	return data
}

func TestNamespace_overrideFromConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *unversionedTeleport
		want Namespace
	}{
		{
			name: "default",
			cfg: &unversionedTeleport{
				ProxyServer: "example.com",
				DataDir:     "/data",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:3080",
				dataDir:          "/data",
			},
		},
		{
			name: "empty",
			cfg:  &unversionedTeleport{},
			want: Namespace{
				defaultProxyAddr: "default.example.com",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "full proxy",
			cfg: &unversionedTeleport{
				ProxyServer: "https://example.com:8080",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:8080",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "protocol and host",
			cfg: &unversionedTeleport{
				ProxyServer: "https://example.com",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:3080",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "host and port",
			cfg: &unversionedTeleport{
				ProxyServer: "example.com:443",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:443",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "host",
			cfg: &unversionedTeleport{
				ProxyServer: "example.com",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:3080",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "auth server (v3)",
			cfg: &unversionedTeleport{
				AuthServer: "example.com",
			},
			want: Namespace{
				defaultProxyAddr: "example.com:3025",
				dataDir:          "/var/lib/teleport",
			},
		},
		{
			name: "auth server (v1/2)",
			cfg: &unversionedTeleport{
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
			cfg: &unversionedTeleport{
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
			cfg: &unversionedTeleport{
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &Namespace{
				log:              slog.Default(),
				configFile:       filepath.Join(t.TempDir(), "teleport.yaml"),
				defaultProxyAddr: "default.example.com",
				dataDir:          "/var/lib/teleport",
			}
			if tt.cfg != nil {
				out, err := yaml.Marshal(unversionedConfig{Teleport: *tt.cfg})
				require.NoError(t, err)
				err = os.WriteFile(ns.configFile, out, os.ModePerm)
				require.NoError(t, err)
			}
			ctx := context.Background()
			ns.overrideFromConfig(ctx)
			ns.configFile = ""
			ns.log = nil
			require.Equal(t, &tt.want, ns)
		})
	}
}

// In the future, the latest version of the updater may need to read a version of teleport.yaml that has
// an unsupported version which is supported by the updater-managed version of Teleport.
// This test will break if Teleport removes a field that the updater reads.
func TestUnversionedTeleportConfig(t *testing.T) {
	in := unversionedConfig{
		Teleport: unversionedTeleport{
			ProxyServer: "proxy.example.com",
			AuthServer:  "auth.example.com",
			AuthServers: []string{"auth1.example.com", "auth2.example.com"},
			DataDir:     "example_dir",
		},
	}
	var inB bytes.Buffer
	err := yaml.NewEncoder(&inB).Encode(in)
	require.NoError(t, err)
	fc, err := config.ReadConfig(&inB)
	require.NoError(t, err)

	var outB bytes.Buffer
	err = yaml.NewEncoder(&outB).Encode(fc)
	require.NoError(t, err)

	var out unversionedConfig
	err = yaml.NewDecoder(&outB).Decode(&out)
	require.NoError(t, err)
	require.Equal(t, in, out)
}
