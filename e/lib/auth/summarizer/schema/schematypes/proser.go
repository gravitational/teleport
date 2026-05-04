package schematypes

// ProseEmbedding is the structured output returned by the LLM when generating
// a dense prose embedding for a session. The condensed text is then used as
// input to an embedding model to produce a vector representation of the session.
type ProseEmbedding struct {
	// CondensedText is the generated text used for embedding.
	CondensedText string `json:"condensed_text" jsonschema:"required" jsonschema_description:"The generated text used for embedding generation."`
}
