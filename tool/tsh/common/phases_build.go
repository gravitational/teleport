/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/spinner"
	toolcommon "github.com/gravitational/teleport/tool/common"
)

type phaseAddCommand struct {
	*kingpin.CmdClause
	phaseBuildOptions
	name        string
	parent      string
	fromSet     bool
	forkSource  string
	forkLocal   bool
	forkFromSet bool
}

type phaseEditCommand struct {
	*kingpin.CmdClause
	phaseBuildOptions
	name       string
	editSource string
	fromSet    bool
}

type phaseBuildOptions struct {
	commandsFile string
	copies       []string
	files        []phaseConfigFile
	region       string
	roles        string
	format       string
	console      bool
}

func newPhaseAddCommand(parent *kingpin.CmdClause) *phaseAddCommand {
	cmd := &phaseAddCommand{CmdClause: parent.Command("add", "Create and build a Phase image.")}
	cmd.Alias(phaseAddHelp)
	cmd.Arg("name", "Optional Phase name. If omitted, a name is generated.").StringVar(&cmd.name)
	cmd.Flag("from", "Upstream Phase image to use as the base image.").IsSetByUser(&cmd.fromSet).StringVar(&cmd.parent)
	cmd.Flag("fork", "Capture the current build Beam's filesystem and start a new Phase from it.").BoolVar(&cmd.forkLocal)
	cmd.Flag("roles", "Comma-separated roles to delegate to Phase builds. Omit to inherit your delegation permissions.").
		StringVar(&cmd.roles)
	cmd.Flag("fork-from", "Fork a Phase name or parent@sha256:digest. Copy and rebuild Phases cannot be forked.").
		IsSetByUser(&cmd.forkFromSet).
		StringVar(&cmd.forkSource)
	cmd.registerBuildFlags(cmd.CmdClause).Short('f')
	return cmd
}

func newPhaseEditCommand(parent *kingpin.CmdClause) *phaseEditCommand {
	cmd := &phaseEditCommand{
		CmdClause: parent.Command("edit", "Edit an existing Phase in a build console or replace its build instructions."),
	}
	cmd.Alias(phaseEditHelp)
	cmd.Arg("name", "Existing Phase to edit.").Required().StringVar(&cmd.name)
	cmd.Flag("from", "Snapshot source: Phase name or parent@sha256:digest. Copy targets require --copy.").
		IsSetByUser(&cmd.fromSet).
		StringVar(&cmd.editSource)
	cmd.registerBuildFlags(cmd.CmdClause).Short('f')
	return cmd
}

func (c *phaseBuildOptions) registerBuildFlags(cmd *kingpin.CmdClause) *kingpin.FlagClause {
	cmd.Flag("console",
		"Open an interactive shell or stream command logs. "+
			"Disable to return after provisioning. Copies still finish before returning.").
		Default("true").
		BoolVar(&c.console)
	cmd.Flag("commands-file", "Path to a YAML file in the raw command format. Omit for an interactive build.").
		ExistingFileVar(&c.commandsFile)
	cmd.Flag("copy", "Copy files outside /home using absolute-destination:local-source. Repeat for more paths.").
		StringsVar(&c.copies)
	cmd.Flag("region", "Region in which to build the Phase.").StringVar(&c.region)
	format := cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Default(teleport.Text)
	format.EnumVar(&c.format, defaults.DefaultFormats...)
	return format
}

func (c *phaseAddCommand) run(cf *CLIConf) error {
	req, err := c.buildRequest(c.name)
	if err != nil {
		return trace.Wrap(err)
	}
	source, err := c.snapshotSource(cf.Context, localBuildBeam(cf.Stderr()))
	if err != nil {
		return trace.Wrap(err)
	}
	req.SetParent(c.parent)
	return c.runBuild(cf, req, source)
}

func (c *phaseEditCommand) run(cf *CLIConf) error {
	req, err := c.buildRequest(c.name)
	if err != nil {
		return trace.Wrap(err)
	}
	source, err := c.snapshotSource()
	if err != nil {
		return trace.Wrap(err)
	}
	req.SetReplace(true)
	return c.runBuild(cf, req, source)
}

func (c *phaseBuildOptions) buildRequest(name string) (*beamsv1.CreatePhaseRequest, error) {
	if c.commandsFile != "" && len(c.copies) != 0 {
		return nil, trace.BadParameter("--copy and --commands-file are mutually exclusive")
	}
	var err error
	c.files, err = phaseCopyFiles(c.copies)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := beamsv1.CreatePhaseRequest_builder{
		Name:            name,
		BuildType:       beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_SNAPSHOT,
		OverrideRegion:  c.region,
		StreamBuildLogs: c.commandsFile != "" && c.console,
	}.Build()
	if c.roles != "" {
		req.SetDelegation(beamsv1.DelegationSpec_builder{
			Mode:  beamsv1.DelegationMode_DELEGATION_MODE_SELECTED_ROLES,
			Roles: utils.SplitIdentifiers(c.roles),
		}.Build())
	}
	if c.commandsFile != "" {
		contents, err := readPhaseBuildConfig(c.commandsFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req.SetBuildConfig(string(contents))
		req.SetBuildType(beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_REBUILD)
	} else if len(c.files) != 0 {
		req.SetBuildType(beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_COPY)
	} else if c.console {
		switch strings.ToLower(c.format) {
		case teleport.JSON, teleport.YAML:
			return nil, trace.BadParameter(
				"--format=%s cannot be used with interactive builds. "+
					"Specify --no-console, --commands-file or --copy, or use text format",
				c.format,
			)
		}
	}
	return req, nil
}

func (c *phaseBuildOptions) runBuild(cf *CLIConf, req *beamsv1.CreatePhaseRequest, source string) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	proxyRegion, err := discoverProxyRegion(cf.Context, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	beamRegion := os.Getenv("BEAM_REGION")
	var rsp *beamsv1.CreatePhaseResponse
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()
		if c.shouldShowSpinner(utils.IsTerminal(cf.Stdout())) {
			buildingPhaseSpinner := spinner.New(cf.Stdout(), "Building phase...")
			defer buildingPhaseSpinner.Stop()
		}
		rootClient, err := clusterClient.ConnectToRootCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootClient.Close()
		snapshot, err := resolvePhaseSnapshot(cf.Context, source,
			func(ctx context.Context, name string) (*beamsv1.Phase, error) {
				response, err := rootClient.PhaseServiceClient().GetPhase(ctx, beamsv1.GetPhaseRequest_builder{
					Name: name,
				}.Build())
				if err != nil {
					return nil, trace.Wrap(err)
				}
				source := response.GetPhase()
				if source.GetStatus().GetStrategy() == beamsv1.PhaseStrategy_PHASE_STRATEGY_REBUILD {
					return nil, trace.BadParameter("cannot fork rebuild Phase %q. Use --from to build on its image", name)
				}
				if req.GetReplace() &&
					name == req.GetName() &&
					source.GetStatus().GetStrategy() == beamsv1.PhaseStrategy_PHASE_STRATEGY_COPY {
					return nil, trace.BadParameter("copy Phases must be edited with --copy or a files stage")
				}
				return source, nil
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
		if !req.GetReplace() &&
			snapshot.digest != "" &&
			strings.Contains(snapshot.parent, ":") &&
			snapshot.region == "" &&
			c.region == "" &&
			beamRegion == "" {
			return trace.BadParameter("forking a snapshot of a built-in base image requires --region outside a Beam")
		}
		if snapshot.digest != "" {
			req.SetParent(snapshot.parent)
			req.SetWorkingDigest(snapshot.digest)
		}
		if c.region == "" {
			switch {
			case snapshot.region != "":
				proxyRegion = snapshot.region
			case beamRegion != "":
				// An in-Beam build can provide the snapshot's registry
				// region through the environment.
				proxyRegion = beamRegion
			}
		}
		req.SetProxyRegion(proxyRegion)
		rsp, err = rootClient.PhaseServiceClient().CreatePhase(cf.Context, req)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	phase := rsp.GetPhase()
	name := phase.GetMetadata().GetName()
	copyBuild := req.GetBuildType() == beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_COPY
	if beam := rsp.GetBuildBeam(); beam != nil && (copyBuild || req.GetStreamBuildLogs()) {
		if copyBuild {
			doc := phaseDocument{Files: &c.files}
			doc.Metadata.Name = name
			runner := phaseDocumentClient{
				cf:     cf,
				client: tc,
			}
			if err := runner.build(cf.Context, doc, beam, phase); err != nil {
				return trace.Wrap(err, "copy build Beam %q. Check tsh phase ls for the operation's result",
					beam.GetMetadata().GetName())
			}
		} else {
			output := cf.Stdout()
			if c.format == teleport.JSON || c.format == teleport.YAML {
				output = cf.Stderr()
			}
			originalOutput := tc.Stdout
			tc.Stdout = output
			err := sshBeamWithRetry(cf, tc, beam, []string{"/usr/local/bin/beam-logs"}, false)
			tc.Stdout = originalOutput
			if err != nil {
				return trace.Wrap(err, "build log connection ended; the build continues independently (check tsh phase ls)")
			}
		}
		err = client.RetryWithRelogin(cf.Context, tc, func() error {
			clusterClient, err := tc.ConnectToCluster(cf.Context)
			if err != nil {
				return trace.Wrap(err)
			}
			defer clusterClient.Close()
			rootClient, err := clusterClient.ConnectToRootCluster(cf.Context)
			if err != nil {
				return trace.Wrap(err)
			}
			defer rootClient.Close()
			// The log stream has finished. This deadline only bounds waiting
			// for Auth to record the result, not provisioning or the build.
			ctx, cancel := context.WithTimeout(cf.Context, 2*time.Minute)
			defer cancel()
			phase, err = waitForPhaseOperation(ctx, rsp.GetPhase(), func(ctx context.Context) (*beamsv1.Phase, error) {
				response, err := rootClient.PhaseServiceClient().GetPhase(ctx, beamsv1.GetPhaseRequest_builder{Name: name}.Build())
				return response.GetPhase(), trace.Wrap(err)
			}, copyBuild)
			return trace.Wrap(err, "waiting for Phase %q build result. Check tsh phase ls", name)
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if err := c.printResult(cf.Stdout(), req, phase, rsp.GetBuildBeam()); err != nil {
		return trace.Wrap(err)
	}
	if c.console &&
		req.GetBuildType() == beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_SNAPSHOT &&
		rsp.GetBuildBeam() != nil {
		return trace.Wrap(sshBeam(cf, tc, rsp.GetBuildBeam(), nil))
	}
	return nil
}

func (c *phaseBuildOptions) printResult(
	w io.Writer,
	req *beamsv1.CreatePhaseRequest,
	phase *beamsv1.Phase,
	beam *beamsv1.Beam,
) error {
	formatted := formatPhase(phase)
	if !c.console && beam != nil &&
		phase.GetStatus().GetState() == beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BUILDING {
		formatted.BuildBeam = beam.GetMetadata().GetName()
	}
	switch strings.ToLower(c.format) {
	case teleport.JSON:
		return trace.Wrap(toolcommon.PrintJSONIndent(w, formatted))
	case teleport.YAML:
		return trace.Wrap(toolcommon.PrintYAML(w, formatted))
	default:
		name := phase.GetMetadata().GetName()
		interactive := req.GetBuildType() == beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_SNAPSHOT
		if !c.console &&
			phase.GetStatus().GetState() == beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BUILDING {
			if _, err := fmt.Fprintf(w, "Phase %q build started.\n", name); err != nil {
				return trace.Wrap(err)
			}
			if beam != nil {
				if _, err := fmt.Fprintf(w, "Build Beam: %s\n", beam.GetMetadata().GetName()); err != nil {
					return trace.Wrap(err)
				}
				if interactive {
					_, err := fmt.Fprintf(w, "Connect to continue this build: tsh beams ssh %s\n", beam.GetMetadata().GetName())
					return trace.Wrap(err)
				}
			}
			_, err := fmt.Fprintln(w, "Check build status: tsh phases ls")
			return trace.Wrap(err)
		}
		action := "created"
		if req.GetReplace() {
			action = "edit started"
			if !interactive {
				action = "updated"
			}
		}
		if _, err := fmt.Fprintf(w, "Phase %q %s.\n", name, action); err != nil {
			return trace.Wrap(err)
		}
		if beam != nil && interactive {
			_, err := fmt.Fprintln(w,
				"Save your work at any time with \"tsh phase snapshot\".\n"+
					"Use \"tsh phase snapshot --final\" to save and end this build.")
			return trace.Wrap(err)
		}
		if phase.GetStatus().GetState() == beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BLOCKED {
			_, err := fmt.Fprintf(w, "Phase %q was built and is blocked by upstream Phase %q.\n",
				name, phase.GetSpec().GetParent())
			return trace.Wrap(err)
		}
		_, err := fmt.Fprintf(w, "Phase %q is ready.\n", name)
		return trace.Wrap(err)
	}
}

func waitForPhaseOperation(
	ctx context.Context,
	expected *beamsv1.Phase,
	get func(context.Context) (*beamsv1.Phase, error),
	cleanup bool,
) (*beamsv1.Phase, error) {
	return waitForPhaseObservation(ctx, expected, get, func(phase *beamsv1.Phase) (bool, error) {
		state := phase.GetStatus().GetState()
		return (state == beamsv1.PhaseBuildState_PHASE_BUILD_STATE_READY ||
			state == beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BLOCKED) &&
			(!cleanup || phase.GetStatus().GetOperation().GetBuildBeam() == ""), phaseBuildError(phase)
	})
}

func phaseBuildError(phase *beamsv1.Phase) error {
	if phase.GetStatus().GetState() == beamsv1.PhaseBuildState_PHASE_BUILD_STATE_FAILED {
		return trace.Errorf("Phase %q build failed: %s", phase.GetMetadata().GetName(), phase.GetStatus().GetLastError())
	}
	return nil
}

func waitForPhaseObservation(
	ctx context.Context,
	expected *beamsv1.Phase,
	get func(context.Context) (*beamsv1.Phase, error),
	ready func(*beamsv1.Phase) (bool, error),
) (*beamsv1.Phase, error) {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  time.Second,
		Step:   time.Second,
		Max:    5 * time.Second,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	consistencyDeadline := time.Now().Add(2 * time.Minute)
	observed := false
	operationID := expected.GetStatus().GetOperation().GetId()
	for {
		phase, err := get(ctx)
		if err != nil && (!trace.IsNotFound(err) || observed) {
			return nil, trace.Wrap(err)
		}
		// Public reads may still show a previous build or a deleted incarnation
		// of this name. Match the operation returned by CreatePhase, including
		// its published result after cleanup clears the active operation.
		state := phase.GetStatus().GetState()
		completed := (state == beamsv1.PhaseBuildState_PHASE_BUILD_STATE_READY ||
			state == beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BLOCKED) &&
			phase.GetStatus().GetPublished().GetOperationId() == operationID
		if err == nil &&
			operationID != "" &&
			phase.GetStatus().GetId() == expected.GetStatus().GetId() &&
			(phase.GetStatus().GetOperation().GetId() == operationID || completed) {
			observed = true
			consistencyDeadline = time.Now().Add(2 * time.Minute)
			if done, err := ready(phase); done || err != nil {
				return phase, trace.Wrap(err)
			}
		} else if !time.Now().Before(consistencyDeadline) {
			return nil, trace.Wrap(context.DeadlineExceeded, "waiting for the expected Phase build to appear, check tsh phase ls")
		}
		select {
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-retry.After():
			retry.Inc()
		}
	}
}

func (c *phaseBuildOptions) shouldShowSpinner(terminal bool) bool {
	switch strings.ToLower(c.format) {
	case teleport.JSON, teleport.YAML:
		return false
	default:
		return terminal
	}
}

func readPhaseBuildConfig(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, services.MaxPhaseBuildConfigSize+1))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(contents) > services.MaxPhaseBuildConfigSize {
		return nil, trace.BadParameter("commands file exceeds maximum size of %d bytes", services.MaxPhaseBuildConfigSize)
	}
	if len(bytes.TrimSpace(contents)) == 0 {
		return nil, trace.BadParameter("commands file is empty")
	}
	return contents, nil
}
