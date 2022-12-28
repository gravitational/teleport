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
	"io"
	"math"
	"os"
	"strings"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
)

// VersionControlCommand implements the `tctl versioncontrol` family of commands.
type VersionControlCommand struct {
	config *service.Config

	phase string

	to, from string

	kind, name string

	status, msg string

	format string

	file string

	force bool

	vcStatus *kingpin.CmdClause

	vcDirectiveList      *kingpin.CmdClause
	vcDirectivePromote   *kingpin.CmdClause
	vcDirectiveSetStatus *kingpin.CmdClause
	vcDirectiveCreate    *kingpin.CmdClause
	vcDirectiveDelete    *kingpin.CmdClause

	vcInstallerList   *kingpin.CmdClause
	vcInstallerCreate *kingpin.CmdClause
	vcInstallerDelete *kingpin.CmdClause
}

// Initialize allows AccessRequestCommand to plug itself into the CLI parser
func (c *VersionControlCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config
	versioncontrol := app.Command("version-control", "Manage Teleport installation versions").Alias("vc")

	c.vcStatus = versioncontrol.Command("status", "Show version-control status summary")

	directive := versioncontrol.Command("directive", "Manage version directives").Alias("directives")

	c.vcDirectiveList = directive.Command("list", "List version directives").Alias("ls")
	c.vcDirectiveList.Flag("phase", "Directive phase, 'draft', 'pending', or 'active'").StringVar(&c.phase)
	c.vcDirectiveList.Flag("format", "Output format 'text', 'json' or 'yaml'").Default(teleport.Text).StringVar(&c.format)

	c.vcDirectivePromote = directive.Command("promote", "Manually promote a version directive").Hidden()
	c.vcDirectivePromote.Flag("kind", "Version directive kind").Required().StringVar(&c.kind)
	c.vcDirectivePromote.Flag("name", "Version directive name").Required().StringVar(&c.name)
	c.vcDirectivePromote.Flag("to", "Desired target phase").Required().StringVar(&c.to)
	c.vcDirectivePromote.Flag("from", "Phase of source directive").Required().StringVar(&c.from)

	c.vcDirectiveSetStatus = directive.Command("set-status", "Manually set version directive status").Hidden()
	c.vcDirectiveSetStatus.Flag("phase", "Directive phase, 'draft', 'pending', or 'active'").Required().StringVar(&c.phase)
	c.vcDirectiveSetStatus.Flag("kind", "Version directive kind").StringVar(&c.kind)
	c.vcDirectiveSetStatus.Flag("name", "Version directive name").StringVar(&c.name)
	c.vcDirectiveSetStatus.Flag("status", "Desired status").Required().StringVar(&c.status)
	c.vcDirectiveSetStatus.Flag("msg", "Status message").StringVar(&c.msg)

	c.vcDirectiveCreate = directive.Command("create", "Create a version directive")
	c.vcDirectiveCreate.Flag("force", "Forcibly overwrite the previous value.").Short('f').BoolVar(&c.force)
	c.vcDirectiveCreate.Flag("file", "Path to a resource file, or '-' to read from stdin.").Required().StringVar(&c.file)

	c.vcDirectiveDelete = directive.Command("delete", "Delete a version directive").Alias("rm").Hidden()
	c.vcDirectiveDelete.Flag("phase", "Directive phase, 'draft', 'pending', or 'active'").Required().StringVar(&c.phase)
	c.vcDirectiveDelete.Flag("kind", "Version directive kind").StringVar(&c.kind)
	c.vcDirectiveDelete.Flag("name", "Version directive name").StringVar(&c.name)

	installer := versioncontrol.Command("installer", "Manage version control installers").Alias("installers")

	c.vcInstallerList = installer.Command("list", "List version control installers").Alias("ls")
	c.vcInstallerList.Flag("kind", "Filter by installer kind").StringVar(&c.kind)
	c.vcInstallerList.Flag("format", "Output format 'text', 'json', or 'yaml'").Default(teleport.Text).StringVar(&c.format)

	c.vcInstallerCreate = installer.Command("create", "Create a version control installer")
	c.vcInstallerCreate.Flag("force", "Forcibly overwrite the previous value.").Short('f').BoolVar(&c.force)
	// the 'file' arg is required for now, but will become optional in the future since we want to extend
	// the 'installer create' command to let you build the installer from parameters (e.g. `--install.sh=/path/to/install.sh`).
	c.vcInstallerCreate.Flag("file", "Path to a resource file, or '-' to read from stdin.").Required().StringVar(&c.file)

	c.vcInstallerDelete = installer.Command("delete", "Delete a version control installer").Alias("rm").Hidden()
	c.vcInstallerDelete.Flag("kind", "Installer kind").Required().StringVar(&c.kind)
	c.vcInstallerDelete.Flag("name", "Installer name").Required().StringVar(&c.name)

	// TODO(fspmarshall): add higher-level helper commands to assist with promotion, poisoning, and simplified resource creation.
}

// TryRun takes the CLI command as an argument (like "inventory status") and executes it.
func (c *VersionControlCommand) TryRun(ctx context.Context, cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.vcStatus.FullCommand():
		err = c.Status(ctx, client)
	case c.vcDirectiveList.FullCommand():
		err = c.ListDirectives(ctx, client)
	case c.vcDirectivePromote.FullCommand():
		err = c.PromoteDirective(ctx, client)
	case c.vcDirectiveSetStatus.FullCommand():
		err = c.SetDirectiveStatus(ctx, client)
	case c.vcDirectiveCreate.FullCommand():
		err = c.CreateDirective(ctx, client)
	case c.vcDirectiveDelete.FullCommand():
		err = c.DeleteDirective(ctx, client)
	case c.vcInstallerList.FullCommand():
		err = c.ListInstallers(ctx, client)
	case c.vcInstallerCreate.FullCommand():
		err = c.CreateInstaller(ctx, client)
	case c.vcInstallerDelete.FullCommand():
		err = c.DeleteInstaller(ctx, client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

func (c *VersionControlCommand) Status(ctx context.Context, client auth.ClientI) error {
	directives, err := client.GetVersionDirectives(ctx, types.VersionDirectiveFilter{})
	if err != nil {
		return trace.Wrap(err)
	}

	installers, err := client.GetVersionControlInstallers(ctx, types.VersionControlInstallerFilter{})
	if err != nil {
		return trace.Wrap(err)
	}

	activeDirectiveStatus := "none"
	if directives.Active != nil {
		activeDirectiveStatus = fmt.Sprintf("\n  Origin: %s\n  Status: %s", directives.Active.Spec.Origin, directives.Active.Spec.Status)
	}

	installersStatus := "none"

	if len(installers.LocalScript) > 0 {
		table := asciitable.MakeTable([]string{"Kind", "Name", "Status"})

		for _, installer := range installers.LocalScript {
			table.AddRow([]string{string(types.InstallerKindLocalScript), installer.GetName(), string(installer.Spec.Status)})
		}

		// sets installer status string as a table indented two spaces
		installersStatus = "\n  " + strings.ReplaceAll(table.AsBuffer().String(), "\n", "\n  ")
	}

	fmt.Printf("Active Directive: %s\n\nInstallers: %s\n", activeDirectiveStatus, installersStatus)

	// TODO(fspmarshall): display a status summary for current inventory versioning, and the
	// number of recent installs/churns/etc.

	return nil
}

func (c *VersionControlCommand) ListDirectives(ctx context.Context, client auth.ClientI) error {
	phaseArg := types.VersionDirectivePhase(c.phase)
	directives, err := client.GetVersionDirectives(ctx, types.VersionDirectiveFilter{
		Phase: phaseArg,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	switch c.format {
	case teleport.Text:

		table := asciitable.MakeTable([]string{"Phase", "Kind", "Name", "Status", "Origin"})

		directives.Iter(func(d types.VersionDirective) {
			origin := "-"
			if o := d.GetOrigin(); o != "" {
				origin = o
			}
			table.AddRow([]string{string(d.GetDirectivePhase()), d.GetSubKind(), d.GetName(), string(d.GetDirectiveStatus()), origin})
		})

		_, err = table.AsBuffer().WriteTo(os.Stdout)
		return trace.Wrap(err)
	case teleport.JSON:
		// use a concrete slice instead of nil to ensure we always
		// display a sequence.
		ds := []types.VersionDirective{}
		directives.Iter(func(d types.VersionDirective) {
			ds = append(ds, d)
		})

		err = utils.WriteJSON(os.Stdout, ds)
		return trace.Wrap(err)

	case teleport.YAML:
		// use a concrete slice instead of nil to ensure we always
		// display a sequence.
		ds := []types.VersionDirective{}
		directives.Iter(func(d types.VersionDirective) {
			ds = append(ds, d)
		})

		err = utils.WriteYAML(os.Stdout, ds)
		return trace.Wrap(err)
	default:
		return trace.BadParameter("unknown format %q, must be one of [%q, %q, %q]", c.format, teleport.Text, teleport.JSON, teleport.YAML)
	}
}

func (c *VersionControlCommand) PromoteDirective(ctx context.Context, client auth.ClientI) error {
	rsp, err := client.PromoteVersionDirective(ctx, proto.PromoteVersionDirectiveRequest{
		Ref: types.VersionDirectiveFilter{
			Phase: types.VersionDirectivePhase(c.from),
			Kind:  c.kind,
			Name:  c.name,
		},
		ToPhase: types.VersionDirectivePhase(c.to),
	})
	if err != nil {
		return trace.Wrap(err)

	}

	// specialize success message depending on wether or not we promoted
	// the directive to a singleton slot or not.
	if rsp.NewRef.Kind == "" || rsp.NewRef.Name == "" {
		fmt.Printf("Successfully promoted %s directive %s/%s to %s.\n", c.from, c.kind, c.name, rsp.NewRef.Phase)
	} else {
		fmt.Printf("Successfully promoted %s directive %s/%s to %s %s/%s.\n", c.from, c.kind, c.name, rsp.NewRef.Phase, rsp.NewRef.Kind, rsp.NewRef.Name)
	}

	return nil
}

func (c *VersionControlCommand) SetDirectiveStatus(ctx context.Context, client auth.ClientI) error {
	rsp, err := client.SetVersionDirectiveStatus(ctx, proto.SetVersionDirectiveStatusRequest{
		Ref: types.VersionDirectiveFilter{
			Phase: types.VersionDirectivePhase(c.phase),
			Kind:  c.kind,
			Name:  c.name,
		},
		Status:  types.VersionDirectiveStatus(c.status),
		Message: c.msg,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if c.kind == "" || c.name == "" {
		fmt.Printf("Successfully set %s directive status to %s (was %s).\n", c.phase, c.status, rsp.PreviousStatus)
	} else {
		fmt.Printf("Successfully set %s directive %s/%s status to %s (was %s).\n", c.phase, c.kind, c.name, c.status, rsp.PreviousStatus)
	}

	return nil
}

func (c *VersionControlCommand) DeleteDirective(ctx context.Context, client auth.ClientI) error {
	err := client.DeleteVersionDirective(ctx, types.VersionDirectiveFilter{
		Phase: types.VersionDirectivePhase(c.phase),
		Kind:  c.kind,
		Name:  c.name,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if c.kind == "" || c.name == "" {
		fmt.Printf("Successfully deleted %s directive.\n", c.phase)
	} else {
		fmt.Printf("Successfully deleted %s directive %s/%s.\n", c.phase, c.kind, c.name)
	}

	return nil
}

func (c *VersionControlCommand) DeleteInstaller(ctx context.Context, client auth.ClientI) error {
	err := client.DeleteVersionControlInstaller(ctx, types.VersionControlInstallerFilter{
		Kind: types.VersionControlInstallerKind(c.kind),
		Name: c.name,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Successfully deleted installer %s/%s.\n", c.kind, c.name)

	return nil
}

func (c *VersionControlCommand) ListInstallers(ctx context.Context, client auth.ClientI) error {
	kind := types.VersionControlInstallerKind(c.kind)
	if kind != types.InstallerKindNone && kind != types.InstallerKindLocalScript {
		return trace.BadParameter("unsupported installer kind %q", kind)
	}
	installers, err := client.GetVersionControlInstallers(ctx, types.VersionControlInstallerFilter{
		Kind: kind,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	switch c.format {
	case teleport.Text:
		table := asciitable.MakeTable([]string{"Kind", "Name", "Status"})

		installers.Iter(func(i types.VersionControlInstaller) {
			table.AddRow([]string{string(i.GetInstallerKind()), i.GetName(), string(i.GetInstallerStatus())})
		})
		_, err = table.AsBuffer().WriteTo(os.Stdout)
		return trace.Wrap(err)
	case teleport.JSON:
		// use a concrete slice instead of nil to ensure we always
		// display a sequence.
		is := []types.VersionControlInstaller{}
		installers.Iter(func(i types.VersionControlInstaller) {
			is = append(is, i)
		})
		err = utils.WriteJSON(os.Stdout, is)
		return trace.Wrap(err)
	case teleport.YAML:
		// use a concrete slice instead of nil to ensure we always
		// display a sequence.
		is := []types.VersionControlInstaller{}
		installers.Iter(func(i types.VersionControlInstaller) {
			is = append(is, i)
		})
		err = utils.WriteYAML(os.Stdout, is)
		return trace.Wrap(err)
	default:
		return trace.BadParameter("unknown format %q, must be one of [%q, %q, %q]", c.format, teleport.Text, teleport.JSON, teleport.YAML)
	}
}

func (c *VersionControlCommand) CreateInstaller(ctx context.Context, client auth.ClientI) error {
	var reader io.Reader
	if c.file == "-" {
		reader = os.Stdin
	} else {
		f, err := utils.OpenFile(c.file)
		if err != nil {
			return trace.Wrap(err)
		}
		defer f.Close()
		reader = f
	}

	decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)

	var installer types.LocalScriptInstallerV1
	if err := decoder.Decode(&installer); err != nil {
		return trace.Wrap(err)
	}

	if c.force {
		installer.Spec.Nonce = math.MaxUint64
	}

	if err := client.UpsertVersionControlInstaller(ctx, &installer); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *VersionControlCommand) CreateDirective(ctx context.Context, client auth.ClientI) error {
	var reader io.Reader
	if c.file == "-" {
		reader = os.Stdin
	} else {
		f, err := utils.OpenFile(c.file)
		if err != nil {
			return trace.Wrap(err)
		}
		defer f.Close()
		reader = f
	}

	decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)

	var directive types.VersionDirectiveV1
	if err := decoder.Decode(&directive); err != nil {
		return trace.Wrap(err)
	}

	if c.force {
		directive.Spec.Nonce = math.MaxUint64
	}

	if err := client.UpsertVersionDirective(ctx, &directive); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
