package schema

import (
	"github.com/invopop/jsonschema"
)

// generateSchema generates JSON schema for structured outputs
func generateSchema[T any]() *jsonschema.Schema {
	// Structured Outputs uses a subset of JSON schema
	// These flags are necessary to comply with the subset
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	return reflector.Reflect(v)
}
