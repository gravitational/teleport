/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package selinux

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/versioncontrol"
)

//go:embed teleport_ssh.te
var module []byte

//go:embed teleport_ssh.fc.tmpl
var fileContexts string

// ModuleSource returns the source of the SELinux SSH module.
func ModuleSource() []byte {
	return module
}

type filePaths struct {
	InstallDir     string
	DataDir        string
	ConfigPath     string
	UpgradeUnitDir string
}

// FileContexts returns file contexts for the SELinux SSH module.
func FileContexts(installDir, dataDir, configPath string) ([]byte, error) {
	fcTempl, err := template.New("selinux file contexts").Parse(fileContexts)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse file contexts template")
	}

	// Generate a file specifying the locations of important dirs so SELinux
	// will allow Teleport SSH to be able to access them.
	var buf bytes.Buffer
	err = fcTempl.Execute(&buf, filePaths{
		InstallDir:     installDir,
		DataDir:        dataDir,
		ConfigPath:     configPath,
		UpgradeUnitDir: versioncontrol.UnitConfigDir,
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to expand file contexts template")
	}

	return buf.Bytes(), nil
}
