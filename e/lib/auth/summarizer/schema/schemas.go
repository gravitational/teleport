package schema

import (
	_ "embed"
	"encoding/json"
)

var (
	//go:embed generated/command_analysis.json
	commandAnalysisSchemaJSON []byte

	//go:embed generated/prose_embedding.json
	proseEmbeddingSchemaJSON []byte

	//go:embed generated/session_analysis.json
	sessionAnalysisSchemaJSON []byte

	//go:embed generated/desktop_screenshot_analysis.json
	desktopScreenshotAnalysisSchemaJSON []byte

	//go:embed generated/desktop_session_analysis.json
	desktopSessionAnalysisSchemaJSON []byte
)

var (
	CommandAnalysisSchema           = mustSchema(commandAnalysisSchemaJSON)
	ProseEmbeddingSchema            = mustSchema(proseEmbeddingSchemaJSON)
	SessionAnalysisSchema           = mustSchema(sessionAnalysisSchemaJSON)
	DesktopScreenshotAnalysisSchema = mustSchema(desktopScreenshotAnalysisSchemaJSON)
	DesktopSessionAnalysisSchema    = mustSchema(desktopSessionAnalysisSchemaJSON)
)

func mustSchema(data []byte) json.RawMessage {
	if !json.Valid(data) {
		panic("embedded schema is not valid JSON")
	}
	return json.RawMessage(append([]byte(nil), data...))
}
