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
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/auth/proto"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"

	"github.com/sirupsen/logrus"
)

const (
	// typeInit causes a read/write pipe and tar archive to be created and
	// the upload started.
	typeInit = 0

	// typeOpenRaw writes the tar header for a new file that will be streamed
	// chunk by chunk in the tar archive.
	typeOpenRaw = 1

	// typeChunkRaw will write a chunk of the file to the tar archive. Note
	// that no processing occurs on the chunk.
	typeChunkRaw = 2

	// typeCloseRaw will close the file in the tar archive. For raw files this
	// is a NOP.
	typeCloseRaw = 3

	// typeOpenEvents create a in-memory gzip archive into which all chunks will
	// be written.
	typeOpenEvents = 4

	// typeChunkEvents will validate and then write the chunk into the
	// gzip archive.
	typeChunkEvents = 5

	// typeCloseEvents will write the zip archive into the tar archive.
	typeCloseEvents = 6

	// typeComplete will close the tar archive and close the writer to signal
	// to the uploader that the file is complete.
	typeComplete = 7
)

// SessionUploader is an interface implemented by the Audit Log to upload
// a session recording.
type SessionUploader interface {
	UploadSessionRecording(r SessionRecording) error
}

// StreamManager holds resources used in common by each stream like a pool of
// buffers and semaphore to apply backpressure.
type StreamManager struct {
	log *logrus.Entry

	// pool is used to store a pool of buffers used to build in-memory gzip files.
	pool sync.Pool

	// semaphore is used to limit the number of in-memory gzip files.
	semaphore chan struct{}

	// closeContext is used to send a signal to the stream manager that the
	// process is shutting down.
	closeContext context.Context
}

// Stream pulls and processes events off the GRPC stream.
type Stream struct {
	// manager holds common resources across all streams.
	manager *StreamManager

	// chunkType is the last type of chunk that was processed by the stream.
	chunkType int64

	// rawChunkCount keeps track of how many raw event chunks were processed.
	rawChunkCount int64

	// eventsChunkCount keeps track of how many event chunks were processed.
	eventsChunkCount int64

	// uploader implements UploadSessionRecording on the Auth Server.
	uploader SessionUploader

	// stream is the GRPC stream off of which events are consumed.
	stream proto.AuthService_StreamSessionRecordingServer

	// serverID is the identity of the server extracted from the x509 certificate.
	serverID string

	// sessionID is the unique ID of the session.
	sessionID string

	// writer is a io.Pipe writer over which writes to the tarball are done.
	writer io.WriteCloser

	// reader is a io.Pipe reader over which reads from the tarball are read
	// by the uploader.
	reader io.ReadCloser

	// tarWriter is used to create the archive itself.
	tarWriter *tar.Writer

	// zipBuffer
	zipBuffer *bytes.Buffer

	// zipWriter is used to create the zip files within the archive.
	zipWriter *gzip.Writer

	// uploadContext is used to cancel the tarball upload.
	uploadContext context.Context

	// uploadCancel is used to cancel the tarball upload.
	uploadCancel context.CancelFunc

	// waitCh is used to unblock when the upload completes and return
	// the error (or nil).
	waitCh chan error
}

// NewStreamManger is used to manage common stream resources like a pool of
// buffers and a semaphore.
func NewStreamManger(ctx context.Context) *StreamManager {
	// If no context is passed in (like in tests) then set a background context.
	if ctx == nil {
		ctx = context.Background()
	}
	return &StreamManager{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: "stream",
		}),
		pool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
		semaphore:    make(chan struct{}, defaults.ConcurrentStreams),
		closeContext: ctx,
	}
}

// ProcessStream reads events off a GRPC stream and processes them.
func (s *StreamManager) ProcessStream(serverID string, uploader SessionUploader, stream proto.AuthService_StreamSessionRecordingServer) (*Stream, error) {
	// Wrap the parent process shutdown context and create an upload context that is
	// used by the uploader to cancel a upload if an error occurs while creating
	// the archive.
	ctx, cancel := context.WithCancel(s.closeContext)

	st := &Stream{
		manager:       s,
		stream:        stream,
		chunkType:     typeInit,
		uploader:      uploader,
		serverID:      serverID,
		uploadContext: ctx,
		uploadCancel:  cancel,
		waitCh:        make(chan error),
	}

	// Start reading off the stream and processing events.
	go st.start()

	return st, nil
}

// takeSemaphore will acquire the semaphore.
func (s *StreamManager) takeSemaphore() error {
	select {
	case s.semaphore <- struct{}{}:
		return nil
	case <-s.closeContext.Done():
		return errContext
	}
}

// releaseSemaphore will release the semaphore.
func (s *StreamManager) releaseSemaphore() error {
	select {
	case <-s.semaphore:
		return nil
	case <-s.closeContext.Done():
		return errContext
	}
}

// Wait will block until the stream has been processed and a response written
// to the client.
func (s *Stream) Wait() error {
	return <-s.waitCh
}

// Close will terminate the stream and write a response to the client.
func (s *Stream) Close(err error) {
	// Either the upload is complete (call cancel to free resources) or an error
	// has occured (terminate upload), either way cancel the context.
	s.uploadCancel()

	// Close any resources (if not already closed) that may have been allocated.
	if s.reader != nil {
		s.reader.Close()
	}
	if s.writer != nil {
		s.writer.Close()
	}
	if s.tarWriter != nil {
		s.tarWriter.Close()
	}
	if s.zipWriter != nil {
		s.zipWriter.Close()
	}

	// Send a response to the client with the result of the stream.
	if err != nil {
		err = s.stream.SendAndClose(&proto.StreamSessionResponse{
			Success: false,
			Message: err.Error(),
		})
	} else {
		err = s.stream.SendAndClose(&proto.StreamSessionResponse{
			Success: true,
		})
	}
	if err != nil {
		s.manager.log.Debugf("Failed to close stream %v: %v.", s.sessionID, err)
	}

	// Unblock the waiter with the result.
	s.waitCh <- trace.Wrap(err)
}

// start pulls events off the stream and processes them.
func (s *Stream) start() {
	for {
		// Pull a chunk off the stream.
		chunk, err := s.stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			s.Close(trail.ToGRPC(err))
			return
		}

		// Process chunk. If an error occurs, the server will close the stream
		// and send the error to the client.
		err = s.process(chunk)
		if err != nil {
			s.Close(trail.ToGRPC(err))
			return
		}
	}
}

// process will construct a tar archive from the messages in the stream.
func (s *Stream) process(chunk *proto.SessionChunk) error {
	// Check that the chunk transitions are sane.
	err := s.checkTransition(chunk)
	if err != nil {
		return trace.Wrap(err)
	}
	s.chunkType = chunk.GetType()

	switch chunk.GetType() {
	case typeInit:
		s.sessionID = chunk.GetSessionID()
		s.manager.log.Debugf("Initialized stream processing for %v.", s.sessionID)

		// Create a streaming tar reader/writer to reduce how much of the archive
		// is buffered in memory.
		s.reader, s.writer = io.Pipe()
		s.tarWriter = tar.NewWriter(s.writer)

		// Kick off the upload in a goroutine so it can be uploaded as it
		// is processed.
		go s.upload(chunk.GetNamespace(), session.ID(chunk.GetSessionID()), s.reader)
	case typeComplete:
		s.manager.log.Debugf("Stream %v complete. %v raw chunks processed, %v event chunk processed.",
			s.sessionID, s.rawChunkCount, s.eventsChunkCount)

		// Finish the archive by writing the trailer.
		err = s.tarWriter.Close()
		if err != nil {
			return trace.Wrap(err)
		}
		s.tarWriter = nil

		// Close the writer to signal that the file is done.
		err = s.writer.Close()
		if err != nil {
			return trace.Wrap(err)
		}
		s.writer = nil
	// Raw events are directly streamed into the tar archive.
	case typeOpenRaw, typeCloseRaw, typeChunkRaw:
		err = s.processRaw(chunk)
		if err != nil {
			return trace.Wrap(err)
		}
	// Events are aggregated into a gzip archive in memory first, then streamed
	// to the tar archive.
	case typeOpenEvents, typeCloseEvents, typeChunkEvents:
		err = s.processEvents(chunk)
		if err != nil {
			return trace.Wrap(err)
		}
	// Reject all unknown event types.
	default:
		return trace.BadParameter("unknown event type %v", chunk.GetType())
	}

	return nil
}

// processRaw takes chunks and directly streams them into the tar archive.
func (s *Stream) processRaw(chunk *proto.SessionChunk) error {
	var err error

	switch chunk.GetType() {
	// Open the tar archive by writing the header. Since this is a raw stream
	// the size of the content to be written is known.
	case typeOpenRaw:
		err := s.tarWriter.WriteHeader(&tar.Header{
			Name: chunk.GetFileName(),
			Mode: 0600,
			Size: chunk.GetFileSize(),
		})
		if err != nil {
			return trace.Wrap(err)
		}
	// Close is a NOP because writing a header indicates the size of file and
	// where the next file starts.
	case typeCloseRaw:
	// Chunk can be written directly to the tar archive.
	case typeChunkRaw:
		s.rawChunkCount = s.rawChunkCount + 1

		_, err = s.tarWriter.Write(chunk.GetData())
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// processEvents takes chunks, validates them, and then buffers them in a
// gzip stream until complete then writes them to the tar archive.
func (s *Stream) processEvents(chunk *proto.SessionChunk) error {
	var err error

	switch chunk.GetType() {
	case typeOpenEvents:
		// Take semaphore before creating zip archive in memory.
		err = s.manager.takeSemaphore()
		if err != nil {
			return trace.Wrap(err)
		}

		// Get a buffer from the pool.
		s.zipBuffer = s.manager.pool.Get().(*bytes.Buffer)

		s.zipWriter, err = gzip.NewWriterLevel(s.zipBuffer, gzip.BestSpeed)
		if err != nil {
			return trace.Wrap(err)
		}
	case typeCloseEvents:
		// Close zip file and after writing it to the tar archive, release
		// any resources.
		err = s.zipWriter.Close()
		if err != nil {
			return trace.Wrap(err)
		}
		s.zipWriter = nil
		defer s.zipBuffer.Reset()
		defer s.manager.pool.Put(s.zipBuffer)

		// Copy the zip archive into the tar stream.
		err := s.tarWriter.WriteHeader(&tar.Header{
			Name: chunk.GetFileName(),
			Mode: 0600,
			Size: int64(s.zipBuffer.Len()),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = io.Copy(s.tarWriter, s.zipBuffer)
		if err != nil {
			return trace.Wrap(err)
		}

		// Done processing zip archive, release semaphore.
		err = s.manager.releaseSemaphore()
		if err != nil {
			return trace.Wrap(err)
		}
	case typeChunkEvents:
		s.eventsChunkCount = s.eventsChunkCount + 1

		// Validate incoming event.
		var f EventFields
		err = utils.FastUnmarshal(chunk.GetData(), &f)
		if err != nil {
			return trace.Wrap(err)
		}
		err := ValidateEvent(f, s.serverID)
		if err != nil {
			s.manager.log.Warnf("Rejecting audit event %v from %v: %v. A node is attempting to "+
				"submit events for an identity other than the one on its x509 certificate.",
				f.GetType(), s.serverID, err)
			return trace.AccessDenied("failed to validate event")
		}

		// Write event to zip buffer.
		_, err = s.zipWriter.Write(append(chunk.GetData(), '\n'))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// checkTransition makes sure the archive is being created in a sane manner.
func (s *Stream) checkTransition(chunk *proto.SessionChunk) error {
	prev := s.chunkType

	switch chunk.GetType() {
	case typeInit:
		return nil
	case typeOpenRaw, typeOpenEvents:
		if prev == typeInit || prev == typeCloseRaw || prev == typeCloseEvents {
			return nil
		}
	case typeChunkRaw:
		if prev == typeChunkRaw || prev == typeOpenRaw {
			return nil
		}
	case typeCloseRaw:
		if prev == typeChunkRaw {
			return nil
		}
	case typeChunkEvents:
		if prev == typeChunkEvents || prev == typeOpenEvents {
			return nil
		}
	case typeCloseEvents:
		if prev == typeChunkEvents {
			return nil
		}
	case typeComplete:
		if prev == typeCloseRaw || prev == typeCloseEvents {
			return nil
		}
	}

	return trace.BadParameter("invalid chunk transition from %v to %v", prev, chunk.GetType())
}

// upload will call the Audit Log to upload the session recording and then
// close and reply to the client.
func (s *Stream) upload(namespace string, sessionID session.ID, reader io.Reader) {
	err := s.uploader.UploadSessionRecording(SessionRecording{
		CancelContext: s.uploadContext,
		SessionID:     sessionID,
		Namespace:     namespace,
		Recording:     reader,
	})
	if err != nil {
		s.manager.log.Warnf("Failed to upload session recording: %v.", err)
		return
	}

	// The upload is complete, write nil to unblock.
	s.Close(nil)
}

// StreamSessionRecording is called by the client to send a session recording
// to the server.
func StreamSessionRecording(clt proto.AuthServiceClient, r SessionRecording) error {
	// Open the session stream to the Auth Server.
	stream, err := clt.StreamSessionRecording(context.Background())
	if err != nil {
		return trail.FromGRPC(err)
	}

	// Initialize stream.
	err = stream.Send(&proto.SessionChunk{
		Type:      typeInit,
		Namespace: r.Namespace,
		SessionID: r.SessionID.String(),
	})
	if err != nil {
		return trail.FromGRPC(err)
	}

	// Open the tarball for reading, some content (like chunks) will be sent
	// raw and some uncompressed and sent.
	tr := tar.NewReader(r.Recording)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return trace.Wrap(err)
		}

		// All files that end with an events suffix are opened then sent.
		isEvents := strings.HasSuffix(header.Name, eventsSuffix)

		// Send file open chunk.
		err = sendOpenEvent(stream, header, isEvents)
		if err != nil {
			return trace.Wrap(err)
		}

		// Send content chunks. Raw chunks will be sent as-is, event chunks are
		// un-compressed and sent so they can be validated and the archive
		// re-constructed.
		if !isEvents {
			err = sendRawChunks(stream, tr)
		} else {
			err = sendEventChunks(stream, tr)
		}
		if err != nil {
			return trace.Wrap(err)
		}

		// Send file close chunk.
		err = sendCloseEvent(stream, header, isEvents)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Send complete event.
	err = stream.Send(&proto.SessionChunk{
		Type: typeComplete,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Close the stream and get response. An error is returned if a problem
	// occured trying to construct the tar archive or if an invalid event
	// was sent.
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return trail.FromGRPC(err)
	}
	if resp.GetSuccess() == false {
		return trace.BadParameter(resp.GetMessage())
	}

	return nil
}

// sendOpenEvent sends either a stateOpenRaw or stateOpenEvents chunk.
func sendOpenEvent(stream proto.AuthService_StreamSessionRecordingClient, header *tar.Header, isEvents bool) error {
	chunkType := typeOpenRaw
	if isEvents {
		chunkType = typeOpenEvents
	}

	err := stream.Send(&proto.SessionChunk{
		Type:     int64(chunkType),
		FileName: header.Name,
		FileSize: header.Size,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}

	return nil
}

// sendCloseEvent sends either a stateCloseRaw or stateCloseEvents chunk.
func sendCloseEvent(stream proto.AuthService_StreamSessionRecordingClient, header *tar.Header, isEvents bool) error {
	chunkType := typeCloseRaw
	if isEvents {
		chunkType = typeCloseEvents
	}

	err := stream.Send(&proto.SessionChunk{
		Type:     int64(chunkType),
		FileName: header.Name,
		FileSize: header.Size,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// sendRawChunks breaks and streams file in 1 MB chunks.
func sendRawChunks(stream proto.AuthService_StreamSessionRecordingClient, reader io.Reader) error {
	var fileDone bool

	for {
		// Read in one megabyte at a time until the end of the file.
		data := make([]byte, 4096)
		n, err := reader.Read(data)
		if err != nil && err != io.EOF {
			return trace.Wrap(err)
		}
		if err == io.EOF {
			fileDone = true
		}

		// Send raw file chunk.
		if len(data) > 0 {
			err = stream.Send(&proto.SessionChunk{
				Type: typeChunkRaw,
				Data: data[:n],
			})
			if err != nil {
				return trace.Wrap(err)
			}
		}

		// Exit out if no more data to be read or the file is done (got io.EOF).
		if len(data) == 0 || fileDone {
			break
		}
	}

	return nil
}

// sendEventChunks sends the events file one line at a time to allow the
// server to validate each incoming event.
func sendEventChunks(stream proto.AuthService_StreamSessionRecordingClient, reader io.Reader) error {
	// Wrap the reader in a gzip reader to uncompress the archive.
	zr, err := gzip.NewReader(reader)
	if err != nil {
		return trace.Wrap(err)
	}
	defer zr.Close()

	// Loop over file line by line.
	scanner := bufio.NewScanner(zr)
	for scanner.Scan() {
		// Send event chunk.
		err = stream.Send(&proto.SessionChunk{
			Type: typeChunkEvents,
			Data: scanner.Bytes(),
		})
		if err != nil {
			return trail.FromGRPC(err)
		}
	}
	err = scanner.Err()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
