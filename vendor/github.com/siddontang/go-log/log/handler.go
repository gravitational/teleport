package log

import (
	"io"
)

//Handler writes logs to somewhere
type Handler interface {
	Write(p []byte) (n int, err error)
	Close() error
}

// StreamHandler writes logs to a specified io Writer, maybe stdout, stderr, etc...
type StreamHandler struct {
	w io.Writer
}

// NewStreamHandler creates a StreamHandler
func NewStreamHandler(w io.Writer) (*StreamHandler, error) {
	h := new(StreamHandler)

	h.w = w

	return h, nil
}

// Write implements Handler interface
func (h *StreamHandler) Write(b []byte) (n int, err error) {
	return h.w.Write(b)
}

// Close implements Handler interface
func (h *StreamHandler) Close() error {
	return nil
}

// NullHandler does nothing, it discards anything.
type NullHandler struct {
}

// NewNullHandler creates a NullHandler
func NewNullHandler() (*NullHandler, error) {
	return new(NullHandler), nil
}

// // Write implements Handler interface
func (h *NullHandler) Write(b []byte) (n int, err error) {
	return len(b), nil
}

// Close implements Handler interface
func (h *NullHandler) Close() error {
	return nil
}
