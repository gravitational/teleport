/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
)

// EditCommand implements the `tctl edit` command for modifying
// Teleport resources.
type EditCommand struct {
	app    *kingpin.Application
	cmd    *kingpin.CmdClause
	config *service.Config
	ref    services.Ref
}

func (e *EditCommand) Initialize(app *kingpin.Application, config *service.Config) {
	e.app = app
	e.config = config
	e.cmd = app.Command("edit", "Edit a Teleport resource")
	e.cmd.Arg("resource type/resource name", `Resource to update
	<resource type>  Type of a resource [for example: rc]
	<resource name>  Resource name to update
	
	Example:
	$ tctl edit rc/remote`).SetValue(&e.ref)
}

func (e *EditCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	if cmd != e.cmd.FullCommand() {
		return false, nil
	}

	f, err := os.CreateTemp("", "teleport-resource*.yaml")
	if err != nil {
		return true, trace.Wrap(err)
	}

	defer func() {
		if err := os.Remove(f.Name()); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not remove temporary file %v\n", f.Name())
		}
	}()

	rc := &ResourceCommand{
		refs:     services.Refs{e.ref},
		format:   teleport.YAML,
		stdout:   f,
		filename: f.Name(),
		force:    true,
	}
	rc.Initialize(e.app, e.config)

	err = rc.Get(ctx, client)
	if closeErr := f.Close(); closeErr != nil {
		return true, trace.Wrap(err)
	}
	if err != nil {
		return true, trace.Wrap(err, "could not get resource %v: %v", rc.ref.String(), err)
	}

	args := strings.Fields(editor())
	editorCmd := exec.CommandContext(ctx, args[0], append(args[1:], f.Name())...)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Start(); err != nil {
		return true, trace.BadParameter("could not start editor %v: %v", editor(), err)
	}
	if err := editorCmd.Wait(); err != nil {
		return true, trace.BadParameter("skipping resource update, editor did not complete successfully: %v", err)
	}

	// TODO(zmb3): consider skipping this step if the file hasn't changed
	if err := rc.Create(ctx, client); err != nil {
		return true, trace.Wrap(err)
	}

	return true, nil
}

// editor gets the text editor to be used for editing the resource
func editor() string {
	for _, v := range []string{"TELEPORT_EDITOR", "VISUAL", "EDITOR"} {
		if value := os.Getenv(v); value != "" {
			return value
		}
	}
	return "vi"
}
