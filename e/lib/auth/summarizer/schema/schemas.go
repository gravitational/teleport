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
)

var (
	CommandAnalysisSchema = mustSchema(commandAnalysisSchemaJSON)
	ProseEmbeddingSchema  = mustSchema(proseEmbeddingSchemaJSON)
	SessionAnalysisSchema = mustSchema(sessionAnalysisSchemaJSON)
)

func mustSchema(data []byte) json.RawMessage {
	if !json.Valid(data) {
		panic("embedded schema is not valid JSON")
	}
	return json.RawMessage(append([]byte(nil), data...))
}
