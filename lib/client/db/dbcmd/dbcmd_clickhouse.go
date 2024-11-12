/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package dbcmd

import (
	"fmt"
	"os/exec"
	"strconv"
)

const (
	// clickHouseNativeClientBin is the ClickHouse CLI program name that support interactive mode
	// connection with ClickHouse Native protocol to the ClickHouse Database.
	clickHouseNativeClientBin = "clickhouse-client"
)

func (c *CLICommandBuilder) getClickhouseHTTPCommand() (*exec.Cmd, error) {
	var curlCommand *exec.Cmd
	if c.options.noTLS {
		curlCommand = exec.Command(curlBin, fmt.Sprintf("http://%v:%v/", c.host, c.port))
	} else {
		args := []string{
			fmt.Sprintf("https://%v:%v/", c.host, c.port),
			"--key", c.profile.DatabaseKeyPathForCluster(c.tc.SiteName, c.db.ServiceName),
			"--cert", c.profile.DatabaseCertPathForCluster(c.tc.SiteName, c.db.ServiceName),
		}

		if c.tc.InsecureSkipVerify {
			args = append(args, "--insecure")
		}

		if c.options.caPath != "" {
			args = append(args, []string{"--cacert", c.options.caPath}...)
		}

		curlCommand = exec.Command(curlBin, args...)
	}

	return curlCommand, nil
}

func (c *CLICommandBuilder) getClickhouseNativeCommand() (*exec.Cmd, error) {
	args := []string{
		"--host", c.host,
		"--port", strconv.Itoa(c.port),
		"--user", c.db.Username,
	}
	return exec.Command(clickHouseNativeClientBin, args...), nil
}
