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
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"go.yaml.in/yaml/v3"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/spinner"
	toolcommon "github.com/gravitational/teleport/tool/common"
)

const (
	phaseServiceTypePreStart    = "pre-start"
	phaseServiceTypeBackground  = "background"
	phaseServiceContextBuild    = "build"
	phaseServiceContextRebuild  = "rebuild"
	phaseServiceContextSnapshot = "snapshot"
	phaseServiceContextRuntime  = "runtime"
	phaseServiceContextUser     = "user"
	phaseServiceContextAll      = "all"
	phaseBootstrapServiceName   = "bootstrap"
	phaseAssembleServiceName    = "beam-assemble"
)

type phaseApplyCommand struct {
	*kingpin.CmdClause
	configFile string
	region     string
	format     string
	console    bool
}

func newPhaseApplyCommand(parent *kingpin.CmdClause) *phaseApplyCommand {
	cmd := &phaseApplyCommand{
		CmdClause: parent.Command("apply", "Create or update Phases from a multi-document YAML configuration."),
	}
	cmd.Alias(phaseApplyHelp)
	cmd.Flag("console", "Stream command logs while applying. All stages still finish before returning.").
		Default("true").
		BoolVar(&cmd.console)
	cmd.Flag("file", "Path to a multi-document YAML Phase configuration.").
		Short('f').
		Required().
		ExistingFileVar(&cmd.configFile)
	cmd.Flag("region", "Region in which to build the Phases.").StringVar(&cmd.region)
	cmd.Flag("format", defaults.FormatFlagDescription(defaults.DefaultFormats...)).
		Default(teleport.Text).
		EnumVar(&cmd.format, defaults.DefaultFormats...)
	return cmd
}

type phaseDocument struct {
	Version  string `yaml:"version"`
	Metadata struct {
		Name             string   `yaml:"name"`
		From             string   `yaml:"from"`
		RebuildHomePaths []string `yaml:"rebuild_home_paths"`
	} `yaml:"metadata"`
	Commands *[]phaseConfigCommand `yaml:"commands"`
	Files    *[]phaseConfigFile    `yaml:"files"`
	Services []phaseConfigService  `yaml:"services"`
}

type phaseConfigFile struct {
	Comment string `yaml:"comment"`
	From    string `yaml:"from"`
	To      string `yaml:"to"`
}

type phaseConfigCommand struct {
	Comment string           `yaml:"comment" json:"comment,omitempty"`
	Command string           `yaml:"command" json:"command"`
	Args    []string         `yaml:"args" json:"args,omitempty"`
	Shell   string           `yaml:"shell" json:"-"`
	Env     []phaseConfigEnv `yaml:"env" json:"env,omitempty"`
	Workdir string           `yaml:"workdir" json:"workdir,omitempty"`
}

type phaseConfigEnv struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

type phaseConfigService struct {
	phaseConfigCommand `yaml:",inline"`
	Name               string `yaml:"name" json:"name"`
	Type               string `yaml:"type" json:"type,omitempty"`
	Context            string `yaml:"context" json:"context,omitempty"`
}

type phaseRawBuildConfig struct {
	ProtectedPaths []string             `json:"protected_paths,omitempty"`
	Commands       []phaseConfigCommand `json:"commands,omitempty"`
	Services       []phaseConfigService `json:"services,omitempty"`
}

func (doc phaseDocument) strategy() beamsv1.PhaseStrategy {
	if doc.Commands != nil {
		return beamsv1.PhaseStrategy_PHASE_STRATEGY_REBUILD
	}
	return beamsv1.PhaseStrategy_PHASE_STRATEGY_COPY
}

func readPhaseDocuments(filename string) ([]phaseDocument, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(data) > 4<<20 {
		return nil, trace.BadParameter("Phase configuration exceeds 4 MiB")
	}
	return parsePhaseDocuments(data, filepath.Dir(filename))
}

func parsePhaseDocuments(data []byte, dir string) ([]phaseDocument, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var docs []phaseDocument
	for {
		var doc phaseDocument
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, trace.Wrap(err, "document %d", len(docs)+1)
		}
		docs = append(docs, doc)
	}
	if err := validatePhaseDocuments(docs, dir); err != nil {
		return nil, trace.Wrap(err)
	}
	return docs, nil
}

func validatePhaseDocuments(docs []phaseDocument, dir string) error {
	if len(docs) == 0 {
		return trace.BadParameter("Phase configuration contains no documents")
	}
	names := make(map[string]int)
	for i := range docs {
		doc := &docs[i]
		if doc.Version != "v1" {
			return trace.BadParameter("document %d: version must be v1", i+1)
		}
		if doc.Metadata.Name == "" {
			return trace.BadParameter("document %d: metadata.name is required", i+1)
		}
		if _, found := names[doc.Metadata.Name]; found {
			return trace.BadParameter("duplicate Phase name %q", doc.Metadata.Name)
		}
		if err := doc.validate(dir); err != nil {
			return trace.Wrap(err, "Phase %q", doc.Metadata.Name)
		}
		names[doc.Metadata.Name] = i
	}
	for i, doc := range docs {
		if parent, found := names[doc.Metadata.From]; found && parent >= i {
			return trace.BadParameter("Phase %q must appear after its parent %q", doc.Metadata.Name, doc.Metadata.From)
		}
		if i > 0 && doc.Metadata.From != docs[i-1].Metadata.Name {
			return trace.BadParameter("Phase %q must use the preceding Phase %q as its parent",
				doc.Metadata.Name, docs[i-1].Metadata.Name)
		}
	}
	return nil
}

var phaseServiceName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func (doc *phaseDocument) validate(dir string) error {
	if len(validation.IsDNS1123Label(doc.Metadata.Name)) != 0 ||
		len(doc.Metadata.Name) > services.MaxPhaseNameLength {
		return trace.BadParameter("invalid Phase name %q", doc.Metadata.Name)
	}
	if parent := doc.Metadata.From; parent != "" &&
		parent != services.DefaultPhaseName &&
		len(validation.IsDNS1123Label(parent)) != 0 {
		return trace.BadParameter("invalid parent Phase name %q", parent)
	}
	if doc.Commands != nil && doc.Files != nil {
		return trace.BadParameter("commands and files cannot be combined")
	}
	if doc.Commands == nil && doc.Files == nil {
		return trace.BadParameter("commands or files is required")
	}
	if doc.Commands != nil {
		if len(*doc.Commands) == 0 {
			return trace.BadParameter("commands must not be empty")
		}
		for i := range *doc.Commands {
			if err := (*doc.Commands)[i].normalize(); err != nil {
				return trace.Wrap(err, "commands[%d]", i)
			}
		}
	}
	if doc.Commands == nil && len(doc.Metadata.RebuildHomePaths) != 0 {
		return trace.BadParameter("rebuild_home_paths requires commands")
	}
	for _, name := range doc.Metadata.RebuildHomePaths {
		if !path.IsAbs(name) ||
			path.Clean(name) != name ||
			strings.ContainsAny(name, "\x00*?[]") {
			return trace.BadParameter("rebuild_home_paths entries must be clean absolute paths without patterns")
		}
	}
	if doc.Files != nil {
		if err := validatePhaseFiles(*doc.Files, dir); err != nil {
			return trace.Wrap(err)
		}
	}
	for i := range doc.Services {
		service := &doc.Services[i]
		if !phaseServiceName.MatchString(service.Name) ||
			len(service.Name) > 128 ||
			service.Name == phaseBootstrapServiceName ||
			service.Name == phaseAssembleServiceName {
			return trace.BadParameter("services[%d]: invalid service name", i)
		}
		if service.Type != "" &&
			service.Type != phaseServiceTypeBackground &&
			service.Type != phaseServiceTypePreStart {
			return trace.BadParameter("services[%d]: type must be background or pre-start", i)
		}
		switch service.Context {
		case "":
			service.Context = phaseServiceContextUser
		case phaseServiceContextRebuild,
			phaseServiceContextSnapshot,
			phaseServiceContextRuntime,
			phaseServiceContextBuild,
			phaseServiceContextUser,
			phaseServiceContextAll:
		default:
			return trace.BadParameter("services[%d]: context must be rebuild, snapshot, runtime, build, user, or all", i)
		}
		if err := service.normalize(); err != nil {
			return trace.Wrap(err, "services[%d]", i)
		}
	}
	raw := phaseRawBuildConfig{
		ProtectedPaths: doc.Metadata.RebuildHomePaths,
		Services:       doc.Services,
	}
	if doc.Commands != nil {
		raw.Commands = *doc.Commands
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(data) > services.MaxPhaseBuildConfigSize {
		return trace.BadParameter("build configuration exceeds %d bytes", services.MaxPhaseBuildConfigSize)
	}
	return nil
}

func phaseCopyFiles(copies []string) ([]phaseConfigFile, error) {
	files := make([]phaseConfigFile, 0, len(copies))
	for _, value := range copies {
		to, from, ok := strings.Cut(value, ":")
		if !ok || to == "" || from == "" {
			return nil, trace.BadParameter("--copy requires absolute-destination:local-source, got %q", value)
		}
		files = append(files, phaseConfigFile{
			From: from,
			To:   to,
		})
	}
	return files, trace.Wrap(validatePhaseFiles(files, "."))
}

func validatePhaseFiles(files []phaseConfigFile, dir string) error {
	for i := range files {
		f := &files[i]
		if f.From == "" || f.To == "" {
			return trace.BadParameter("files[%d]: from and to are required", i)
		}
		if !path.IsAbs(f.To) || strings.ContainsRune(f.To, 0) {
			return trace.BadParameter("files[%d]: to must be an absolute path", i)
		}
		dirTarget := strings.HasSuffix(f.To, "/")
		f.To = path.Clean(f.To)
		if f.To == "/home" || strings.HasPrefix(f.To, "/home/") {
			return trace.BadParameter("files[%d]: to must be outside /home", i)
		}
		if !filepath.IsAbs(f.From) {
			f.From = filepath.Join(dir, f.From)
		}
		info, err := os.Lstat(f.From)
		if err != nil {
			return trace.Wrap(err)
		}
		if dirTarget && !info.IsDir() {
			f.To = path.Join(f.To, filepath.Base(f.From))
		}
		if err := filepath.WalkDir(f.From, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() &&
				!entry.Type().IsRegular() &&
				entry.Type()&os.ModeSymlink == 0 {
				return trace.BadParameter("unsupported source file %q", name)
			}
			return nil
		}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (cmd *phaseConfigCommand) normalize() error {
	if cmd.Shell != "" {
		if cmd.Command != "" || len(cmd.Args) != 0 {
			return trace.BadParameter("shell cannot be combined with command or args")
		}
		cmd.Command, cmd.Args = "/bin/sh", []string{"-c", cmd.Shell}
		cmd.Shell = ""
	}
	if cmd.Command == "" {
		return trace.BadParameter("command or shell is required")
	}
	for _, value := range append([]string{cmd.Command, cmd.Workdir}, cmd.Args...) {
		if strings.ContainsRune(value, 0) {
			return trace.BadParameter("commands cannot contain NUL bytes")
		}
	}
	if cmd.Workdir != "" &&
		(!path.IsAbs(cmd.Workdir) ||
			path.Clean(cmd.Workdir) != cmd.Workdir) {
		return trace.BadParameter("workdir must be a clean absolute path")
	}
	seen := make(map[string]bool)
	for _, env := range cmd.Env {
		if env.Name == "" ||
			strings.ContainsAny(env.Name, "=\x00") ||
			strings.ContainsRune(env.Value, 0) ||
			seen[env.Name] {
			return trace.BadParameter("invalid or duplicated environment variable %q", env.Name)
		}
		switch env.Name {
		case "BEAM_BUILD_CONFIG_FILE",
			"BEAM_PARENT_PHASE_NAME",
			"BEAM_PHASE_NAME",
			"BEAM_WORKING_DIGEST",
			"BEAM_STREAM_BUILD_LOGS",
			"BEAM_BUILD_TYPE":
			return trace.BadParameter("environment variable %q is reserved for builds", env.Name)
		}
		seen[env.Name] = true
	}
	return nil
}

func (c *phaseApplyCommand) run(cf *CLIConf) error {
	docs, err := readPhaseDocuments(c.configFile)
	if err != nil {
		return trace.Wrap(err)
	}
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	proxyRegion, err := discoverProxyRegion(cf.Context, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	runner := &phaseDocumentClient{
		cf:          cf,
		client:      tc,
		proxyRegion: proxyRegion,
		region:      c.region,
		stages:      len(docs),
		console:     c.console,
	}
	phases, err := applyPhaseDocuments(cf.Context, docs, runner)
	if err != nil {
		return trace.Wrap(err)
	}
	formatted := make([]formattedPhase, 0, len(phases))
	for _, phase := range phases {
		formatted = append(formatted, formatPhase(phase))
	}
	switch c.format {
	case teleport.JSON:
		return trace.Wrap(toolcommon.PrintJSONIndent(cf.Stdout(), formatted))
	case teleport.YAML:
		return trace.Wrap(toolcommon.PrintYAML(cf.Stdout(), formatted))
	default:
		_, err := fmt.Fprintf(cf.Stdout(), "Applied %d Phases.\n", len(phases))
		return trace.Wrap(err)
	}
}

type phaseDocumentRunner interface {
	get(context.Context, string) (*beamsv1.Phase, error)
	apply(context.Context, phaseDocument, *beamsv1.Phase, []string) (*beamsv1.Phase, error)
}

func applyPhaseDocuments(ctx context.Context, docs []phaseDocument, runner phaseDocumentRunner) ([]*beamsv1.Phase, error) {
	existing := make([]*beamsv1.Phase, len(docs))
	for i, doc := range docs {
		current, err := runner.get(ctx, doc.Metadata.Name)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if err == nil {
			// Copy builds remain static until their first snapshot succeeds.
			// Retry a failed initial copy without allowing other strategy changes.
			status := current.GetStatus()
			retryCopy := doc.Files != nil &&
				status.GetStrategy() == beamsv1.PhaseStrategy_PHASE_STRATEGY_STATIC &&
				status.GetState() == beamsv1.PhaseBuildState_PHASE_BUILD_STATE_FAILED &&
				status.GetPublished().GetDigest() == "" &&
				status.GetOperation().GetBuildType() == beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_COPY
			if status.GetStrategy() != doc.strategy() && !retryCopy {
				return nil, trace.BadParameter("Phase %q already exists with a different strategy", doc.Metadata.Name)
			}
			parent := doc.Metadata.From
			if parent == "" {
				parent = services.DefaultPhaseName
			}
			if current.GetSpec().GetParent() != parent {
				return nil, trace.BadParameter("Phase %q already exists with a different parent", doc.Metadata.Name)
			}
			existing[i] = current
		}
	}
	var phases []*beamsv1.Phase
	for i, doc := range docs {
		if existing[i] != nil {
			// Send the parent validated above instead of letting replacement
			// inherit a different parent if this name is deleted and recreated.
			doc.Metadata.From = existing[i].GetSpec().GetParent()
		}
		var suppress []string
		if i+1 < len(docs) && existing[i+1] != nil {
			suppress = []string{existing[i+1].GetStatus().GetId()}
		}
		phase, err := runner.apply(ctx, doc, existing[i], suppress)
		if err != nil {
			return nil, trace.Wrap(err, "Phase %q failed. Completed stages are preserved", doc.Metadata.Name)
		}
		phases = append(phases, phase)
	}
	return phases, nil
}

type phaseDocumentClient struct {
	cf          *CLIConf
	client      *client.TeleportClient
	proxyRegion string
	region      string
	stage       int
	stages      int
	console     bool
}

func (runner *phaseDocumentClient) get(ctx context.Context, name string) (*beamsv1.Phase, error) {
	var phase *beamsv1.Phase
	err := withRootCluster(ctx, runner.client, func(root authclient.ClientI, _ string) error {
		resp, err := root.PhaseServiceClient().GetPhase(ctx, beamsv1.GetPhaseRequest_builder{Name: name}.Build())
		phase = resp.GetPhase()
		return trace.Wrap(err)
	})
	return phase, trace.Wrap(err)
}

func (runner *phaseDocumentClient) apply(
	ctx context.Context, doc phaseDocument, existing *beamsv1.Phase, suppress []string,
) (*beamsv1.Phase, error) {
	runner.stage++
	fmt.Fprintf(runner.cf.Stderr(), "\nStage %d/%d: %s Phase %q\n",
		runner.stage, runner.stages, phaseStrategyString(doc.strategy()), doc.Metadata.Name)
	buildConfig := ""
	buildType := beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_COPY
	if doc.Commands != nil {
		buildType = beamsv1.PhaseBuildType_PHASE_BUILD_TYPE_REBUILD
		data, err := json.Marshal(phaseRawBuildConfig{
			Commands:       *doc.Commands,
			ProtectedPaths: doc.Metadata.RebuildHomePaths,
			Services:       doc.Services,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		buildConfig = string(data)
	}
	var (
		resp *beamsv1.CreatePhaseResponse
		err  error
	)
	req := beamsv1.CreatePhaseRequest_builder{
		BuildType:        buildType,
		Name:             doc.Metadata.Name,
		Parent:           doc.Metadata.From,
		BuildConfig:      buildConfig,
		Replace:          existing != nil,
		ProxyRegion:      runner.proxyRegion,
		OverrideRegion:   runner.region,
		StreamBuildLogs:  doc.Commands != nil && runner.console,
		SuppressRebuilds: suppress,
	}.Build()
	var progress *spinner.Spinner
	if utils.IsTerminal(runner.cf.Stderr()) {
		progress = spinner.New(runner.cf.Stderr(), "Starting build...")
		defer progress.Stop()
	}
	// Retry after an earlier operation finishes, with at most three creation
	// attempts so concurrent changes cannot keep restarting this stage forever.
	for range 3 {
		err = withRootCluster(ctx, runner.client, func(root authclient.ClientI, _ string) error {
			resp, err = root.PhaseServiceClient().CreatePhase(ctx, req)
			return trace.Wrap(err)
		})
		if existing == nil || !trace.IsCompareFailed(err) {
			break
		}
		current, readErr := runner.get(ctx, doc.Metadata.Name)
		if readErr != nil {
			return nil, trace.Wrap(readErr)
		}
		if current.GetStatus().GetId() != existing.GetStatus().GetId() {
			return nil, trace.CompareFailed("Phase %q was recreated while applying configuration", doc.Metadata.Name)
		}
		ready, waitErr := phaseReadyForReplacement(current)
		if waitErr != nil {
			return nil, trace.Wrap(waitErr)
		}
		if ready {
			continue
		}
		fmt.Fprintf(runner.cf.Stderr(), "Waiting for the current operation on %q...\n", doc.Metadata.Name)
		if _, err := waitForPhaseObservation(ctx, current, func(ctx context.Context) (*beamsv1.Phase, error) {
			return runner.get(ctx, doc.Metadata.Name)
		}, phaseReadyForReplacement); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err != nil {
		return nil, trace.Wrap(err, "check tsh phase ls before retrying if the creation result is uncertain")
	}
	beam := resp.GetBuildBeam()
	if beam == nil {
		return nil, trace.BadParameter("Phase creation did not return a build Beam")
	}
	phase := resp.GetPhase()
	if progress != nil {
		progress.Stop()
	}
	if err := runner.build(ctx, doc, beam, phase); err != nil {
		return nil, trace.Wrap(err, "build Beam %q. Check tsh phase ls for the operation's result", beam.GetMetadata().GetName())
	}
	if doc.Commands == nil || runner.console {
		// Build logs or snapshotting have finished. Bound the remaining wait
		// for Auth to record the result and remove the build Beam.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
	} else if utils.IsTerminal(runner.cf.Stderr()) {
		progress := spinner.New(runner.cf.Stderr(), "Waiting for build to finish...")
		defer progress.Stop()
	}
	return waitForPhaseOperation(ctx, phase, func(ctx context.Context) (*beamsv1.Phase, error) {
		return runner.get(ctx, doc.Metadata.Name)
	}, true)
}

func (runner *phaseDocumentClient) exec(beam *beamsv1.Beam, r io.Reader, w io.Writer, cmd ...string) error {
	cf := *runner.cf
	cf.overrideStdin = r
	oldInput, oldLogin := runner.client.Stdin, runner.client.HostLogin
	oldOutput, oldError := runner.client.Stdout, runner.client.Stderr
	defer func() {
		runner.client.Stdin, runner.client.HostLogin = oldInput, oldLogin
		runner.client.Stdout, runner.client.Stderr = oldOutput, oldError
	}()
	runner.client.SetExitStatus(0)
	runner.client.Stdout, runner.client.Stderr = w, cf.Stderr()
	escaped := make([]string, len(cmd))
	for i, arg := range cmd {
		escaped[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return sshBeamWithRetry(&cf, runner.client, beam, escaped, false)
}

func (runner *phaseDocumentClient) build(ctx context.Context, doc phaseDocument, beam *beamsv1.Beam, phase *beamsv1.Phase) error {
	if doc.Commands != nil {
		if !runner.console {
			return nil
		}
		return runner.exec(beam, strings.NewReader(""), runner.cf.Stderr(), "/usr/local/bin/beam-logs")
	}
	var progress *spinner.Spinner
	if utils.IsTerminal(runner.cf.Stderr()) {
		progress = spinner.New(runner.cf.Stderr(), "Copying files and saving snapshot...")
		defer progress.Stop()
	}
	if len(*doc.Files) != 0 {
		r, w := io.Pipe()
		written := make(chan error, 1)
		go func() {
			err := writePhaseFiles(w, *doc.Files)
			_ = w.CloseWithError(err)
			written <- err
		}()
		err := runner.exec(beam, r, runner.cf.Stderr(), "sudo", "-n", "tar", "-xmpf", "-", "--no-same-owner", "-C", "/")
		_ = r.CloseWithError(err)
		writeErr := <-written
		if err := trace.NewAggregate(err, writeErr); err != nil {
			return err
		}
	}
	if len(doc.Services) != 0 {
		serviceConfig, err := json.Marshal(phaseRawBuildConfig{Services: doc.Services})
		if err != nil {
			return trace.Wrap(err)
		}
		if err := runner.exec(beam, bytes.NewReader(serviceConfig), runner.cf.Stderr(),
			"sudo", "-n", "/usr/local/bin/beam-config"); err != nil {
			return trace.Wrap(err)
		}
	}
	current, err := waitForPhaseObservation(ctx, phase, func(ctx context.Context) (*beamsv1.Phase, error) {
		return runner.get(ctx, doc.Metadata.Name)
	}, func(phase *beamsv1.Phase) (bool, error) {
		return true, phaseBuildError(phase)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	var snapshot bytes.Buffer
	if err := runner.exec(beam, strings.NewReader(""), &snapshot,
		"/usr/local/bin/beam-snapshot", "--mode="+phaseSnapshotModeProtected); err != nil {
		return trace.Wrap(err)
	}
	parent, digest, err := services.ParsePhaseSnapshot(strings.TrimSpace(snapshot.String()))
	if err != nil {
		return trace.Wrap(err)
	}
	publication := newPhasePublication(current, parent, digest)
	publication.beamID = beam.GetMetadata().GetName()
	err = withRootCluster(ctx, runner.client, func(root authclient.ClientI, _ string) error {
		_, err := publication.publish(ctx, root.PhaseServiceClient())
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if progress != nil {
		progress.Stop()
	}
	fmt.Fprintln(runner.cf.Stderr(), "Snapshot saved. Finalizing copy phase...")
	if err := runner.exec(beam, strings.NewReader(""), io.Discard, "/usr/local/bin/beam-abandon"); err != nil {
		cleanupCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if _, cleanupErr := waitForPhaseOperation(cleanupCtx, phase, func(ctx context.Context) (*beamsv1.Phase, error) {
			return runner.get(ctx, doc.Metadata.Name)
		}, true); cleanupErr != nil {
			return trace.Wrap(err, "snapshot is saved, but build Beam cleanup could not be confirmed")
		}
	}
	return nil
}

func phaseReadyForReplacement(phase *beamsv1.Phase) (bool, error) {
	if phase.GetStatus().GetState() == beamsv1.PhaseBuildState_PHASE_BUILD_STATE_DELETING {
		return false, trace.CompareFailed("Phase %q is being deleted", phase.GetMetadata().GetName())
	}
	return phase.GetStatus().GetState() != beamsv1.PhaseBuildState_PHASE_BUILD_STATE_BUILDING &&
		phase.GetStatus().GetOperation().GetBuildBeam() == "", nil
}

func writePhaseFiles(out io.Writer, files []phaseConfigFile) error {
	w := tar.NewWriter(out)
	for _, f := range files {
		err := filepath.WalkDir(f.From, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			link := ""
			if info.Mode()&os.ModeSymlink != 0 {
				link, err = os.Readlink(name)
				if err != nil {
					return err
				}
			}
			h, err := tar.FileInfoHeader(info, link)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(f.From, name)
			if err != nil {
				return err
			}
			h.Name = strings.TrimPrefix(path.Join(f.To, filepath.ToSlash(rel)), "/")
			if h.Name == "" {
				return nil
			}
			h.Uid, h.Gid, h.Uname, h.Gname = 0, 0, "", ""
			if err := w.WriteHeader(h); err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			f, err := os.Open(name)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, f)
			closeErr := f.Close()
			return trace.NewAggregate(err, closeErr)
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(w.Close())
}
