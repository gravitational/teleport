package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/invopop/jsonschema"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema/schematypes"
)

func main() {
	outDir := filepath.Join(repoRoot(), "e", "lib", "auth", "summarizer", "schema", "generated")
	must(os.RemoveAll(outDir))
	must(os.MkdirAll(outDir, 0o755))

	must(generateSchemaFile[schematypes.CommandAnalysis](filepath.Join(outDir, "command_analysis.json")))
	must(generateSchemaFile[schematypes.ProseEmbedding](filepath.Join(outDir, "prose_embedding.json")))
	must(generateSchemaFile[schematypes.SessionAnalysis](filepath.Join(outDir, "session_analysis.json")))
	must(generateSchemaFile[schematypes.DesktopScreenshotAnalysis](filepath.Join(outDir, "desktop_screenshot_analysis.json")))
	must(generateSchemaFile[schematypes.DesktopSessionAnalysis](filepath.Join(outDir, "desktop_session_analysis.json")))
}

func generateSchemaFile[T any](out string) error {
	reflector := jsonschema.Reflector{
		BaseSchemaID:              "https://github.com/gravitational/teleport/e/lib/auth/summarizer/schema",
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	s := reflector.Reflect(v)

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	return os.WriteFile(out, data, 0o644)
}

func repoRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to determine schemagen path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
