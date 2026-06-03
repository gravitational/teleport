package mcp

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/mcputils"
)

// sanitizeParamsName removes all non-canonical "name" params (e.g. "Name") from the MCP Request
// message. This is to prevent vulnerable upstream MCP servers and responding to malformed messages. This can happen when diff
func sanitizeParamsName(r *mcputils.JSONRPCRequest) {
	deleteNonCanonicalKey(r.Params, mcputils.ParamsNameKey)
}

func sanitizeRawRequest(request []byte) ([]byte, error) {
	if len(request) == 0 {
		return request, nil
	}

	// User json.RawMessage, to not mess with the content, e.g. introduce floating-point
	// corruption caused by rounding.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(request, &m); err != nil {
		return nil, trace.Wrap(err, "unmarshaling MCP Request into a map")
	}

	modifiedM := deleteNonCanonicalKey(m, mcputils.MethodKey, mcputils.ParamsKey)

	var err error
	var params map[string]json.RawMessage

	rawParams := m[mcputils.ParamsKey]
	if len(rawParams) == 0 {
		if modifiedM {
			goto encodeAndReturnM
		}
		return request, nil
	}

	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, trace.Wrap(err, "unmarshaling MCP Request params into a map")
	}

	deleteNonCanonicalKey(params, mcputils.ParamsNameKey)

	m[mcputils.ParamsKey], err = json.Marshal(params)
	if err != nil {
		return nil, trace.Wrap(err, "encoding MCP Request params map back into JSON")
	}

encodeAndReturnM:
	buf := bytes.NewBuffer(request[:0]) // reuse the slice to avoid allocations
	if err := json.NewEncoder(buf).Encode(m); err != nil {
		return nil, trace.Wrap(err, "encoding MCP Request map back into JSON")
	}
	return buf.Bytes(), nil
}

func deleteNonCanonicalKey[V any](m map[string]V, canonicalKey ...string) (modified bool) {
	for k := range m {
		for _, ck := range canonicalKey {
			if strings.EqualFold(k, ck) && k != ck {
				modified = true
				delete(m, k)
			}
		}
	}
	return
}
