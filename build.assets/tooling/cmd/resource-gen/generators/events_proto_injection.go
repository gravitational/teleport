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

package generators

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

// eventsProtoRelPath is the path to events.proto relative to the proto root.
const eventsProtoRelPath = "teleport/legacy/types/events/events.proto"

// InjectEventsProto reads events.proto, determines which event messages and
// OneOf entries are missing for the given resource specs, and appends them.
// It returns whether any changes were made.
//
// This function is append-only — it never removes existing entries.
//
// repoRoot is the repository root (parent of api/proto) — needed to run
// buf export for dependency resolution.
func InjectEventsProto(ctx context.Context, protoDir string, repoRoot string, specs []spec.ResourceSpec) (changed bool, err error) {
	eventsPath := filepath.Join(protoDir, filepath.FromSlash(eventsProtoRelPath))

	// Read current file content.
	content, err := os.ReadFile(eventsPath)
	if err != nil {
		return false, trace.Wrap(err, "reading events.proto")
	}

	// Export all protos (including deps like gogoproto) into a temp dir
	// so protocompile can resolve every import.
	exportDir, err := bufExport(ctx, repoRoot)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer os.RemoveAll(exportDir)

	// Compile events.proto to get the OneOf descriptor.
	existing, maxFieldNum, err := parseExistingOneOfEntries(ctx, exportDir)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// Determine which event messages are needed but missing.
	needed := neededEventMessages(specs)
	var missing []eventMessageSpec
	for _, m := range needed {
		if !existing[m.Name] {
			missing = append(missing, m)
		}
	}
	if len(missing) == 0 {
		return false, nil
	}

	// Assign field numbers sequentially from max+1.
	nextFieldNum := maxFieldNum + 1
	for i := range missing {
		missing[i].FieldNum = nextFieldNum
		nextFieldNum++
	}

	// Text manipulation: insert OneOf entries and append message definitions.
	result := string(content)
	result, err = insertOneOfEntries(result, missing)
	if err != nil {
		return false, trace.Wrap(err)
	}
	result = appendEventMessages(result, missing)

	if err := os.WriteFile(eventsPath, []byte(result), 0o644); err != nil {
		return false, trace.Wrap(err, "writing events.proto")
	}
	return true, nil
}

// eventMessageSpec describes an event message to inject.
type eventMessageSpec struct {
	Name        string // e.g. "CookieCreate"
	Lower       string // e.g. "cookie"
	Article     string // "a" or "an"
	OpPastTense string // e.g. "created"
	FieldNum    int    // OneOf field number (assigned later)
}

// bufExport runs "buf export" to produce a flat directory containing all proto
// sources including resolved dependencies (gogoproto, googleapis, etc.).
func bufExport(ctx context.Context, repoRoot string) (string, error) {
	dir, err := os.MkdirTemp("", "resource-gen-buf-export-*")
	if err != nil {
		return "", trace.Wrap(err)
	}

	cmd := exec.CommandContext(ctx, "buf", "export", "-o", dir)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", trace.Wrap(err, "buf export failed: %s", string(out))
	}
	return dir, nil
}

// parseExistingOneOfEntries compiles events.proto and extracts all existing
// OneOf field names and the maximum field number.
func parseExistingOneOfEntries(ctx context.Context, protoDir string) (existing map[string]bool, maxFieldNum int, err error) {
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: []string{protoDir},
		}),
	}
	files, err := compiler.Compile(ctx, eventsProtoRelPath)
	if err != nil {
		return nil, 0, trace.Wrap(err, "compiling events.proto")
	}
	if len(files) == 0 {
		return nil, 0, trace.BadParameter("events.proto produced no file descriptors")
	}

	fd := files[0]
	oneOfMsg := fd.Messages().ByName("OneOf")
	if oneOfMsg == nil {
		return nil, 0, trace.BadParameter("events.proto missing message OneOf")
	}
	eventOneof := oneOfMsg.Oneofs().ByName("Event")
	if eventOneof == nil {
		return nil, 0, trace.BadParameter("events.proto OneOf missing oneof Event")
	}

	existing = make(map[string]bool, eventOneof.Fields().Len())
	for i := 0; i < eventOneof.Fields().Len(); i++ {
		f := eventOneof.Fields().Get(i)
		existing[string(f.Name())] = true
		if num := int(f.Number()); num > maxFieldNum {
			maxFieldNum = num
		}
	}
	return existing, maxFieldNum, nil
}

// neededEventMessages computes the full list of event messages needed across
// all resource specs, sorted alphabetically by name.
func neededEventMessages(specs []spec.ResourceSpec) []eventMessageSpec {
	var msgs []eventMessageSpec
	for _, rs := range specs {
		kind := rs.KindPascal
		lower := rs.Kind
		art := indefiniteArticle(kind)
		if rs.Audit.EmitOnCreate && rs.Operations.Create {
			msgs = append(msgs, eventMessageSpec{Name: kind + "Create", Lower: lower, Article: art, OpPastTense: "created"})
		}
		if rs.Audit.EmitOnUpdate && (rs.Operations.Update || rs.Operations.Upsert) {
			msgs = append(msgs, eventMessageSpec{Name: kind + "Update", Lower: lower, Article: art, OpPastTense: "updated"})
		}
		if rs.Audit.EmitOnDelete && rs.Operations.Delete {
			msgs = append(msgs, eventMessageSpec{Name: kind + "Delete", Lower: lower, Article: art, OpPastTense: "deleted"})
		}
		if rs.Audit.EmitOnGet && rs.Operations.Get {
			msgs = append(msgs, eventMessageSpec{Name: kind + "Get", Lower: lower, Article: art, OpPastTense: "read"})
		}
	}
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].Name < msgs[j].Name
	})
	return msgs
}

// insertOneOfEntries finds the closing brace of `oneof Event {` inside
// `message OneOf {` and inserts new entries before it.
func insertOneOfEntries(content string, entries []eventMessageSpec) (string, error) {
	// Find "oneof Event {" then brace-count to its closing "}".
	oneofIdx := strings.Index(content, "oneof Event {")
	if oneofIdx < 0 {
		return "", trace.BadParameter("could not find 'oneof Event {' in events.proto")
	}

	// Find the opening brace of the oneof.
	braceStart := strings.Index(content[oneofIdx:], "{")
	if braceStart < 0 {
		return "", trace.BadParameter("could not find opening brace of oneof Event")
	}
	braceStart += oneofIdx

	// Count braces to find the matching closing brace.
	depth := 0
	closeIdx := -1
	for i := braceStart; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				closeIdx = i
			}
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx < 0 {
		return "", trace.BadParameter("could not find closing brace of oneof Event")
	}

	// Build the new entries text.
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "    events.%s %s = %d;\n", e.Name, e.Name, e.FieldNum)
	}

	// Walk back from the closing brace past any whitespace to find the
	// end of the last real line. Insert our entries there so indentation
	// is not doubled with the closing brace's leading whitespace.
	insertIdx := closeIdx
	for insertIdx > 0 && (content[insertIdx-1] == ' ' || content[insertIdx-1] == '\t') {
		insertIdx--
	}

	return content[:insertIdx] + b.String() + content[insertIdx:], nil
}

// watchProtoRelPath is the path to event.proto (watch events) relative to the proto root.
const watchProtoRelPath = "teleport/legacy/client/proto/event.proto"

// watchEntry describes a resource entry to inject into the Event.Resource oneof.
type watchEntry struct {
	Kind        string // PascalCase, e.g. "Cookie"
	Lower       string // snake_case, e.g. "cookie"
	ProtoImport string // e.g. "teleport/cookie/v1/cookie.proto"
	ProtoType   string // e.g. "teleport.cookie.v1.Cookie"
	FieldNum    int    // oneof field number (assigned later)
}

// InjectEventProtoWatch reads event.proto, determines which import and oneof
// Resource entries are missing for cache-enabled resource specs, and inserts
// them. It returns whether any changes were made.
//
// This function is append-only — it never removes existing entries.
func InjectEventProtoWatch(ctx context.Context, protoDir string, repoRoot string, specs []spec.ResourceSpec) (changed bool, err error) {
	// Filter to cache-enabled resources only.
	var cacheSpecs []spec.ResourceSpec
	for _, rs := range specs {
		if rs.Cache.Enabled {
			cacheSpecs = append(cacheSpecs, rs)
		}
	}
	if len(cacheSpecs) == 0 {
		return false, nil
	}

	eventPath := filepath.Join(protoDir, filepath.FromSlash(watchProtoRelPath))
	content, err := os.ReadFile(eventPath)
	if err != nil {
		return false, trace.Wrap(err, "reading event.proto")
	}

	// Export all protos (including deps) into a temp dir
	// so protocompile can resolve every import.
	exportDir, err := bufExport(ctx, repoRoot)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer os.RemoveAll(exportDir)

	// Parse event.proto to get existing oneof Resource entries.
	existingNames, maxFieldNum, err := parseExistingWatchEntries(ctx, exportDir)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// Determine which entries are missing.
	needed := neededWatchEntries(cacheSpecs)
	var missing []watchEntry
	for _, e := range needed {
		if !existingNames[e.Kind] {
			missing = append(missing, e)
		}
	}
	if len(missing) == 0 {
		return false, nil
	}

	// Assign field numbers sequentially from max+1.
	nextFieldNum := maxFieldNum + 1
	for i := range missing {
		missing[i].FieldNum = nextFieldNum
		nextFieldNum++
	}

	// Text manipulation: insert imports and oneof entries.
	result := string(content)
	result = insertWatchImports(result, missing)
	result, err = insertWatchOneOfEntries(result, missing)
	if err != nil {
		return false, trace.Wrap(err)
	}

	if err := os.WriteFile(eventPath, []byte(result), 0o644); err != nil {
		return false, trace.Wrap(err, "writing event.proto")
	}
	return true, nil
}

// parseExistingWatchEntries compiles event.proto and extracts all existing
// oneof Resource field names and the maximum field number.
func parseExistingWatchEntries(ctx context.Context, protoDir string) (existing map[string]bool, maxFieldNum int, err error) {
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: []string{protoDir},
		}),
	}
	files, err := compiler.Compile(ctx, watchProtoRelPath)
	if err != nil {
		return nil, 0, trace.Wrap(err, "compiling event.proto")
	}
	if len(files) == 0 {
		return nil, 0, trace.BadParameter("event.proto produced no file descriptors")
	}

	fd := files[0]
	eventMsg := fd.Messages().ByName("Event")
	if eventMsg == nil {
		return nil, 0, trace.BadParameter("event.proto missing message Event")
	}
	resourceOneof := eventMsg.Oneofs().ByName("Resource")
	if resourceOneof == nil {
		return nil, 0, trace.BadParameter("event.proto Event missing oneof Resource")
	}

	existing = make(map[string]bool, resourceOneof.Fields().Len())
	for i := 0; i < resourceOneof.Fields().Len(); i++ {
		f := resourceOneof.Fields().Get(i)
		existing[string(f.Name())] = true
		if num := int(f.Number()); num > maxFieldNum {
			maxFieldNum = num
		}
	}
	return existing, maxFieldNum, nil
}

// neededWatchEntries computes the watch entries needed for cache-enabled
// resources, sorted alphabetically by kind.
func neededWatchEntries(specs []spec.ResourceSpec) []watchEntry {
	var entries []watchEntry
	for _, rs := range specs {
		parts := strings.Split(rs.ServiceName, ".")
		if len(parts) < 4 {
			continue
		}
		pkgParts := parts[:len(parts)-1]
		protoImport := strings.Join(pkgParts, "/") + "/" + rs.Kind + ".proto"
		protoType := strings.Join(pkgParts, ".") + "." + rs.KindPascal

		entries = append(entries, watchEntry{
			Kind:        rs.KindPascal,
			Lower:       rs.Kind,
			ProtoImport: protoImport,
			ProtoType:   protoType,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Lower < entries[j].Lower
	})
	return entries
}

// insertWatchImports inserts missing proto import lines in alphabetical order.
func insertWatchImports(content string, entries []watchEntry) string {
	for _, e := range entries {
		importStr := fmt.Sprintf("import \"%s\";", e.ProtoImport)
		if strings.Contains(content, importStr) {
			continue
		}

		// Find the right alphabetical position among existing imports.
		lines := strings.Split(content, "\n")
		bestIdx := -1
		firstImportIdx := -1
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "import \"") {
				continue
			}
			if firstImportIdx == -1 {
				firstImportIdx = i
			}
			if trimmed < importStr {
				bestIdx = i
			}
		}

		var insertAt int
		if bestIdx >= 0 {
			insertAt = bestIdx + 1
		} else if firstImportIdx >= 0 {
			insertAt = firstImportIdx
		} else {
			continue // no imports found
		}

		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:insertAt]...)
		newLines = append(newLines, importStr)
		newLines = append(newLines, lines[insertAt:]...)
		content = strings.Join(newLines, "\n")
	}
	return content
}

// insertWatchOneOfEntries finds the closing brace of `oneof Resource {` inside
// `message Event {` and inserts new entries before it.
func insertWatchOneOfEntries(content string, entries []watchEntry) (string, error) {
	oneofIdx := strings.Index(content, "oneof Resource {")
	if oneofIdx < 0 {
		return "", trace.BadParameter("could not find 'oneof Resource {' in event.proto")
	}

	braceStart := strings.Index(content[oneofIdx:], "{")
	if braceStart < 0 {
		return "", trace.BadParameter("could not find opening brace of oneof Resource")
	}
	braceStart += oneofIdx

	depth := 0
	closeIdx := -1
	for i := braceStart; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				closeIdx = i
			}
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx < 0 {
		return "", trace.BadParameter("could not find closing brace of oneof Resource")
	}

	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "    // %s is a resource for %s management.\n", e.Kind, e.Lower)
		fmt.Fprintf(&b, "    %s %s = %d;\n", e.ProtoType, e.Kind, e.FieldNum)
	}

	insertIdx := closeIdx
	for insertIdx > 0 && (content[insertIdx-1] == ' ' || content[insertIdx-1] == '\t') {
		insertIdx--
	}

	return content[:insertIdx] + b.String() + content[insertIdx:], nil
}

// appendEventMessages appends event message definitions at the end of the file.
func appendEventMessages(content string, msgs []eventMessageSpec) string {
	var b strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&b, `
// %s is emitted when %s %s resource is %s.
message %s {
  Metadata Metadata = 1 [
    (gogoproto.nullable) = false,
    (gogoproto.embed) = true,
    (gogoproto.jsontag) = ""
  ];

  ResourceMetadata Resource = 2 [
    (gogoproto.nullable) = false,
    (gogoproto.embed) = true,
    (gogoproto.jsontag) = ""
  ];

  UserMetadata User = 3 [
    (gogoproto.nullable) = false,
    (gogoproto.embed) = true,
    (gogoproto.jsontag) = ""
  ];

  ConnectionMetadata Connection = 4 [
    (gogoproto.nullable) = false,
    (gogoproto.embed) = true,
    (gogoproto.jsontag) = ""
  ];

  Status Status = 5 [
    (gogoproto.nullable) = false,
    (gogoproto.embed) = true,
    (gogoproto.jsontag) = ""
  ];
}
`, m.Name, m.Article, m.Lower, m.OpPastTense, m.Name)
	}
	// Ensure file ends with a newline.
	result := strings.TrimRight(content, "\n") + "\n" + b.String()
	return result
}
