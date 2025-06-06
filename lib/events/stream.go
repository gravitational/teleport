/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package events

import (
	"io"

	"github.com/gravitational/teleport/lib/eventsclient"
)

const (
	// Int32Size is a constant for 32 bit integer byte size
	Int32Size = eventsclient.Int32Size

	// Int64Size is a constant for 64 bit integer byte size
	Int64Size = eventsclient.Int64Size

	// ConcurrentUploadsPerStream limits the amount of concurrent uploads
	// per stream
	ConcurrentUploadsPerStream = eventsclient.ConcurrentUploadsPerStream

	// MaxProtoMessageSizeBytes is maximum protobuf marshaled message size
	MaxProtoMessageSizeBytes = eventsclient.MaxProtoMessageSizeBytes

	// MinUploadPartSizeBytes is the minimum allowed part size when uploading a part to
	// Amazon S3.
	MinUploadPartSizeBytes = eventsclient.MinUploadPartSizeBytes

	// ProtoStreamV1 is a version of the binary protocol
	ProtoStreamV1 = eventsclient.ProtoStreamV1

	// ProtoStreamV1PartHeaderSize is the size of the part of the protocol stream
	// on disk format, it consists of
	// * 8 bytes for the format version
	// * 8 bytes for meaningful size of the part
	// * 8 bytes for optional padding size at the end of the slice
	ProtoStreamV1PartHeaderSize = eventsclient.ProtoStreamV1PartHeaderSize

	// ProtoStreamV1RecordHeaderSize is the size of the header
	// of the record header, it consists of the record length
	ProtoStreamV1RecordHeaderSize = eventsclient.ProtoStreamV1RecordHeaderSize
)

// ProtoStreamerConfig specifies configuration for the part
type ProtoStreamerConfig = eventsclient.ProtoStreamerConfig

// NewProtoStreamer creates protobuf-based streams
func NewProtoStreamer(cfg ProtoStreamerConfig) (*ProtoStreamer, error) {
	return eventsclient.NewProtoStreamer(cfg)
}

// ProtoStreamer creates protobuf-based streams uploaded to the storage
// backends, for example S3 or GCS
type ProtoStreamer = eventsclient.ProtoStreamer

// ProtoStreamConfig configures proto stream
type ProtoStreamConfig = eventsclient.ProtoStreamConfig

// NewProtoStream uploads session recordings in the protobuf format.
//
// The individual session stream is represented by continuous globally
// ordered sequence of events serialized to binary protobuf format.
//
// The stream is split into ordered slices of gzipped audit events.
//
// Each slice is composed of three parts:
//
// 1. Slice starts with 24 bytes version header
//
// * 8 bytes for the format version (used for future expansion)
// * 8 bytes for meaningful size of the part
// * 8 bytes for padding at the end of the slice (if present)
//
// 2. V1 body of the slice is gzipped protobuf messages in binary format.
//
// 3. Optional padding (if specified in the header), required
// to bring slices to minimum slice size.
//
// The slice size is determined by S3 multipart upload requirements:
//
// https://docs.aws.amazon.com/AmazonS3/latest/dev/qfacts.html
//
// This design allows the streamer to upload slices using S3-compatible APIs
// in parallel without buffering to disk.
func NewProtoStream(cfg ProtoStreamConfig) (*ProtoStream, error) {
	return eventsclient.NewProtoStream(cfg)
}

// ProtoStream implements concurrent safe event emitter
// that uploads the parts in parallel to S3
type ProtoStream = eventsclient.ProtoStream

// NewProtoReader returns a new proto reader with slice pool
func NewProtoReader(r io.Reader) *ProtoReader {
	return eventsclient.NewProtoReader(r)
}

// SessionReader provides method to read
// session events one by one
type SessionReader = eventsclient.SessionReader

// ProtoReader reads protobuf encoding from reader
type ProtoReader = eventsclient.ProtoReader

// ProtoReaderStats contains some reader statistics
type ProtoReaderStats = eventsclient.ProtoReaderStats
