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
package main

import (
	"os"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
	tctl "github.com/gravitational/teleport/tool/tctl/common"
)

func main() {
	if err := os.RemoveAll(outputPath()); err != nil {
		utils.FatalError(err)
	}
	if err := generateAll(); err != nil {
		utils.FatalError(err)
	}
}

func generateAll() error {
	return trace.NewAggregate(
		genDBCreateUserDBNameWarning(),
		genDBReferenceTCLAuthSign(),
	)
}

func init() {
	tctlApp = kingpin.New("tctl", tctl.GlobalHelpString)
	for _, command := range tctl.Commands() {
		command.Initialize(tctlApp, nil)
	}
}

var tctlApp *kingpin.Application
