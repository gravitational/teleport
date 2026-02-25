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

package parser

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

// ParseProtoDir compiles proto files under protoDir and returns specs for services that define teleport.resource_config.
func ParseProtoDir(ctx context.Context, protoDir string) ([]spec.ResourceSpec, error) {
	if strings.TrimSpace(protoDir) == "" {
		return nil, trace.BadParameter("proto dir is required")
	}

	candidates, err := findCandidateProtoFiles(protoDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{ImportPaths: []string{protoDir}}),
	}
	files, err := compiler.Compile(ctx, candidates...)
	if err != nil {
		return nil, trace.Wrap(err, "compiling proto files in %q", protoDir)
	}

	var out []spec.ResourceSpec
	for _, fd := range files {
		for i := range fd.Services().Len() {
			svc := fd.Services().Get(i)
			if !hasResourceConfigOption(svc) {
				continue
			}
			rs, err := ParseServiceDescriptor(svc)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out = append(out, rs)
		}
	}

	return out, nil
}

func findCandidateProtoFiles(protoDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(protoDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".proto" {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return trace.Wrap(err)
		}
		if !strings.Contains(string(b), "resource_config") {
			return nil
		}

		rel, err := filepath.Rel(protoDir, path)
		if err != nil {
			return trace.Wrap(err)
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sort.Strings(files)
	return files, nil
}
