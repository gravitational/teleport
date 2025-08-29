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

package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"google.golang.org/protobuf/types/known/durationpb"

	recordingmetadatav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

type sessionRecordingMessageType string

const (
	recordingThumbnailMessageType sessionRecordingMessageType = "thumbnail"
	recordingMetadataMessageType  sessionRecordingMessageType = "metadata"
	recordingErrorMessageType     sessionRecordingMessageType = "error"
)

type sessionRecordingErrorResponse struct {
	Error string `json:"error"`
}

// sessionRecordingMessageWrapper is a wrapper for session recording messages sent over WebSocket.
// This makes it easier to have strongly typed messages on the frontend, switching on the `Type` field.
type sessionRecordingMessageWrapper struct {
	Type sessionRecordingMessageType `json:"type"`
	Data any                         `json:"data"`
}

// getSessionRecordingMetadata handles the WebSocket connection to stream session recording metadata and thumbnails.
// The metadata is loaded over a websocket connection to avoid gRPC message size limits.
// It sends metadata and thumbnails as JSON messages to the client.
func (h *Handler) getSessionRecordingMetadata(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	cluster reversetunnelclient.Cluster,
	ws *websocket.Conn,
) (interface{}, error) {
	sessionID := p.ByName("session_id")
	if sessionID == "" {
		return nil, trace.BadParameter("missing session ID in request URL")
	}

	ctx := r.Context()
	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		sendMessage(ws, recordingErrorMessageType, sessionRecordingErrorResponse{
			Error: err.Error(),
		})
		return nil, nil
	}

	stream, err := clt.RecordingMetadataServiceClient().GetMetadata(ctx, &recordingmetadatav1.GetMetadataRequest{
		SessionId: sessionID,
	})
	if err != nil {
		sendMessage(ws, recordingErrorMessageType, sessionRecordingErrorResponse{
			Error: err.Error(),
		})
		return nil, nil
	}

	metadata, err := stream.Recv()
	if err != nil {
		if trace.IsNotFound(err) {
			sendMessage(ws, recordingErrorMessageType, sessionRecordingErrorResponse{
				Error: fmt.Sprintf("metadata for session %q not found", sessionID),
			})
		} else {
			h.logger.ErrorContext(ctx, "failed to receive metadata", "session_id", sessionID, "error", err)
			sendMessage(ws, recordingErrorMessageType, sessionRecordingErrorResponse{
				Error: err.Error(),
			})
		}
		return nil, nil
	}

	if metadata.GetMetadata() == nil {
		h.logger.ErrorContext(ctx, "received nil metadata in stream", "session_id", sessionID)
		sendMessage(ws, recordingErrorMessageType, sessionRecordingErrorResponse{
			Error: trace.BadParameter("received nil metadata").Error(),
		})
		return nil, nil
	}

	if err := sendMessage(ws, recordingMetadataMessageType, encodeSessionRecordingMetadata(metadata.GetMetadata())); err != nil {
		h.logger.ErrorContext(ctx, "failed to send metadata", "session_id", sessionID, "error", err)
		return nil, nil
	}

	for {
		chunk, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			h.logger.ErrorContext(ctx, "failed to receive chunk", "session_id", sessionID, "error", err)
			sendMessage(ws, recordingErrorMessageType, sessionRecordingErrorResponse{
				Error: err.Error(),
			})
			return nil, nil
		}

		if chunk.GetFrame() == nil {
			h.logger.ErrorContext(ctx, "received nil frame in metadata stream")
			sendMessage(ws, recordingErrorMessageType, sessionRecordingErrorResponse{
				Error: trace.BadParameter("received nil frame").Error(),
			})
			return nil, nil
		}

		if err := sendMessage(ws, recordingThumbnailMessageType, encodeSessionRecordingThumbnail(chunk.GetFrame())); err != nil {
			h.logger.ErrorContext(ctx, "failed to send thumbnail", "session_id", sessionID, "error", err)
			return nil, nil
		}
	}

	return nil, nil
}

func sendMessage(ws *websocket.Conn, msgType sessionRecordingMessageType, data interface{}) error {
	return ws.WriteJSON(sessionRecordingMessageWrapper{
		Type: msgType,
		Data: data,
	})
}

func (h *Handler) getSessionRecordingThumbnail(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster,
) (any, error) {
	sessionId := p.ByName("session_id")
	if sessionId == "" {
		return nil, trace.BadParameter("session_id is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.RecordingMetadataServiceClient().GetThumbnail(r.Context(), &recordingmetadatav1.GetThumbnailRequest{
		SessionId: sessionId,
	})
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("thumbnail not found for session %q", sessionId)
		}
		return nil, trace.Wrap(err)
	}

	if response.Thumbnail == nil {
		return nil, trace.NotFound("thumbnail not found for session %q", sessionId)
	}

	return encodeSessionRecordingThumbnail(response.Thumbnail), nil
}

type baseEvent struct {
	StartOffset int64  `json:"startTime"`
	EndOffset   int64  `json:"endTime"`
	Type        string `json:"type"`
}

type resizeEvent struct {
	baseEvent
	Cols int32 `json:"cols"`
	Rows int32 `json:"rows"`
}

func (resizeEvent) isSessionRecordingEvent() {}

type joinEvent struct {
	baseEvent
	User string `json:"user"`
}

func (joinEvent) isSessionRecordingEvent() {}

type inactivityEvent struct {
	baseEvent
}

func (inactivityEvent) isSessionRecordingEvent() {}

type sessionRecordingEvent interface {
	isSessionRecordingEvent()
}

type sessionRecordingMetadata struct {
	Duration     int64                   `json:"duration"`
	Events       []sessionRecordingEvent `json:"events"`
	StartCols    int32                   `json:"startCols"`
	StartRows    int32                   `json:"startRows"`
	StartTime    int64                   `json:"startTime"`
	EndTime      int64                   `json:"endTime"`
	ClusterName  string                  `json:"clusterName"`
	ResourceName string                  `json:"resourceName,omitempty"`
	User         string                  `json:"user,omitempty"`
	Type         string                  `json:"type,omitempty"`
}

func pbTypeToString(t recordingmetadatav1.SessionRecordingType) string {
	switch t {
	case recordingmetadatav1.SessionRecordingType_SESSION_RECORDING_TYPE_SSH:
		return "ssh"
	case recordingmetadatav1.SessionRecordingType_SESSION_RECORDING_TYPE_KUBERNETES:
		return "k8s"
	default:
		return "unknown"
	}
}

// encodeSessionRecordingMetadata converts the session recording metadata to a format more suitable for the frontend
// to use.
func encodeSessionRecordingMetadata(metadata *recordingmetadatav1.SessionRecordingMetadata) sessionRecordingMetadata {
	result := sessionRecordingMetadata{
		Duration:     convertDurationToMs(metadata.Duration),
		StartCols:    metadata.StartCols,
		StartRows:    metadata.StartRows,
		Events:       make([]sessionRecordingEvent, 0, len(metadata.Events)),
		StartTime:    metadata.StartTime.AsTime().Unix(),
		EndTime:      metadata.EndTime.AsTime().Unix(),
		ClusterName:  metadata.ClusterName,
		ResourceName: metadata.ResourceName,
		User:         metadata.User,
		Type:         pbTypeToString(metadata.Type),
	}

	for _, event := range metadata.Events {
		base := baseEvent{
			StartOffset: convertDurationToMs(event.StartOffset),
			EndOffset:   convertDurationToMs(event.EndOffset),
		}

		switch e := event.Event.(type) {
		case *recordingmetadatav1.SessionRecordingEvent_Inactivity:
			base.Type = "inactivity"
			result.Events = append(result.Events, inactivityEvent{baseEvent: base})
		case *recordingmetadatav1.SessionRecordingEvent_Join:
			base.Type = "join"
			result.Events = append(result.Events, joinEvent{
				baseEvent: base,
				User:      e.Join.User,
			})
		case *recordingmetadatav1.SessionRecordingEvent_Resize:
			base.Type = "resize"
			result.Events = append(result.Events, resizeEvent{
				baseEvent: base,
				Cols:      e.Resize.Cols,
				Rows:      e.Resize.Rows,
			})
		}
	}

	return result
}

type sessionRecordingThumbnailResponse struct {
	Svg           string `json:"svg"`
	Cols          int32  `json:"cols"`
	Rows          int32  `json:"rows"`
	CursorX       int32  `json:"cursorX"`
	CursorY       int32  `json:"cursorY"`
	CursorVisible bool   `json:"cursorVisible"`
	StartOffset   int64  `json:"startOffset"`
	EndOffset     int64  `json:"endOffset"`
}

// encodeSessionRecordingThumbnail converts the session recording thumbnail to a format more suitable for the frontend.
func encodeSessionRecordingThumbnail(thumbnail *recordingmetadatav1.SessionRecordingThumbnail) sessionRecordingThumbnailResponse {
	return sessionRecordingThumbnailResponse{
		Svg:           string(thumbnail.Svg),
		Cols:          thumbnail.Cols,
		Rows:          thumbnail.Rows,
		CursorX:       thumbnail.CursorX,
		CursorY:       thumbnail.CursorY,
		CursorVisible: thumbnail.CursorVisible,
		StartOffset:   convertDurationToMs(thumbnail.StartOffset),
		EndOffset:     convertDurationToMs(thumbnail.EndOffset),
	}
}

func convertDurationToMs(d *durationpb.Duration) int64 {
	if d == nil {
		return 0
	}
	return d.Seconds*1000 + int64(d.Nanos/1000000)
}
