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
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// Header returns information about playback
type Header struct {
	// Tar detected tar format
	Tar bool
	// Proto is for proto format
	Proto bool
	// ProtoVersion is a version of the format, valid if Proto is true
	ProtoVersion int64
}

// DetectFormat detects format by reading first bytes
// of the header. Callers should call Seek()
// to reuse reader after calling this function.
func DetectFormat(r io.ReadSeeker) (*Header, error) {
	version := make([]byte, Int64Size)
	_, err := io.ReadFull(r, version)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	protocolVersion := binary.BigEndian.Uint64(version)
	if protocolVersion == ProtoStreamV1 {
		return &Header{
			Proto:        true,
			ProtoVersion: int64(protocolVersion),
		}, nil
	}
	_, err = r.Seek(0, 0)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	tr := tar.NewReader(r)
	_, err = tr.Next()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return &Header{Tar: true}, nil
}

// Export converts session files from binary/protobuf to text/JSON.
func Export(ctx context.Context, rs io.ReadSeeker, w io.Writer, exportFormat string) error {
	switch exportFormat {
	case teleport.JSON:
	default:
		return trace.BadParameter("unsupported format %q, %q is the only supported format", exportFormat, teleport.JSON)
	}

	format, err := DetectFormat(rs)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = rs.Seek(0, 0)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	switch {
	case format.Proto:
		protoReader := NewProtoReader(rs)
		for {
			event, err := protoReader.Read(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return trace.Wrap(err)
			}
			switch exportFormat {
			case teleport.JSON:
				data, err := utils.FastMarshal(event)
				if err != nil {
					return trace.ConvertSystemError(err)
				}
				_, err = fmt.Fprintln(w, string(data))
				if err != nil {
					return trace.ConvertSystemError(err)
				}
			default:
				return trace.BadParameter("unsupported format %q, %q is the only supported format", exportFormat, teleport.JSON)
			}
		}
	case format.Tar:
		return trace.BadParameter(
			"to review events in format of Teleport before version 4.4, extract the tarball and look inside")
	default:
		return trace.BadParameter("unsupported format %v", format)
	}
}

// WriteForSSHPlayback reads events from an SessionReader and writes them to disk in a format optimized for playback.
func WriteForSSHPlayback(ctx context.Context, sid session.ID, reader SessionReader, dir string) (*SSHPlaybackWriter, error) {
	w := &SSHPlaybackWriter{
		sid:        sid,
		reader:     reader,
		dir:        dir,
		eventIndex: -1,
	}
	defer func() {
		if err := w.Close(); err != nil {
			log.WithError(err).Warningf("Failed to close writer.")
		}
	}()
	return w, w.Write(ctx)
}

// SessionEvents returns slice of event fields from gzipped events file.
func (w *SSHPlaybackWriter) SessionEvents() ([]EventFields, error) {
	var sessionEvents []EventFields
	// events
	eventFile, err := os.Open(w.EventsPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer eventFile.Close()

	grEvents, err := gzip.NewReader(eventFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer grEvents.Close()
	scanner := bufio.NewScanner(grEvents)
	for scanner.Scan() {
		var f EventFields
		err := utils.FastUnmarshal(scanner.Bytes(), &f)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return sessionEvents, nil
			}
			return nil, trace.Wrap(err)
		}
		sessionEvents = append(sessionEvents, f)
	}

	if err := scanner.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	return sessionEvents, nil
}

// SessionChunks interprets the file at the given path as gzip-compressed list of session events and returns
// the uncompressed contents as a result.
func (w *SSHPlaybackWriter) SessionChunks() ([]byte, error) {
	var stream []byte
	chunkFile, err := os.Open(w.ChunksPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer chunkFile.Close()
	grChunk, err := gzip.NewReader(chunkFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer grChunk.Close()
	stream, err = io.ReadAll(grChunk)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return stream, nil
}

// SSHPlaybackWriter reads messages from an SessionReader and writes them
// to disk in a format suitable for SSH session playback.
type SSHPlaybackWriter struct {
	sid        session.ID
	dir        string
	reader     SessionReader
	indexFile  *os.File
	eventsFile *gzipWriter
	chunksFile *gzipWriter
	eventIndex int64
	EventsPath string
	ChunksPath string
}

// Close closes all files
func (w *SSHPlaybackWriter) Close() error {
	if w.indexFile != nil {
		if err := w.indexFile.Close(); err != nil {
			log.Warningf("Failed to close index file: %v.", err)
		}
		w.indexFile = nil
	}

	if w.chunksFile != nil {
		if err := w.chunksFile.Flush(); err != nil {
			log.Warningf("Failed to flush chunks file: %v.", err)
		}

		if err := w.chunksFile.Close(); err != nil {
			log.Warningf("Failed closing chunks file: %v.", err)
		}
	}

	if w.eventsFile != nil {
		if err := w.eventsFile.Flush(); err != nil {
			log.Warningf("Failed to flush events file: %v.", err)
		}

		if err := w.eventsFile.Close(); err != nil {
			log.Warningf("Failed closing events file: %v.", err)
		}
	}

	return nil
}

// Write writes all events from the SessionReader and writes
// files to disk in the format optimized for playback.
func (w *SSHPlaybackWriter) Write(ctx context.Context) error {
	if err := w.openIndexFile(); err != nil {
		return trace.Wrap(err)
	}
	for {
		event, err := w.reader.Read(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return trace.Wrap(err)
		}
		if err := w.writeEvent(event); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (w *SSHPlaybackWriter) writeEvent(event apievents.AuditEvent) error {
	switch event.GetType() {
	// Timing events for TTY playback go to both a chunks file (the raw bytes) as
	// well as well as the events file (structured events).
	case SessionPrintEvent:
		return trace.Wrap(w.writeSessionPrintEvent(event))

	// Playback does not use enhanced events at the moment,
	// so they are skipped
	case SessionCommandEvent, SessionDiskEvent, SessionNetworkEvent:
		return nil

	// PlaybackWriter is not used for desktop playback, so we should never see
	// these events, but skip them if a user or developer somehow tries to playback
	// a desktop session using this TTY PlaybackWriter
	case DesktopRecordingEvent:
		return nil

	// All other events get put into the general events file. These are events like
	// session.join, session.end, etc.
	default:
		return trace.Wrap(w.writeRegularEvent(event))
	}
}

func (w *SSHPlaybackWriter) writeSessionPrintEvent(event apievents.AuditEvent) error {
	print, ok := event.(*apievents.SessionPrint)
	if !ok {
		return trace.BadParameter("expected session print event, got %T", event)
	}
	w.eventIndex++
	event.SetIndex(w.eventIndex)
	if err := w.openEventsFile(0); err != nil {
		return trace.Wrap(err)
	}
	if err := w.openChunksFile(0); err != nil {
		return trace.Wrap(err)
	}
	data := print.Data
	print.Data = nil
	bytes, err := utils.FastMarshal(event)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.eventsFile.Write(append(bytes, '\n'))
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.chunksFile.Write(data)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (w *SSHPlaybackWriter) writeRegularEvent(event apievents.AuditEvent) error {
	w.eventIndex++
	event.SetIndex(w.eventIndex)
	if err := w.openEventsFile(0); err != nil {
		return trace.Wrap(err)
	}
	bytes, err := utils.FastMarshal(event)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.eventsFile.Write(append(bytes, '\n'))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (w *SSHPlaybackWriter) openIndexFile() error {
	if w.indexFile != nil {
		return nil
	}
	var err error
	w.indexFile, err = os.OpenFile(
		filepath.Join(w.dir, fmt.Sprintf("%v.index", w.sid.String())), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (w *SSHPlaybackWriter) openEventsFile(eventIndex int64) error {
	if w.eventsFile != nil {
		return nil
	}
	w.EventsPath = eventsFileName(w.dir, w.sid, "", eventIndex)

	// update the index file to write down that new events file has been created
	data, err := utils.FastMarshal(indexEntry{
		FileName: filepath.Base(w.EventsPath),
		Type:     fileTypeEvents,
		Index:    eventIndex,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = fmt.Fprintf(w.indexFile, "%v\n", string(data))
	if err != nil {
		return trace.Wrap(err)
	}

	// open new events file for writing
	file, err := os.OpenFile(w.EventsPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640)
	if err != nil {
		return trace.Wrap(err)
	}
	w.eventsFile = newGzipWriter(file)
	return nil
}

func (w *SSHPlaybackWriter) openChunksFile(offset int64) error {
	if w.chunksFile != nil {
		return nil
	}
	w.ChunksPath = chunksFileName(w.dir, w.sid, offset)

	// Update the index file to write down that new chunks file has been created.
	data, err := utils.FastMarshal(indexEntry{
		FileName: filepath.Base(w.ChunksPath),
		Type:     fileTypeChunks,
		Offset:   offset,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// index file will contain file name with extension .gz (assuming it was gzipped)
	_, err = fmt.Fprintf(w.indexFile, "%v\n", string(data))
	if err != nil {
		return trace.Wrap(err)
	}

	// open the chunks file for writing, but because the file is written without
	// compression, remove the .gz
	file, err := os.OpenFile(w.ChunksPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640)
	if err != nil {
		return trace.Wrap(err)
	}
	w.chunksFile = newGzipWriter(file)
	return nil
}
