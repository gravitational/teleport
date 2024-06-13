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

package openssh

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
)

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		usr, err := apiutils.CurrentUser()
		if err != nil {
			return "", trace.ConvertSystemError(err)
		}
		home = usr.HomeDir
	}

	return filepath.Join(home, ".ssh", "config"), nil
}

func readConfig() (string, error) {
	path, err := defaultConfigPath()
	if err != nil {
		return "", trace.Wrap(err)
	}

	file, err := os.Open(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(content), nil
}

func writeConfig(content string) error {
	path, err := defaultConfigPath()
	if err != nil {
		return trace.Wrap(err)
	}

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return trace.Wrap(err)
}

func (c *SSHConfig) SaveToUserConfig(params *SSHConfigParameters) error {
	oldConfig, err := readConfig()
	if err != nil {
		// TODO handle not exist (create folder)
		return trace.Wrap(err)
	}

	// What is expected.
	var sb strings.Builder
	if err := c.GetSSHConfig(&sb, params); err != nil {
		return trace.Wrap(err)
	}

	// Up-to-date?
	newSection := sb.String()
	if strings.Contains(oldConfig, newSection) {
		return nil
	}

	// New Teleport content.
	header := fmt.Sprintf("# Begin generated Teleport configuration for %s by %s", params.ProxyHost, params.AppName)
	if !strings.Contains(oldConfig, header) {
		return trace.Wrap(writeConfig(oldConfig + "\n" + newSection))
	}

	// Replace.
	footer := fmt.Sprintf("# End generated Teleport configuration for %s by %s\n", params.ProxyHost, params.AppName)
	headerIndex := strings.Index(oldConfig, header)
	footerIndex := strings.LastIndex(oldConfig, footer)
	oldSection := oldConfig[headerIndex : footerIndex+len(footer)]
	newConfig := strings.ReplaceAll(oldConfig, oldSection, newSection)
	return trace.Wrap(writeConfig(newConfig))
}
