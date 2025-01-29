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

package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// EditCommand implements the `tctl edit` command for modifying
// Teleport resources.
type EditCommand struct {
	app     *kingpin.Application
	cmd     *kingpin.CmdClause
	config  *servicecfg.Config
	ref     services.Ref
	confirm bool

	// Editor is used by tests to inject the editing mechanism
	// so that different scenarios can be asserted.
	Editor func(filename string) error
}

func (e *EditCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	e.app = app
	e.config = config
	e.cmd = app.Command("edit", "Edit a Teleport resource.")
	e.cmd.Arg("resource type/resource name", `Resource to update
	<resource type>  Type of a resource [for example: rc]
	<resource name>  Resource name to update

	Example:
	$ tctl edit rc/remote`).SetValue(&e.ref)
	e.cmd.Flag("confirm", "Confirm an unsafe or temporary resource update").Hidden().BoolVar(&e.confirm)
}

func (e *EditCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (bool, error) {
	if cmd != e.cmd.FullCommand() {
		return false, nil
	}
	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)
	err = e.editResource(ctx, client)
	return true, trace.Wrap(err)
}

func (e *EditCommand) runEditor(ctx context.Context, name string) error {
	if e.Editor != nil {
		return trace.Wrap(e.Editor(name))
	}

	textEditor := getTextEditor()
	args := strings.Fields(textEditor)
	editorCmd := exec.CommandContext(ctx, args[0], append(args[1:], name)...)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Start(); err != nil {
		return trace.BadParameter("could not start editor %v: %v", textEditor, err)
	}
	if err := editorCmd.Wait(); err != nil {
		return trace.BadParameter("skipping resource update, editor did not complete successfully: %v", err)
	}

	return nil
}

func (e *EditCommand) editResource(ctx context.Context, client *authclient.Client) error {
	f, err := os.CreateTemp("", "teleport-resource*.yaml")
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := os.Remove(f.Name()); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not remove temporary file %v\n", f.Name())
		}
	}()

	rc := &ResourceCommand{
		refs:        services.Refs{e.ref},
		format:      teleport.YAML,
		stdout:      f,
		filename:    f.Name(),
		force:       true,
		withSecrets: true,
		confirm:     e.confirm,
	}
	rc.Initialize(e.app, nil, e.config)

	err = rc.Get(ctx, client)
	if closeErr := f.Close(); closeErr != nil {
		return trace.Wrap(err)
	}
	if err != nil {
		return trace.Wrap(err, "could not get resource %v", rc.ref.String())
	}

	originalSum, err := checksum(f.Name())
	if err != nil {
		return trace.Wrap(err)
	}

	originalName, err := resourceName(f.Name())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := e.runEditor(ctx, f.Name()); err != nil {
		return trace.Wrap(err)
	}

	newSum, err := checksum(f.Name())
	if err != nil {
		return trace.Wrap(err)
	}

	// nothing to do if the resource was not modified
	if newSum == originalSum {
		fmt.Println("edit canceled, no changes made")
		return nil
	}

	newName, err := resourceName(f.Name())
	if err != nil {
		return trace.Wrap(err)
	}

	if originalName != newName {
		return trace.NotImplemented("renaming resources is not supported with tctl edit")
	}

	f, err = utils.OpenFileAllowingUnsafeLinks(rc.filename)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Close()

	decoder := kyaml.NewYAMLOrJSONDecoder(f, defaults.LookaheadBufSize)
	var raw services.UnknownResource
	if err := decoder.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			return trace.BadParameter("no resources found, empty input?")
		}
		return trace.Wrap(err)
	}

	// Use the UpdateHandler if the resource has one, otherwise fallback to using
	// the CreateHandler. UpdateHandlers are preferred over CreateHandler because an update
	// will not forcibly overwrite a resource unlike with create which requires the force
	// flag to be set to update an existing resource.
	updator, found := rc.UpdateHandlers[ResourceKind(raw.Kind)]
	if found {
		return trace.Wrap(updator(ctx, client, raw))
	}

	// TODO(tross) remove the fallback to CreateHandlers once all the resources
	// have been updated to implement an UpdateHandler.
	if creator, found := rc.CreateHandlers[ResourceKind(raw.Kind)]; found {
		return trace.Wrap(creator(ctx, client, raw))
	}

	return trace.BadParameter("updating resources of type %q is not supported", raw.Kind)
}

// getTextEditor returns the text editor to be used for editing the resource.
func getTextEditor() string {
	for _, v := range []string{"TELEPORT_EDITOR", "VISUAL", "EDITOR"} {
		if value := os.Getenv(v); value != "" {
			return value
		}
	}
	return "vi"
}

func checksum(filename string) (string, error) {
	f, err := utils.OpenFileAllowingUnsafeLinks(filename)
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
	f, err := utils.OpenFileAllowingUnsafeLinks(filename)
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
