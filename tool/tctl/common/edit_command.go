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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
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

	originalSum, err := checksum(f.Name())
	if err != nil {
		return true, trace.Wrap(err)
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

	newSum, err := checksum(f.Name())
	if err != nil {
		return true, trace.Wrap(err)
	}

	// nothing to do if the resource was not modified
	if newSum == originalSum {
		fmt.Println("edit canceled, no changes made")
		return true, nil
	}

	newName, err := resourceName(f.Name())
	if err != nil {
		return true, trace.Wrap(err)
	}

	if e.ref.Name != newName {
		return true, trace.NotImplemented("renaming resources is not supported with tctl edit")
	}

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

func checksum(filename string) (string, error) {
	f, err := utils.OpenFile(filename)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", trace.Wrap(err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func resourceName(filename string) (string, error) {
	f, err := utils.OpenFile(filename)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer f.Close()

	decoder := kyaml.NewYAMLOrJSONDecoder(f, defaults.LookaheadBufSize)

	var raw services.UnknownResource
	if err := decoder.Decode(&raw); err != nil {
		return "", trace.Wrap(err)
	}

	return raw.GetName(), nil
}
