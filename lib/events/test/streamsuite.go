package test

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"

	"github.com/stretchr/testify/assert"
)

// StreamParams configures parameters of a stream test suite
type StreamParams struct {
	// PrintEvents is amount of print events to generate
	PrintEvents int64
	// ConcurrentUploads is amount of concurrent uploads
	ConcurrentUploads int
	// MinUploadBytes is minimum required upload bytes
	MinUploadBytes int64
}

// StreamSinglePart tests stream upload and subsequent download and reads the results
func StreamSinglePart(t *testing.T, handler events.MultipartHandler) {
	StreamWithParameters(t, handler, StreamParams{
		PrintEvents:    1024,
		MinUploadBytes: 1024 * 1024,
	})
}

// Stream tests stream upload and subsequent download and reads the results
func Stream(t *testing.T, handler events.MultipartHandler) {
	StreamWithParameters(t, handler, StreamParams{
		PrintEvents:       1024,
		MinUploadBytes:    1024,
		ConcurrentUploads: 2,
	})
}

// StreamManyParts tests stream upload and subsequent download and reads the results
func StreamManyParts(t *testing.T, handler events.MultipartHandler) {
	StreamWithParameters(t, handler, StreamParams{
		PrintEvents:       8192,
		MinUploadBytes:    1024,
		ConcurrentUploads: 64,
	})
}

// StreamResumeManyParts tests stream upload, failure to complete, resuming
// and subsequent download and reads the results.
func StreamResumeManyParts(t *testing.T, handler events.MultipartHandler) {
	StreamResumeWithParameters(t, handler, StreamParams{
		PrintEvents:       8192,
		MinUploadBytes:    1024,
		ConcurrentUploads: 64,
	})
}

// StreamWithParameters tests stream upload and subsequent download and reads the results
func StreamWithParameters(t *testing.T, handler events.MultipartHandler, params StreamParams) {
	ctx := context.TODO()

	inEvents := events.GenerateTestSession(events.SessionParams{PrintEvents: params.PrintEvents})
	sid := session.ID(inEvents[0].(events.SessionMetadataGetter).GetSessionID())

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:          handler,
		MinUploadBytes:    params.MinUploadBytes,
		ConcurrentUploads: params.ConcurrentUploads,
	})
	assert.Nil(t, err)

	stream, err := streamer.CreateAuditStream(ctx, sid)
	assert.Nil(t, err)

	select {
	case status := <-stream.Status():
		assert.Equal(t, status.LastEventIndex, int64(-1))
	case <-time.After(time.Second):
		t.Fatalf("Timed out waiting for status update.")
	}

	for _, event := range inEvents {
		err := stream.EmitAuditEvent(ctx, event)
		assert.Nil(t, err)
	}

	err = stream.Complete(ctx)
	assert.Nil(t, err)

	f, err := ioutil.TempFile("", string(sid))
	assert.Nil(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = handler.Download(ctx, sid, f)
	assert.Nil(t, err)

	_, err = f.Seek(0, 0)
	assert.Nil(t, err)

	reader := events.NewProtoReader(f)
	out, err := reader.ReadAll(ctx)
	assert.Nil(t, err)

	stats := reader.GetStats()
	assert.Equal(t, stats.SkippedEvents, int64(0))
	assert.Equal(t, stats.OutOfOrderEvents, int64(0))
	assert.Equal(t, stats.TotalEvents, int64(len(inEvents)))

	assert.Equal(t, inEvents, out)
}

// StreamResumeWithParameters expects initial complete attempt to fail
// but subsequent resume to succeed
func StreamResumeWithParameters(t *testing.T, handler events.MultipartHandler, params StreamParams) {
	ctx := context.TODO()

	inEvents := events.GenerateTestSession(events.SessionParams{PrintEvents: params.PrintEvents})
	sid := session.ID(inEvents[0].(events.SessionMetadataGetter).GetSessionID())

	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader:          handler,
		MinUploadBytes:    params.MinUploadBytes,
		ConcurrentUploads: params.ConcurrentUploads,
	})
	assert.Nil(t, err)

	upload, err := handler.CreateUpload(ctx, sid)
	assert.Nil(t, err)

	stream, err := streamer.CreateAuditStreamForUpload(ctx, sid, *upload)
	assert.Nil(t, err)

	for _, event := range inEvents {
		err := stream.EmitAuditEvent(ctx, event)
		assert.Nil(t, err)
	}

	err = stream.Complete(ctx)
	assert.NotNil(t, err, "First complete attempt should fail here.")

	stream, err = streamer.ResumeAuditStream(ctx, sid, upload.ID)
	assert.Nil(t, err)

	// First update always starts with -1 and indicates
	// that resume has been started successfully
	select {
	case status := <-stream.Status():
		assert.Equal(t, status.LastEventIndex, int64(-1))
	case <-time.After(time.Second):
		t.Fatalf("Timed out waiting for status update.")
	}

	err = stream.Complete(ctx)
	assert.Nil(t, err, "Complete after resume should succeed")

	f, err := ioutil.TempFile("", string(sid))
	assert.Nil(t, err)
	defer os.Remove(f.Name())
	defer f.Close()

	err = handler.Download(ctx, sid, f)
	assert.Nil(t, err)

	_, err = f.Seek(0, 0)
	assert.Nil(t, err)

	reader := events.NewProtoReader(f)
	out, err := reader.ReadAll(ctx)
	assert.Nil(t, err)

	stats := reader.GetStats()
	assert.Equal(t, stats.SkippedEvents, int64(0))
	assert.Equal(t, stats.OutOfOrderEvents, int64(0))
	assert.Equal(t, stats.TotalEvents, int64(len(inEvents)))

	assert.Equal(t, inEvents, out)
}
