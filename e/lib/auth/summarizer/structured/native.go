package structured

import (
	"fmt"
	"hash/fnv"
)

// NativeSupport classifies whether an endpoint is known to support, known not to support, or has unknown
// support for a provider's native structured output API.
type NativeSupport int

const (
	// SupportUnknown means support is undetermined, so the native API is attempted optimistically once and, if the
	// endpoint rejects it, the provider falls back to the prompt-based path and remembers the verdict.
	SupportUnknown NativeSupport = iota
	// SupportYes means the endpoint is known to support the native structured output API.
	SupportYes
	// SupportNo means the endpoint is known not to support the native structured output API, so the prompt-based
	// path is used directly.
	SupportNo
)

// SchemaFingerprint returns a stable short fingerprint of a marshaled JSON schema, suitable as the schema
// dimension of a [SupportCache] key. The native-support verdict is cached per schema because an endpoint can
// reject one schema as incompatible while supporting the native API for others.
func SchemaFingerprint(schema []byte) string {
	h := fnv.New64a()
	_, _ = h.Write(schema)
	return fmt.Sprintf("%x", h.Sum64())
}
