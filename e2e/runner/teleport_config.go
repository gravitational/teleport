/**
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

package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// mergeTeleportConfig deep-merges a custom config into the base Teleport config at basePath and writes the
// result to outPath.
func mergeTeleportConfig(basePath, outPath, e2eDir, raw string) error {
	raw = strings.ReplaceAll(raw, "${E2E_DIR}", e2eDir)

	var override map[string]any
	if err := yaml.Unmarshal([]byte(raw), &override); err != nil {
		return fmt.Errorf("parsing declared teleport config %q: %w", raw, err)
	}

	baseData, err := os.ReadFile(basePath)
	if err != nil {
		return fmt.Errorf("reading base config %s: %w", basePath, err)
	}
	base := map[string]any{}
	if err := yaml.Unmarshal(baseData, &base); err != nil {
		return fmt.Errorf("parsing base config %s: %w", basePath, err)
	}

	deepMerge(base, override)

	merged, err := yaml.Marshal(base)
	if err != nil {
		return fmt.Errorf("marshaling merged config: %w", err)
	}
	if err := os.WriteFile(outPath, merged, 0o644); err != nil {
		return fmt.Errorf("writing merged config %s: %w", outPath, err)
	}
	return nil
}

func deepMerge(dst, src map[string]any) {
	for k, sv := range src {
		if svMap, ok := sv.(map[string]any); ok {
			if dvMap, ok := dst[k].(map[string]any); ok {
				deepMerge(dvMap, svMap)
				continue
			}
		}
		dst[k] = sv
	}
}
