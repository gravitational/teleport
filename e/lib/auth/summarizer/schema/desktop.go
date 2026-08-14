package schema

// ImageData holds a single screenshot for desktop session analysis.
type ImageData struct {
	// Data is the raw PNG-encoded bytes of the screenshot.
	Data []byte
}
