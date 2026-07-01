package report

import (
	"encoding/json"
	"io"
)

// RenderJSON writes the full report as formatted JSON.
func RenderJSON(rpt *Report, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rpt)
}
