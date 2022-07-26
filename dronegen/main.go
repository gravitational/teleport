// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

func main() {
	if err := checkTDR(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	var pipelines []pipeline

	pipelines = append(pipelines, pushPipelines()...)
	pipelines = append(pipelines, tagPipelines()...)
	pipelines = append(pipelines, cronPipelines()...)
	pipelines = append(pipelines, artifactMigrationPipeline())
	pipelines = append(pipelines, promoteBuildPipelines()...)
	pipelines = append(pipelines, updateDocsPipeline())
	pipelines = append(pipelines, buildboxPipeline())

	if err := writePipelines(".drone.yml", pipelines); err != nil {
		fmt.Println("failed writing drone pipelines:", err)
		os.Exit(1)
	}

	if err := signDroneConfig(); err != nil {
		fmt.Println("failed signing .drone.yml:", err)
		os.Exit(1)
	}
}

func writePipelines(path string, newPipelines []pipeline) error {
	// Read the existing config and replace only those pipelines defined in
	// newPipelines.
	//
	// TODO: When all pipelines are migrated, remove this merging logic and
	// write the file directly. This will be simpler and allow cleanup of
	// pipelines when they are removed from this generator.
	existingConfig, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read existing config: %w", err)
	}
	existingPipelines, err := parsePipelines(existingConfig)
	if err != nil {
		return fmt.Errorf("failed to parse existing config: %w", err)
	}

	newPipelinesSet := make(map[string]pipeline, len(newPipelines))
	for _, p := range newPipelines {
		// TODO: remove this check once promoteBuildPipeline and
		// updateDocsPipeline are implemented.
		if p.Name == "" {
			continue
		}
		newPipelinesSet[p.Name] = p
	}

	pipelines := existingPipelines
	// Overwrite all existing pipelines with new ones that have the same name.
	for i, p := range pipelines {
		if np, ok := newPipelinesSet[p.Name]; ok {
			out, err := yaml.Marshal(np)
			if err != nil {
				return fmt.Errorf("failed to encode pipelines: %w", err)
			}
			// Add a little note about this being generated.
			out = append([]byte(np.comment), out...)
			pipelines[i] = parsedPipeline{pipeline: np, raw: out}
			delete(newPipelinesSet, np.Name)
		}
	}
	// If we decide to add new pipelines before everything is migrated to this
	// generator, this check needs to change.
	if len(newPipelinesSet) != 0 {
		var names []string
		for n := range newPipelinesSet {
			names = append(names, n)
		}
		return fmt.Errorf("pipelines %q don't exist in the current config, aborting", names)
	}

	var pipelinesEnc [][]byte
	for _, p := range pipelines {
		pipelinesEnc = append(pipelinesEnc, p.raw)
	}
	configData := bytes.Join(pipelinesEnc, []byte("\n---\n"))

	return os.WriteFile(path, configData, 0664)
}

// parsedPipeline is a single pipeline parsed from .drone.yml along with its
// unparsed form. It's used to preserve YAML comments and minimize diffs due to
// formatting.
//
// TODO: remove this when all pipelines are migrated. All comments will be
// moved to this generator instead.
type parsedPipeline struct {
	pipeline
	raw []byte
}

func parsePipelines(data []byte) ([]parsedPipeline, error) {
	chunks := bytes.Split(data, []byte("\n---\n"))
	var pipelines []parsedPipeline
	for _, c := range chunks {
		// Discard the signature, it will be re-generated.
		if bytes.HasPrefix(c, []byte("kind: signature")) {
			continue
		}
		var p pipeline
		if err := yaml.UnmarshalStrict(c, &p); err != nil {
			return nil, err
		}
		pipelines = append(pipelines, parsedPipeline{pipeline: p, raw: c})
	}
	return pipelines, nil
}
