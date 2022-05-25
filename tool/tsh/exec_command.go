// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/teleport/lib/client"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
)

// execCommand is a command to execute shell command, while setting Teleport-specific environment variables to use.
// Can be used to run arbitrary programs by setting shell and controlling the arguments.
type execCommand struct {
	shell     string
	arguments []string

	cmd *kingpin.CmdClause
}

func findShell(shell string, lookPath func(file string) (string, error)) (string, error) {
	// if shell is explicit, don't try heuristics.
	if shell != "" {
		return lookPath(shell)
	}

	// try to find usable shell.
	options := []string{os.Getenv("SHELL"), "bash", "sh"}

	for _, option := range options {
		if option == "" {
			continue
		}

		fp, err := lookPath(option)
		if err == nil {
			return fp, nil
		}
	}

	// nothing worked.
	return "", trace.BadParameter("unable to find a working shell. set the SHELL env variable or make sure `bash` or `sh` are in PATH.")
}

func (c *execCommand) runCommand(cf *CLIConf) error {
	shell, err := findShell(c.shell, exec.LookPath)
	if err != nil {
		return trace.Wrap(err, "failed to find a working shell/executable")
	}

	cmd := exec.Command(shell, c.arguments...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// preserve existing env
	cmd.Env = os.Environ()

	// env variables from `tsh env`.
	// ignore errors if there is no profile.
	profile, _ := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	env := getTeleportEnvironment(profile)

	// extra stuff
	env["TSH"] = cf.executablePath
	env["TSH_COMMAND"] = shell

	// add new entries
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%v=%v", key, value))
	}

	log.Debugf("running command: %v", cmd)

	err = cmd.Run()
	if err == nil {
		return nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return trace.Wrap(exitErr)
	}

	return trace.Wrap(err, "failed to run command: %v %v", shell, strings.Join(c.arguments, " "))
}

func newExecCommand(app *kingpin.Application) *execCommand {
	execCmd := execCommand{}
	execCmd.cmd = app.Command("exec", "Run provided command, by default using a shell, with Teleport-specific environment variables set. Can be used to create combined commands.")
	execCmd.cmd.Hidden()
	execCmd.cmd.Flag("shell", "Shell to use. May be any executable. Defaults to first found out of: $SHELL, bash, sh.").StringVar(&execCmd.shell)
	execCmd.cmd.Arg("arguments", "Command arguments").StringsVar(&execCmd.arguments)
	execCmd.cmd.Alias(`
Notes:

  The command will be executed under modified environment, enriched with the following variables:

  | Name        | Value                                              |
  |-------------|----------------------------------------------------|
  | TSH_COMMAND | The program being executed.                        |
  | TSH         | The path of the tsh binary that invoked the alias. |
  
  Additionally, any variables reported by "tsh env" will also be included. If the "tsh env" reports
  no variables (e.g. because the user is not logged in), they will not be added.

Examples:

  1. Print some environment variables as well as reversed arguments:

    $ tsh exec -- -c 'echo $TSH $TSH_COMMAND $2 $1 $0' arg0 arg1 arg2
    /bin/tsh /bin/bash arg2 arg1 arg0

  2. Login to cluster "leaf" and ssh as "ubuntu" to "node-1":

    $ tsh exec -- -c '$TSH login --proxy=tele.example.com $0 && $TSH ssh $1' leaf ubuntu@node-1

  3. Execute Python script using "python3" as interpreter:

    $ tsh exec --shell python3 -- -c "import os; print('path to tsh:', os.environ['TSH'])"
    path to tsh: /opt/teleport/bin/tsh
`)
	return &execCmd
}
