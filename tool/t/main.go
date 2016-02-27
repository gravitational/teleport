/*
Copyright 2016 Gravitational, Inc.

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
package main

import (
	"fmt"
	"os"

	"github.com/gravitational/teleport/lib/utils"
)

func main() {
	run(os.Args[1:], false)
}

// run executes TSH client. same as main() but easier to test
func run(args []string, underTest bool) {
	utils.InitLoggerCLI()
	app := utils.InitCLIParser("t", "TSH: Teleport SSH client")

	ver := app.Command("version", "Print the version")
	ssh := app.Command("ssh", "SSH into a remote machine").Default()

	// parse CLI commands+flags:
	command, err := app.Parse(args)
	if err != nil {
		utils.FatalError(err)
	}

	switch command {
	case ver.FullCommand():
		onVersion()
	case ssh.FullCommand():
		// SSH is a default command. if there are no args, show the default usage:
		if len(args) == 0 {
			app.Usage([]string{})
			os.Exit(1)
		} else {
			onSSH()
		}
	}
}

func onSSH() {
	fmt.Println("SSH!")
}

func onVersion() {
	fmt.Println("Version!")
}
