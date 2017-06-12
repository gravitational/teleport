/*
Copyright 2017 Gravitational, Inc.

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
	"os"
	"path"

	"github.com/gravitational/teleport/tool/tsh/common"
)

func main() {
	cmd_line_orig := os.Args[1:]
	cmd_line := []string{}

	// lets see: if the executable name is 'ssh' or 'scp' we convert
	// that to "tsh ssh" or "tsh scp"
	switch path.Base(os.Args[0]) {
	case "ssh":
		cmd_line = append([]string{"ssh"}, cmd_line_orig...)
	case "scp":
		cmd_line = append([]string{"scp"}, cmd_line_orig...)
	default:
		cmd_line = cmd_line_orig
	}
	common.Run(cmd_line, false)
}
