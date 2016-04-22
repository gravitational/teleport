package events

import (
	"github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/session"
)

func infof(format string, params ...interface{}) {
	logrus.Infof("-----> "+format, params...)
}

// AuditLog is a new combined facility to record Teleport events and
// sessions. It implements these interfaces:
//	- events.Log
//	- recorder.Recorder
//  - recorder.ChunkWriter
type AuditLog struct {
}

func NewAuditLog() (*AuditLog, error) {
	return &AuditLog{}, nil
}

func (l *AuditLog) Log(id lunk.EventID, e lunk.Event) {
	l.LogEntry(lunk.NewEntry(id, e))
}

func (l *AuditLog) LogEntry(entry lunk.Entry) error {
	infof("LogEntry() -> %s: %v", entry.String(), entry.Properties)
	return nil
}

func (l *AuditLog) LogSession(s session.Session) error {
	infof("LogSession() -> %v", s.ID)
	return nil
}

func (l *AuditLog) GetEvents(filter Filter) ([]lunk.Entry, error) {
	return nil, nil
}

func (l *AuditLog) GetSessionEvents(filter Filter) ([]session.Session, error) {
	return nil, nil
}

func (l *AuditLog) Close() error {
	infof("Close()")
	return nil
}

// GetChunkWriter returns a new writer that can record
// chunks with active session data to the recording server
func (l *AuditLog) GetChunkWriter(id string) (recorder.ChunkWriteCloser, error) {
	infof("GetChunkWriter(%s)", id)
	return l, nil
}

// GetChunkReader returns a reader of recorded chunks
func (l *AuditLog) GetChunkReader(id string) (recorder.ChunkReadCloser, error) {
	return l, nil
}

func (l *AuditLog) WriteChunks(chunks []recorder.Chunk) error {
	lc := len(chunks)
	tl := 0
	for i := range chunks {
		tl += len(chunks[i].Data)
	}
	infof("WriteChunks(%v chunks, total len: %v)", lc, tl)
	return nil
}

func (l *AuditLog) ReadChunks(start int, end int) ([]recorder.Chunk, error) {
	infof("ReadChunks(%v, %v)", start, end)
	return make([]recorder.Chunk, 0), nil
}

func (l *AuditLog) GetChunksCount() (uint64, error) {
	infof("GetChunkCount()")
	return 0, nil
}
