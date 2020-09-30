/*
Copyright 2020 Gravitational, Inc.

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

package events

import (
	"archive/tar"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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

// WriteForPlayback reads events from audit reader
// and writes them to the format optimized for playback
func WriteForPlayback(ctx context.Context, sid session.ID, reader AuditReader, dir string) error {
	w := &PlaybackWriter{
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
	return w.Write(ctx)
}

// PlaybackWriter reads messages until end of file
// and writes them to directory in compatibility playback format
type PlaybackWriter struct {
	sid        session.ID
	dir        string
	reader     AuditReader
	indexFile  *os.File
	eventsFile *gzipWriter
	chunksFile *gzipWriter
	eventIndex int64
}

// Close closes all files
func (w *PlaybackWriter) Close() error {
	if w.indexFile != nil {
		w.indexFile.Close()
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

// Write writes the files in the format optimized for playback
func (w *PlaybackWriter) Write(ctx context.Context) error {
	if err := w.openIndexFile(); err != nil {
		return trace.Wrap(err)
	}
	for {
		event, err := w.reader.Read(ctx)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return trace.Wrap(err)
		}
		if err := w.writeEvent(event); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (w *PlaybackWriter) writeEvent(event AuditEvent) error {
	switch event.GetType() {
	// Timing events for TTY playback go to both a chunks file (the raw bytes) as
	// well as well as the events file (structured events).
	case SessionPrintEvent:
		return trace.Wrap(w.writeSessionPrintEvent(event))
		// Playback does not use enhanced events at the moment,
		// so they are skipped
	case SessionCommandEvent, SessionDiskEvent, SessionNetworkEvent:
		return nil
	// All other events get put into the general events file. These are events like
	// session.join, session.end, etc.
	default:
		return trace.Wrap(w.writeRegularEvent(event))
	}
}

func (w *PlaybackWriter) writeSessionPrintEvent(event AuditEvent) error {
	print, ok := event.(*SessionPrint)
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

func (w *PlaybackWriter) writeRegularEvent(event AuditEvent) error {
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

func (w *PlaybackWriter) openIndexFile() error {
	if w.indexFile != nil {
		return nil
	}
	var err error
	w.indexFile, err = os.OpenFile(
		filepath.Join(w.dir, fmt.Sprintf("%v.index", w.sid.String())), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (w *PlaybackWriter) openEventsFile(eventIndex int64) error {
	if w.eventsFile != nil {
		return nil
	}
	eventsFileName := eventsFileName(w.dir, w.sid, "", eventIndex)

	// update the index file to write down that new events file has been created
	data, err := utils.FastMarshal(indexEntry{
		FileName: filepath.Base(eventsFileName),
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
	file, err := os.OpenFile(eventsFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	w.eventsFile = newGzipWriter(file)
	return nil
}

func (w *PlaybackWriter) openChunksFile(offset int64) error {
	if w.chunksFile != nil {
		return nil
	}
	chunksFileName := chunksFileName(w.dir, w.sid, offset)

	// Update the index file to write down that new chunks file has been created.
	data, err := utils.FastMarshal(indexEntry{
		FileName: filepath.Base(chunksFileName),
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
	file, err := os.OpenFile(chunksFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	w.chunksFile = newGzipWriter(file)
	return nil
}
