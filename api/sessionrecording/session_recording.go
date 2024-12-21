/*
Copyright 2024 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package sessionrecording can be used to read Teleport's
// session recording files.
package sessionrecording

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
)

const (
	// Int32Size is a constant for 32 bit integer byte size
	Int32Size = 4

	// Int64Size is a constant for 64 bit integer byte size
	Int64Size = 8

	// MaxProtoMessageSizeBytes is maximum protobuf marshaled message size
	MaxProtoMessageSizeBytes = 64 * 1024

	// ProtoStreamV1 is a version of the binary protocol
	ProtoStreamV1 = 1
)

const (
	// maxIterationLimit is max iteration limit
	maxIterationLimit = 1000
)

// NewReader returns a new session recording reader
func NewReader(r io.Reader) *Reader {
	return &Reader{
		reader:    r,
		lastIndex: -1,
	}
}

const (
	// protoReaderStateInit is ready to start reading the next part
	protoReaderStateInit = 0
	// protoReaderStateCurrent will read the data from the current part
	protoReaderStateCurrent = iota
	// protoReaderStateEOF indicates that reader has completed reading
	// all parts
	protoReaderStateEOF = iota
	// protoReaderStateError indicates that reader has reached internal
	// error and should close
	protoReaderStateError = iota
)

// Reader reads Teleport's session recordings
type Reader struct {
	gzipReader   *gzipReader
	padding      int64
	reader       io.Reader
	sizeBytes    [Int64Size]byte
	messageBytes [MaxProtoMessageSizeBytes]byte
	state        int
	error        error
	lastIndex    int64
	stats        ReaderStats
}

// ReaderStats contains some reader statistics
type ReaderStats struct {
	// SkippedEvents is a counter with encountered
	// events recorded several times or events
	// that have been out of order as skipped
	SkippedEvents int64
	// OutOfOrderEvents is a counter with events
	// received out of order
	OutOfOrderEvents int64
	// TotalEvents contains total amount of
	// processed events (including duplicates)
	TotalEvents int64
}

// ToFields returns a copy of the stats to be used as log fields
func (p ReaderStats) ToFields() map[string]any {
	return map[string]any{
		"skipped-events":      p.SkippedEvents,
		"out-of-order-events": p.OutOfOrderEvents,
		"total-events":        p.TotalEvents,
	}
}

// Close releases reader resources
func (r *Reader) Close() error {
	if r.gzipReader != nil {
		return r.gzipReader.Close()
	}
	return nil
}

// Reset sets reader to read from the new reader
// without resetting the stats, could be used
// to deduplicate the events
func (r *Reader) Reset(reader io.Reader) error {
	if r.error != nil {
		return r.error
	}
	if r.gzipReader != nil {
		if r.error = r.gzipReader.Close(); r.error != nil {
			return trace.Wrap(r.error)
		}
		r.gzipReader = nil
	}
	r.reader = reader
	r.state = protoReaderStateInit
	return nil
}

func (r *Reader) setError(err error) error {
	r.state = protoReaderStateError
	r.error = err
	return err
}

// GetStats returns stats about processed events
func (r *Reader) GetStats() ReaderStats {
	return r.stats
}

// Read returns next event or io.EOF in case of the end of the parts
func (r *Reader) Read(ctx context.Context) (apievents.AuditEvent, error) {
	// periodic checks of context after fixed amount of iterations
	// is an extra precaution to avoid
	// accidental endless loop due to logic error crashing the system
	// and allows ctx timeout to kick in if specified
	var checkpointIteration int64
	for {
		checkpointIteration++
		if checkpointIteration%maxIterationLimit == 0 {
			select {
			case <-ctx.Done():
				if ctx.Err() != nil {
					return nil, trace.Wrap(ctx.Err())
				}
				return nil, trace.LimitExceeded("context has been canceled")
			default:
			}
		}
		switch r.state {
		case protoReaderStateEOF:
			return nil, io.EOF
		case protoReaderStateError:
			return nil, r.error
		case protoReaderStateInit:
			// read the part header that consists of the protocol version
			// and the part size (for the V1 version of the protocol)
			_, err := io.ReadFull(r.reader, r.sizeBytes[:Int64Size])
			if err != nil {
				// reached the end of the stream
				if errors.Is(err, io.EOF) {
					r.state = protoReaderStateEOF
					return nil, err
				}
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			protocolVersion := binary.BigEndian.Uint64(r.sizeBytes[:Int64Size])
			if protocolVersion != ProtoStreamV1 {
				return nil, trace.BadParameter("unsupported protocol version %v", protocolVersion)
			}
			// read size of this gzipped part as encoded by V1 protocol version
			_, err = io.ReadFull(r.reader, r.sizeBytes[:Int64Size])
			if err != nil {
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			partSize := binary.BigEndian.Uint64(r.sizeBytes[:Int64Size])
			// read padding size (could be 0)
			_, err = io.ReadFull(r.reader, r.sizeBytes[:Int64Size])
			if err != nil {
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			r.padding = int64(binary.BigEndian.Uint64(r.sizeBytes[:Int64Size]))
			gzipReader, err := newGzipReader(io.NopCloser(io.LimitReader(r.reader, int64(partSize))))
			if err != nil {
				return nil, r.setError(trace.Wrap(err))
			}
			r.gzipReader = gzipReader
			r.state = protoReaderStateCurrent
			continue
			// read the next version from the gzip reader
		case protoReaderStateCurrent:
			// the record consists of length of the protobuf encoded
			// message and the message itself
			_, err := io.ReadFull(r.gzipReader, r.sizeBytes[:Int32Size])
			if err != nil {
				if !errors.Is(err, io.EOF) {
					return nil, r.setError(trace.ConvertSystemError(err))
				}

				// due to a bug in older versions of teleport it was possible that padding
				// bytes would end up inside of the gzip section of the archive. we should
				// skip any dangling data in the gzip secion.
				n, err := io.CopyBuffer(io.Discard, r.gzipReader.inner, r.messageBytes[:])
				if err != nil {
					return nil, r.setError(trace.ConvertSystemError(err))
				}

				if n != 0 {
					// log the number of bytes that were skipped
					slog.DebugContext(ctx, "skipped dangling data in session recording section", "length", n)
				}

				// reached the end of the current part, but not necessarily
				// the end of the stream
				if err := r.gzipReader.Close(); err != nil {
					return nil, r.setError(trace.ConvertSystemError(err))
				}
				if r.padding != 0 {
					skipped, err := io.CopyBuffer(io.Discard, io.LimitReader(r.reader, r.padding), r.messageBytes[:])
					if err != nil {
						return nil, r.setError(trace.ConvertSystemError(err))
					}
					if skipped != r.padding {
						return nil, r.setError(trace.BadParameter(
							"data truncated, expected to read %v bytes, but got %v", r.padding, skipped))
					}
				}
				r.padding = 0
				r.gzipReader = nil
				r.state = protoReaderStateInit
				continue
			}
			messageSize := binary.BigEndian.Uint32(r.sizeBytes[:Int32Size])
			// zero message size indicates end of the part
			// that sometimes is present in partially submitted parts
			// that have to be filled with zeroes for parts smaller
			// than minimum allowed size
			if messageSize == 0 {
				return nil, r.setError(trace.BadParameter("unexpected message size 0"))
			}
			_, err = io.ReadFull(r.gzipReader, r.messageBytes[:messageSize])
			if err != nil {
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			oneof := apievents.OneOf{}
			err = oneof.Unmarshal(r.messageBytes[:messageSize])
			if err != nil {
				return nil, trace.Wrap(err)
			}
			event, err := apievents.FromOneOf(oneof)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			r.stats.TotalEvents++
			if event.GetIndex() <= r.lastIndex {
				r.stats.SkippedEvents++
				continue
			}
			if r.lastIndex > 0 && event.GetIndex() != r.lastIndex+1 {
				r.stats.OutOfOrderEvents++
			}
			r.lastIndex = event.GetIndex()
			return event, nil
		default:
			return nil, trace.BadParameter("unsupported reader size")
		}
	}
}

// ReadAll reads all events until EOF
func (r *Reader) ReadAll(ctx context.Context) ([]apievents.AuditEvent, error) {
	var events []apievents.AuditEvent
	for {
		event, err := r.Read(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return events, nil
			}
			return nil, trace.Wrap(err)
		}
		events = append(events, event)
	}
}
