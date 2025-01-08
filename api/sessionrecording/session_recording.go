/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package sessionrecording can be used to read Teleport's
// session recording files.
package sessionrecording

import (
	"compress/gzip"
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
//
// It is the caller's responsibility to call Close on the returned [Reader] from NewReader when done
// with the [Reader].
func NewReader(r io.Reader) *Reader {
	return &Reader{
		rawReader: r,
		lastIndex: -1,
	}
}

const (
	// protoReaderStateInit is ready to start reading the next part
	protoReaderStateInit = iota
	// protoReaderStateCurrent will read the data from the current part
	protoReaderStateCurrent
	// protoReaderStateEOF indicates that reader has completed reading
	// all parts
	protoReaderStateEOF
	// protoReaderStateError indicates that reader has reached internal
	// error and should close
	protoReaderStateError
)

// Reader reads Teleport's session recordings
type Reader struct {
	// partReader wraps rawReader and is limited to reading a single
	// (compressed) part from the session recording
	partReader io.Reader
	// gzipReader wraps partReader and decompresses a single part
	// from the session recording
	gzipReader *gzip.Reader
	// padding is how many bytes were added to hit a minimum file upload size
	padding int64
	// rawReader is the raw data source we read from
	rawReader io.Reader
	// state tracks where the Reader is at in consuming a session recording
	state int
	// error holds any error encountered while reading a session recording
	error error
	// lastIndex stores the last parsed event's index within a session (events found with an index less than or equal to lastIndex are skipped)
	lastIndex int64
	// stats contains info about processed events (e.g. total events processed, how many events were skipped)
	stats ReaderStats
}

// ReaderStats contains some reader statistics
type ReaderStats struct {
	// SkippedBytes is a counter with encountered bytes that have been skipped for processing.
	// Typically occurring due to a bug in older Teleport versions having padding bytes
	// written to the gzip section.
	SkippedBytes int64
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

// LogValue returns a copy of the stats to be used as log fields
func (p ReaderStats) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int64("skipped_bytes", p.SkippedBytes),
		slog.Int64("skipped_events", p.SkippedEvents),
		slog.Int64("out_of_order_events", p.OutOfOrderEvents),
		slog.Int64("total_events", p.TotalEvents),
	)
}

// Close releases reader resources
func (r *Reader) Close() error {
	if r.gzipReader != nil {
		return r.gzipReader.Close()
	}
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
	var sizeBytes [Int64Size]byte

	// periodic checks of context after fixed amount of iterations
	// is an extra precaution to avoid
	// accidental endless loop due to logic error crashing the system
	// and allows ctx timeout to kick in if specified
	for checkpointIteration := int64(1); ; checkpointIteration++ {
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
			if _, err := io.ReadFull(r.rawReader, sizeBytes[:Int64Size]); err != nil {
				// reached the end of the stream
				if errors.Is(err, io.EOF) {
					r.state = protoReaderStateEOF
					return nil, err
				}
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			protocolVersion := binary.BigEndian.Uint64(sizeBytes[:Int64Size])
			if protocolVersion != ProtoStreamV1 {
				return nil, trace.BadParameter("unsupported protocol version %v", protocolVersion)
			}
			// read size of this gzipped part as encoded by V1 protocol version
			if _, err := io.ReadFull(r.rawReader, sizeBytes[:Int64Size]); err != nil {
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			partSize := binary.BigEndian.Uint64(sizeBytes[:Int64Size])
			// read padding size (could be 0)
			if _, err := io.ReadFull(r.rawReader, sizeBytes[:Int64Size]); err != nil {
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			r.padding = int64(binary.BigEndian.Uint64(sizeBytes[:Int64Size]))
			r.partReader = io.LimitReader(r.rawReader, int64(partSize))
			gzipReader, err := gzip.NewReader(r.partReader)
			// older bugged versions of teleport would sometimes incorrectly inject padding bytes into
			// the gzip section of the archive. this causes gzip readers with multistream enabled (the
			// default behavior) to fail. we  disable multistream here in order to ensure that the gzip
			// reader halts when it reaches the end of the current (only) valid gzip entry.
			if err != nil {
				return nil, r.setError(trace.Wrap(err))
			}
			gzipReader.Multistream(false)
			r.gzipReader = gzipReader
			r.state = protoReaderStateCurrent
			continue
			// read the next version from the gzip reader
		case protoReaderStateCurrent:
			var messageBytes [MaxProtoMessageSizeBytes]byte

			// the record consists of length of the protobuf encoded
			// message and the message itself
			if _, err := io.ReadFull(r.gzipReader, sizeBytes[:Int32Size]); err != nil {
				if !errors.Is(err, io.EOF) {
					return nil, r.setError(trace.ConvertSystemError(err))
				}

				// due to a bug in older versions of teleport it was possible that padding
				// bytes would end up inside of the gzip section of the archive. we should
				// skip any dangling data in the gzip secion.
				n, err := io.CopyBuffer(io.Discard, r.partReader, messageBytes[:])
				if err != nil {
					return nil, r.setError(trace.ConvertSystemError(err))
				}

				if n != 0 {
					r.stats.SkippedBytes += n
					// log the number of bytes that were skipped
					slog.DebugContext(ctx, "skipped dangling data in session recording section", "length", n)
				}

				// reached the end of the current part, but not necessarily
				// the end of the stream
				if err := r.gzipReader.Close(); err != nil {
					return nil, r.setError(trace.ConvertSystemError(err))
				}
				if r.padding != 0 {
					skipped, err := io.CopyBuffer(io.Discard, io.LimitReader(r.rawReader, r.padding), messageBytes[:])
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
			messageSize := binary.BigEndian.Uint32(sizeBytes[:Int32Size])
			// zero message size indicates end of the part
			// that sometimes is present in partially submitted parts
			// that have to be filled with zeroes for parts smaller
			// than minimum allowed size
			if messageSize == 0 {
				return nil, r.setError(trace.BadParameter("unexpected message size 0"))
			}
			if _, err := io.ReadFull(r.gzipReader, messageBytes[:messageSize]); err != nil {
				return nil, r.setError(trace.ConvertSystemError(err))
			}
			oneof := apievents.OneOf{}
			if err := oneof.Unmarshal(messageBytes[:messageSize]); err != nil {
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
