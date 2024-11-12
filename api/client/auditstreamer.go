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

package client

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	ggzip "google.golang.org/grpc/encoding/gzip"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types/events"
)

// createOrResumeAuditStream creates or resumes audit stream described in the request.
func (c *Client) createOrResumeAuditStream(ctx context.Context, request proto.AuditStreamRequest) (events.Stream, error) {
	closeCtx, cancel := context.WithCancel(ctx)
	stream, err := c.grpc.CreateAuditStream(closeCtx, grpc.UseCompressor(ggzip.Name))
	if err != nil {
		cancel()
		return nil, trace.Wrap(err)
	}
	s := &auditStreamer{
		stream:   stream,
		statusCh: make(chan events.StreamStatus, 1),
		closeCtx: closeCtx,
		cancel:   cancel,
	}
	go s.recv()
	err = s.stream.Send(&request)
	if err != nil {
		return nil, trace.NewAggregate(s.Close(ctx), trace.Wrap(err))
	}
	return s, nil
}

// ResumeAuditStream resumes existing audit stream.
func (c *Client) ResumeAuditStream(ctx context.Context, sessionID, uploadID string) (events.Stream, error) {
	return c.createOrResumeAuditStream(ctx, proto.AuditStreamRequest{
		Request: &proto.AuditStreamRequest_ResumeStream{
			ResumeStream: &proto.ResumeStream{
				SessionID: sessionID,
				UploadID:  uploadID,
			},
		},
	})
}

// CreateAuditStream creates new audit stream.
func (c *Client) CreateAuditStream(ctx context.Context, sessionID string) (events.Stream, error) {
	return c.createOrResumeAuditStream(ctx, proto.AuditStreamRequest{
		Request: &proto.AuditStreamRequest_CreateStream{
			CreateStream: &proto.CreateStream{SessionID: sessionID},
		},
	})
}

type auditStreamer struct {
	statusCh chan events.StreamStatus
	mu       sync.RWMutex
	stream   proto.AuthService_CreateAuditStreamClient
	err      error
	closeCtx context.Context
	cancel   context.CancelFunc
}

// Close flushes non-uploaded flight stream data without marking
// the stream completed and closes the stream instance.
func (s *auditStreamer) Close(ctx context.Context) error {
	defer s.closeWithError(nil)
	return trace.Wrap(s.stream.Send(&proto.AuditStreamRequest{
		Request: &proto.AuditStreamRequest_FlushAndCloseStream{
			FlushAndCloseStream: &proto.FlushAndCloseStream{},
		},
	}))
}

// Complete completes stream.
func (s *auditStreamer) Complete(ctx context.Context) error {
	return trace.Wrap(s.stream.Send(&proto.AuditStreamRequest{
		Request: &proto.AuditStreamRequest_CompleteStream{
			CompleteStream: &proto.CompleteStream{},
		},
	}))
}

// Status returns a StreamStatus channel for the auditStreamer,
// which can be received from to interact with new updates.
func (s *auditStreamer) Status() <-chan events.StreamStatus {
	return s.statusCh
}

// RecordEvent records adds an event to a session recording.
func (s *auditStreamer) RecordEvent(ctx context.Context, event events.PreparedSessionEvent) error {
	oneof, err := events.ToOneOf(event.GetAuditEvent())
	if err != nil {
		return trace.Wrap(err)
	}
	err = trace.Wrap(s.stream.Send(&proto.AuditStreamRequest{
		Request: &proto.AuditStreamRequest_Event{Event: oneof},
	}))
	if err != nil {
		s.closeWithError(err)
		return trace.Wrap(err)
	}
	return nil
}

// Done returns channel closed when streamer is closed.
func (s *auditStreamer) Done() <-chan struct{} {
	return s.closeCtx.Done()
}

// Error returns last error of the stream.
func (s *auditStreamer) Error() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

// recv is necessary to receive errors from the
// server, otherwise no errors will be propagated.
func (s *auditStreamer) recv() {
	for {
		status, err := s.stream.Recv()
		if err != nil {
			s.closeWithError(trace.Wrap(err))
			return
		}
		select {
		case <-s.closeCtx.Done():
			return
		case s.statusCh <- *status:
		default:
		}
	}
}

func (s *auditStreamer) closeWithError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
	s.cancel()
}
