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
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/golden"
)

func TestNewNamespace(t *testing.T) {
	for _, p := range []struct {
		name      string
		namespace string
		linkDir   string
		dataDir   string
		errMatch  string
		ns        *Namespace
	}{
		{
			name: "no namespace",
			ns: &Namespace{
				dataDir:            "/var/lib/teleport",
				linkDir:            "/usr/local/bin",
				versionsDir:        "/opt/teleport/default/versions",
				serviceFile:        "/lib/systemd/system/teleport.service",
				configFile:         "/etc/teleport.yaml",
				pidFile:            "/run/teleport.pid",
				updaterLockFile:    "/opt/teleport/default/update.lock",
				updaterConfigFile:  "/opt/teleport/default/update.yaml",
				updaterBinFile:     "/usr/local/bin/teleport-update",
				updaterServiceFile: "/etc/systemd/system/teleport-update.service",
				updaterTimerFile:   "/etc/systemd/system/teleport-update.timer",
			},
		},
		{
			name:    "no namespace with dirs",
			linkDir: "/link",
			dataDir: "/data",
			ns: &Namespace{
				dataDir:            "/data",
				linkDir:            "/link",
				versionsDir:        "/opt/teleport/default/versions",
				serviceFile:        "/lib/systemd/system/teleport.service",
				configFile:         "/etc/teleport.yaml",
				pidFile:            "/run/teleport.pid",
				updaterLockFile:    "/opt/teleport/default/update.lock",
				updaterConfigFile:  "/opt/teleport/default/update.yaml",
				updaterBinFile:     "/link/teleport-update",
				updaterServiceFile: "/etc/systemd/system/teleport-update.service",
				updaterTimerFile:   "/etc/systemd/system/teleport-update.timer",
			},
		},
		{
			name:      "test namespace",
			namespace: "test",
			ns: &Namespace{
				name:               "test",
				dataDir:            "/var/lib/teleport_test",
				linkDir:            "/opt/teleport/test/bin",
				versionsDir:        "/opt/teleport/test/versions",
				serviceFile:        "/etc/systemd/system/teleport_test.service",
				configFile:         "/etc/teleport_test.yaml",
				pidFile:            "/run/teleport_test.pid",
				updaterLockFile:    "/opt/teleport/test/update.lock",
				updaterConfigFile:  "/opt/teleport/test/update.yaml",
				updaterBinFile:     "/opt/teleport/test/bin/teleport-update",
				updaterServiceFile: "/etc/systemd/system/teleport-update_test.service",
				updaterTimerFile:   "/etc/systemd/system/teleport-update_test.timer",
			},
		},
		{
			name:      "test namespace with dirs",
			namespace: "test",
			linkDir:   "/link",
			dataDir:   "/data",
			ns: &Namespace{
				name:               "test",
				dataDir:            "/data",
				linkDir:            "/link",
				versionsDir:        "/opt/teleport/test/versions",
				serviceFile:        "/etc/systemd/system/teleport_test.service",
				configFile:         "/etc/teleport_test.yaml",
				pidFile:            "/run/teleport_test.pid",
				updaterLockFile:    "/opt/teleport/test/update.lock",
				updaterConfigFile:  "/opt/teleport/test/update.yaml",
				updaterBinFile:     "/link/teleport-update",
				updaterServiceFile: "/etc/systemd/system/teleport-update_test.service",
				updaterTimerFile:   "/etc/systemd/system/teleport-update_test.timer",
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
			ns, err := NewNamespace(log, p.namespace, p.dataDir, p.linkDir)
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
			ns, err := NewNamespace(log, p.namespace, "", linkDir)
			require.NoError(t, err)
			ns.updaterServiceFile = filepath.Join(linkDir, serviceDir, filepath.Base(ns.updaterServiceFile))
			ns.updaterTimerFile = filepath.Join(linkDir, serviceDir, filepath.Base(ns.updaterTimerFile))
			err = ns.writeConfigFiles()
			require.NoError(t, err)

			data, err := os.ReadFile(ns.updaterServiceFile)
			require.NoError(t, err)
			data = replaceValues(data, map[string]string{
				DefaultLinkDir: linkDir,
			})
			if golden.ShouldSet() {
				golden.SetNamed(t, "service", data)
			}
			require.Equal(t, string(golden.GetNamed(t, "service")), string(data))

			data, err = os.ReadFile(ns.updaterTimerFile)
			require.NoError(t, err)
			data = replaceValues(data, map[string]string{
				DefaultLinkDir: linkDir,
			})
			if golden.ShouldSet() {
				golden.SetNamed(t, "timer", data)
			}
			require.Equal(t, string(golden.GetNamed(t, "timer")), string(data))
		})
	}
}

func replaceValues(data []byte, m map[string]string) []byte {
	for k, v := range m {
		data = bytes.ReplaceAll(data, []byte(v), []byte(k))
	}
	return data
}
