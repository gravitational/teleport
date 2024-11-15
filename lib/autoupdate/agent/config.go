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
	"os"
	"path/filepath"
	"text/template"

	"github.com/gravitational/trace"
)

const (
	teleportDropinTemplate = `# teleport-update
[Service]
ExecStopPost=/bin/bash -c 'date +%%s > {{.DataDir}}/last-restart'
`
	updateServiceTemplate = `# teleport-update
[Unit]
Description=Teleport update service

[Service]
Type=oneshot
ExecStart={{.LinkDir}}/bin/teleport-update update
`
	updateTimerTemplate = `# teleport-update
[Unit]
Description=Teleport update timer unit

[Timer]
OnActiveSec=1m
OnUnitActiveSec=5m
RandomizedDelaySec=1m

[Install]
WantedBy=teleport.service
`
)

func WriteConfigFiles(linkDir, dataDir string) error {
	// TODO(sclevine): revert on failure

	dropinPath := filepath.Join(linkDir, serviceDir, serviceName+".d", serviceDropinName)
	err := writeTemplate(dropinPath, teleportDropinTemplate, linkDir, dataDir)
	if err != nil {
		return trace.Wrap(err)
	}
	servicePath := filepath.Join(linkDir, serviceDir, updateServiceName)
	err = writeTemplate(servicePath, updateServiceTemplate, linkDir, dataDir)
	if err != nil {
		return trace.Wrap(err)
	}
	timerPath := filepath.Join(linkDir, serviceDir, updateTimerName)
	err = writeTemplate(timerPath, updateTimerTemplate, linkDir, dataDir)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func writeTemplate(path, t, linkDir, dataDir string) error {
	if err := os.MkdirAll(filepath.Dir(path), systemDirMode); err != nil {
		return trace.Wrap(err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, configFileMode)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()
	tmpl, err := template.New(filepath.Base(path)).Parse(t)
	if err != nil {
		return trace.Wrap(err)
	}
	err = tmpl.Execute(f, struct {
		LinkDir string
		DataDir string
	}{linkDir, dataDir})
	return trace.Wrap(f.Close())
}
