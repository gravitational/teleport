/**
 * Copyright (C) 2024 Gravitational, Inc.
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
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	recordingmetadatav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

func (h *Handler) getSessionRecordingMetadata(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite,
) (any, error) {
	sessionId := p.ByName("session_id")
	if sessionId == "" {
		return nil, trace.BadParameter("session_id is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.RecordingMetadataServiceClient().GetMetadata(r.Context(), &recordingmetadatav1.GetMetadataRequest{
		SessionId: sessionId,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return convertSessionRecordingMetadata(response.Metadata), nil
}

func (h *Handler) getSessionRecordingThumbnail(
	w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite,
) (any, error) {
	sessionId := p.ByName("session_id")
	if sessionId == "" {
		return nil, trace.BadParameter("session_id is required")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.RecordingMetadataServiceClient().GetThumbnail(r.Context(), &recordingmetadatav1.GetThumbnailRequest{
		SessionId: sessionId,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response.Thumbnail, nil
}

func convertSessionRecordingMetadata(details *recordingmetadatav1.SessionRecordingMetadata) any {
	type baseEvent struct {
		StartTime int64  `json:"startTime"`
		EndTime   int64  `json:"endTime"`
		Type      string `json:"type"`
	}

	type resizeEvent struct {
		baseEvent
		Cols int32 `json:"cols"`
		Rows int32 `json:"rows"`
	}

	type joinEvent struct {
		baseEvent
		User string `json:"user"`
	}

	type inactivityEvent struct {
		baseEvent
	}

	type thumbnail struct {
		Cols      int32  `json:"cols"`
		Rows      int32  `json:"rows"`
		CursorX   int32  `json:"cursorX"`
		CursorY   int32  `json:"cursorY"`
		Svg       string `json:"svg"`
		StartTime int64  `json:"startTime"`
		EndTime   int64  `json:"endTime"`
	}

	type response struct {
		Duration   int64         `json:"duration"`
		Thumbnails []thumbnail   `json:"thumbnails"`
		Events     []interface{} `json:"events"`
		StartCols  int32         `json:"startCols"`
		StartRows  int32         `json:"startRows"`
	}

	result := response{
		Duration:  details.Duration,
		StartCols: details.StartCols,
		StartRows: details.StartRows,
		Events:    make([]interface{}, 0, len(details.Events)),
	}

	for _, thumb := range details.Thumbnails {
		result.Thumbnails = append(result.Thumbnails, thumbnail{
			Cols:      thumb.Cols,
			Rows:      thumb.Rows,
			CursorX:   thumb.CursorX,
			CursorY:   thumb.CursorY,
			Svg:       thumb.Svg,
			StartTime: thumb.StartTime,
			EndTime:   thumb.EndTime,
		})
	}

	for _, event := range details.Events {
		base := baseEvent{
			StartTime: event.StartTime,
			EndTime:   event.EndTime,
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
