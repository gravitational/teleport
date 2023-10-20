/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
			"--key", c.profile.KeyPath(),
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
