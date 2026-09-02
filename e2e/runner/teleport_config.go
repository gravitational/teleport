package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// mergeTeleportConfig deep-merges a custom config into the base Teleport config at basePath and writes the
// result to outPath. A non-empty licenseFile overrides auth_service.license_file in the result.
func mergeTeleportConfig(basePath, outPath, e2eDir, raw, licenseFile string) error {
	raw = strings.ReplaceAll(raw, "${E2E_DIR}", e2eDir)

	override := map[string]any{}
	if raw != "" {
		if err := yaml.Unmarshal([]byte(raw), &override); err != nil {
			return fmt.Errorf("parsing declared teleport config %q: %w", raw, err)
		}
	}

	if licenseFile != "" {
		auth, ok := override["auth_service"].(map[string]any)
		if !ok {
			auth = map[string]any{}
			override["auth_service"] = auth
		}

		auth["license_file"] = licenseFile
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

// licensePath resolves a test-declared license name to its PEM under e/fixtures, the single home
// for licenses shared with the enterprise Go tests.
func licensePath(repoRoot, name string) (string, error) {
	if name != filepath.Base(name) || name == "." || name == ".." {
		return "", fmt.Errorf("invalid license name %q: must not contain a path", name)
	}

	dir := filepath.Join(repoRoot, "e", "fixtures")
	path := filepath.Join(dir, "license-"+name+".pem")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("no license %q: %s does not exist", name, path)
	}

	return path, nil
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
