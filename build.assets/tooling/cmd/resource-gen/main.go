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

// Command resource-gen generates resource service boilerplate from proto options.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/generators"
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/parser"
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

// Config defines command-line inputs for the generator.
type Config struct {
	ProtoDir   string
	OutputDir  string
	Module     string
	WebDir     string
	DryRun     bool
	EventsOnly bool
}

func main() {
	cfg, err := parseFlags(flag.CommandLine, os.Args[1:])
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}

	if err := runWithWriter(cfg, os.Stdout); err != nil {
		log.Fatalf("resource-gen failed: %v", err)
	}
}

func parseFlags(fs *flag.FlagSet, args []string) (Config, error) {
	if fs == nil {
		return Config{}, trace.BadParameter("flag set is required")
	}

	fs.SetOutput(io.Discard)

	protoDir := fs.String("proto-dir", "", "Proto directory to scan for resource service definitions")
	outputDir := fs.String("output-dir", ".", "Output directory for generated files")
	module := fs.String("module", "", "Go module path for generated imports")
	dryRun := fs.Bool("dry-run", false, "Print planned changes without writing files")
	eventsOnly := fs.Bool("events-only", false, "Only inject event messages into events.proto (no Go file generation)")
	webDir := fs.String("web-dir", "", "Web directory for generated TypeScript audit event file (empty = skip)")
	if err := fs.Parse(args); err != nil {
		return Config{}, trace.Wrap(err)
	}
	if *protoDir == "" {
		return Config{}, trace.BadParameter("--proto-dir is required")
	}
	if !*eventsOnly && *module == "" {
		return Config{}, trace.BadParameter("--module is required")
	}

	return Config{
		ProtoDir:   *protoDir,
		OutputDir:  *outputDir,
		Module:     *module,
		WebDir:     *webDir,
		DryRun:     *dryRun,
		EventsOnly: *eventsOnly,
	}, nil
}

func run(cfg Config) error {
	return runWithWriter(cfg, io.Discard)
}

func runWithWriter(cfg Config, out io.Writer) error {
	if cfg.ProtoDir == "" {
		return trace.BadParameter("proto dir is required")
	}
	if !cfg.EventsOnly && cfg.Module == "" {
		return trace.BadParameter("module is required")
	}
	if out == nil {
		return trace.BadParameter("output writer is required")
	}

	ctx := context.Background()
	specs, err := parser.ParseProtoDir(ctx, cfg.ProtoDir)
	if err != nil {
		return trace.Wrap(err)
	}

	// --events-only: inject event messages into events.proto and watch
	// entries into event.proto, then stop.
	if cfg.EventsOnly {
		// Repo root is the parent of the proto dir (e.g., api/proto -> repo root).
		// buf export needs to run from the repo root where buf.yaml lives.
		repoRoot := filepath.Join(cfg.ProtoDir, "..", "..")
		changed, err := generators.InjectEventsProto(ctx, cfg.ProtoDir, repoRoot, specs)
		if err != nil {
			return trace.Wrap(err)
		}
		if changed {
			fmt.Fprintln(out, "events.proto: injected new event messages")
		}
		watchChanged, err := generators.InjectEventProtoWatch(ctx, cfg.ProtoDir, repoRoot, specs)
		if err != nil {
			return trace.Wrap(err)
		}
		if watchChanged {
			fmt.Fprintln(out, "event.proto: injected new watch entries")
		}
		return nil
	}

	files, err := generateFiles(specs, cfg.Module)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, f := range files {
		relPath := filepath.ToSlash(f.Path)
		target := filepath.Join(cfg.OutputDir, f.Path)

		if cfg.DryRun {
			label := relPath
			if f.SkipIfExists {
				label += " (scaffold)"
			}
			if _, err := fmt.Fprintln(out, label); err != nil {
				return trace.Wrap(err)
			}
			continue
		}

		if f.SkipIfExists {
			if _, err := os.Stat(target); err == nil {
				continue // scaffold already exists, don't overwrite
			}
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return trace.Wrap(err)
		}
		if err := os.WriteFile(target, []byte(f.Content), 0o644); err != nil {
			return trace.Wrap(err)
		}
	}

	// Generate web UI audit event TypeScript if --web-dir is set.
	if cfg.WebDir != "" {
		tsContent, err := generators.GenerateWebEventsTS(specs)
		if err != nil {
			return trace.Wrap(err, "generating web events TypeScript")
		}
		tsPath := filepath.Join(cfg.WebDir, "packages", "teleport", "src", "services", "audit", "generatedResourceEvents.gen.ts")
		target := filepath.Join(cfg.OutputDir, tsPath)
		if cfg.DryRun {
			fmt.Fprintln(out, tsPath)
		} else {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return trace.Wrap(err)
			}
			if err := os.WriteFile(target, []byte(tsContent), 0o644); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

const generatedHeader = "// Code generated by resource-gen; DO NOT EDIT.\n\n"

type generatedFile struct {
	Path         string
	Content      string
	SkipIfExists bool
}

func generateFiles(specs []spec.ResourceSpec, module string) ([]generatedFile, error) {
	// Validate cross-resource uniqueness of audit code prefixes.
	prefixOwner := map[string]string{} // code_prefix -> kind
	for _, rs := range specs {
		if rs.Audit.CodePrefix == "" {
			continue
		}
		if owner, exists := prefixOwner[rs.Audit.CodePrefix]; exists {
			return nil, trace.BadParameter("duplicate audit code_prefix %q: used by both %q and %q", rs.Audit.CodePrefix, owner, rs.Kind)
		}
		prefixOwner[rs.Audit.CodePrefix] = rs.Kind
	}

	gens := generators.Generators()
	files := make([]generatedFile, 0, len(specs)*len(gens))
	for _, rs := range specs {
		if err := rs.Validate(); err != nil {
			return nil, trace.Wrap(err, "validating resource %q", rs.Kind)
		}
		kind := strings.ToLower(rs.Kind)
		for _, g := range gens {
			if g.Condition != nil && !g.Condition(rs) {
				continue
			}
			content, err := g.Generate(rs, module)
			if err != nil {
				return nil, trace.Wrap(err, "generating %s for %q", g.Name, kind)
			}
			fileContent := content
			if !g.SkipIfExists {
				fileContent = generatedHeader + content
			}
			files = append(files, generatedFile{
				Path:         g.PathFunc(kind, rs),
				Content:      fileContent,
				SkipIfExists: g.SkipIfExists,
			})
		}
	}

	// Cross-resource: kind constants file
	{
		content, err := generators.GenerateKindConstants(specs)
		if err != nil {
			return nil, trace.Wrap(err, "generating kind constants")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("api", "types", "constants.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// Cross-resource: services gathering (lib/auth/services.gen.go)
	// Always generated — the main codebase embeds servicesGenerated.
	{
		content, err := generators.GenerateServicesGathering(specs, module)
		if err != nil {
			return nil, trace.Wrap(err, "generating services gathering")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("lib", "auth", "services.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// Cross-resource: cache gathering (lib/cache/index.gen.go)
	// Always generated — the main codebase embeds generatedConfig.
	{
		content, err := generators.GenerateCacheGathering(specs, module)
		if err != nil {
			return nil, trace.Wrap(err, "generating cache gathering")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("lib", "cache", "index.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// Cross-resource: authclient gathering (lib/auth/authclient/api.gen.go)
	// Always generated — the main codebase embeds cacheGeneratedServices.
	{
		content, err := generators.GenerateAuthclientGathering(specs, module)
		if err != nil {
			return nil, trace.Wrap(err, "generating authclient gathering")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("lib", "auth", "authclient", "api.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// Cross-resource: event type constants (lib/events/api.gen.go)
	{
		content, err := generators.GenerateEventsAPI(specs)
		if err != nil {
			return nil, trace.Wrap(err, "generating event type constants")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("lib", "events", "api.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// Cross-resource: event code constants (lib/events/codes.gen.go)
	{
		content, err := generators.GenerateEventsCodes(specs)
		if err != nil {
			return nil, trace.Wrap(err, "generating event code constants")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("lib", "events", "codes.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// Cross-resource: dynamic event factory (lib/events/dynamic.gen.go)
	{
		content, err := generators.GenerateEventsDynamic(specs, module)
		if err != nil {
			return nil, trace.Wrap(err, "generating dynamic event factory")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("lib", "events", "dynamic.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// Cross-resource: events test map (lib/events/events_test.gen.go)
	{
		content, err := generators.GenerateEventsTest(specs, module)
		if err != nil {
			return nil, trace.Wrap(err, "generating events test map")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("lib", "events", "events_test.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// Cross-resource: OneOf converter (api/types/events/oneof.gen.go)
	{
		content, err := generators.GenerateEventsOneOf(specs)
		if err != nil {
			return nil, trace.Wrap(err, "generating OneOf converter")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("api", "types", "events", "oneof.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// api/types/events/trim.gen.go — TrimToMaxSize for generated event types.
	{
		content, err := generators.GenerateEventsTrim(specs)
		if err != nil {
			return nil, trace.Wrap(err, "generating events trim")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("api", "types", "events", "trim.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// Cross-resource: events client gathering (api/client/events_generated.gen.go)
	// Generates dispatch functions for EventToGRPC/EventFromGRPC.
	{
		content, err := generators.GenerateEventsClientGathering(specs, module)
		if err != nil {
			return nil, trace.Wrap(err, "generating events client gathering")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("api", "client", "events_generated.gen.go"),
			Content: generatedHeader + content,
		})
	}

	// Cross-resource: shortcut aliases for ParseShortcut (lib/services/shortcuts.gen.go)
	{
		content, err := generators.GenerateShortcutGathering(specs)
		if err != nil {
			return nil, trace.Wrap(err, "generating shortcut gathering")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("lib", "services", "shortcuts.gen.go"),
			Content: generatedHeader + content,
		})
	}

	return files, nil
}
