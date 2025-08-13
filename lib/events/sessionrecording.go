/**
* Copyright (C) 2025 Gravitational, Inc.
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

//import (
//	"cmp"
//	"context"
//	"math/rand"
//	"strconv"
//	"strings"
//	"time"
//
//	"github.com/gravitational/trace"
//
//	"github.com/gravitational/teleport/api/types"
//	"github.com/gravitational/teleport/api/types/events"
//	"github.com/gravitational/teleport/lib/session"
//	"github.com/gravitational/teleport/lib/web/ttyplayback/terminal"
//	"github.com/gravitational/teleport/lib/web/ttyplayback/vt10x"
//)
//
//type processedSession struct {
//	kind            types.SessionKind
//	missingEndEvent bool
//	startEvent      events.AuditEvent
//	details         types.SessionRecordingMetadata
//	thumbnail       *types.SessionRecordingThumbnail
//	lastEvent       events.AuditEvent
//	participants    []string
//}
//
//const (
//	// inactivityThreshold is the duration after which an inactivity event is recorded.
//	inactivityThreshold = 5 * time.Second
//
//	// maxThumbnails is the maximum number of thumbnails to store in the session details.
//	maxThumbnails = 1000
//)
//
//func processSessionEvents(ctx context.Context, sessionID session.ID, evts chan events.AuditEvent, errors chan error) (*processedSession, error) {
//	var lastEvent events.AuditEvent
//	var lastEventTime time.Time
//	var nextThumbnailTime time.Time
//
//	thumbnailInterval := 1 * time.Second
//	activeUsers := make(map[string]int64)
//
//	vt := vt10x.New()
//
//	processed := processedSession{
//		missingEndEvent: true,
//		details: types.SessionRecordingMetadata{
//			Thumbnails: make([]*types.SessionRecordingThumbnail, 0),
//			Events:     make([]*types.SessionRecordingEvent, 0),
//		},
//	}
//
//	addInactivityEvent := func(start, end time.Time) {
//		if start.IsZero() || end.IsZero() {
//			return
//		}
//
//		inactivityStart := int64(start.Sub(processed.startEvent.GetTime()) / time.Millisecond)
//		inactivityEnd := int64(end.Sub(processed.startEvent.GetTime()) / time.Millisecond)
//
//		processed.details.Events = append(processed.details.Events, &types.SessionRecordingEvent{
//			StartTime: inactivityStart,
//			EndTime:   inactivityEnd,
//			Event: &types.SessionRecordingEvent_Inactivity{
//				Inactivity: &types.SessionRecordingInactivityEvent{},
//			},
//		})
//	}
//
//	recordThumbnail := func(start, end time.Time) {
//		serialized := terminal.Serialize(vt)
//
//		thumbnailStart := int64(start.Sub(processed.startEvent.GetTime()) / time.Millisecond)
//		thumbnailEnd := int64(end.Sub(processed.startEvent.GetTime()) / time.Millisecond)
//
//		processed.details.Thumbnails = append(processed.details.Thumbnails, &types.SessionRecordingThumbnail{
//			Screen: &types.SerializedTerminal{
//				Cols:    int32(serialized.Cols),
//				Rows:    int32(serialized.Rows),
//				CursorX: int32(serialized.CursorX),
//				CursorY: int32(serialized.CursorY),
//				Data:    serialized.Data,
//			},
//			StartTime: thumbnailStart,
//			EndTime:   thumbnailEnd,
//		})
//	}
//
//loop:
//	for {
//		select {
//		case evt, more := <-evts:
//			if !more {
//				break loop
//			}
//
//			lastEvent = evt
//
//			switch e := evt.(type) {
//			case *events.SessionEnd:
//				processed.missingEndEvent = false
//				processed.details.Duration = int64(e.EndTime.Sub(e.StartTime) / time.Millisecond)
//
//				if lastEventTime != (time.Time{}) && e.Time.Sub(lastEventTime) > inactivityThreshold {
//					addInactivityEvent(lastEventTime, e.Time)
//				}
//
//				recordThumbnail(e.EndTime, e.EndTime)
//
//				endTime := int64(e.EndTime.Sub(processed.startEvent.GetTime()) / time.Millisecond)
//
//				for user, startTime := range activeUsers {
//					processed.details.Events = append(processed.details.Events, &types.SessionRecordingEvent{
//						StartTime: startTime,
//						EndTime:   endTime,
//						Event: &types.SessionRecordingEvent_Join{
//							Join: &types.SessionRecordingJoinEvent{
//								User: user,
//							},
//						},
//					})
//				}
//
//			case *events.WindowsDesktopSessionEnd:
//				processed.missingEndEvent = false
//				processed.details.Duration = int64(e.EndTime.Sub(e.StartTime) / time.Millisecond)
//
//			case *events.WindowsDesktopSessionStart:
//				processed.kind = types.WindowsDesktopSessionKind
//				processed.startEvent = e
//
//			case *events.SessionStart:
//				processed.kind = types.SSHSessionKind
//				processed.startEvent = e
//
//				lastEventTime = e.Time
//
//				parts := strings.Split(e.TerminalSize, ":")
//
//				if len(parts) == 2 {
//					cols, cErr := strconv.Atoi(parts[0])
//					rows, rErr := strconv.Atoi(parts[1])
//
//					if cmp.Or(cErr, rErr) == nil {
//						processed.details.StartCols = int32(cols)
//						processed.details.StartRows = int32(rows)
//
//						vt.Resize(cols, rows)
//					}
//				}
//
//			case *events.SessionJoin:
//				processed.participants = append(processed.participants, e.User)
//
//				activeUsers[e.User] = int64(e.Time.Sub(processed.startEvent.GetTime()) / time.Millisecond)
//
//			case *events.SessionLeave:
//				if startTime, ok := activeUsers[e.User]; ok {
//					processed.details.Events = append(processed.details.Events, &types.SessionRecordingEvent{
//						StartTime: startTime,
//						EndTime:   int64(e.Time.Sub(processed.startEvent.GetTime()) / time.Millisecond),
//						Event: &types.SessionRecordingEvent_Join{
//							Join: &types.SessionRecordingJoinEvent{
//								User: e.User,
//							},
//						},
//					})
//
//					delete(activeUsers, e.User)
//				}
//
//			case *events.SessionPrint:
//				if lastEventTime != (time.Time{}) && e.Time.Sub(lastEventTime) > inactivityThreshold {
//					addInactivityEvent(lastEventTime, e.Time)
//				}
//
//				if _, err := vt.Write(e.Data); err != nil {
//					return nil, trace.Errorf("failed to write data to terminal: %w", err)
//				}
//
//				if e.Time.After(nextThumbnailTime) {
//					startTime := e.Time
//					endTime := e.Time.Add(thumbnailInterval).Add(-1 * time.Millisecond)
//
//					recordThumbnail(startTime, endTime)
//
//					nextThumbnailTime = e.Time.Add(thumbnailInterval)
//				}
//
//			case *events.Resize:
//				parts := strings.Split(e.TerminalSize, ":")
//
//				if len(parts) == 2 {
//					cols, cErr := strconv.Atoi(parts[0])
//					rows, rErr := strconv.Atoi(parts[1])
//
//					if cmp.Or(cErr, rErr) == nil {
//						processed.details.Events = append(processed.details.Events, &types.SessionRecordingEvent{
//							StartTime: int64(e.Time.Sub(processed.startEvent.GetTime()) / time.Millisecond),
//							Event: &types.SessionRecordingEvent_Resize{
//								Resize: &types.SessionRecordingResizeEvent{
//									Cols: int32(cols),
//									Rows: int32(rows),
//								},
//							},
//						})
//
//						vt.Resize(cols, rows)
//					}
//				}
//			}
//
//		case err := <-errors:
//			return nil, trace.Wrap(err)
//		case <-ctx.Done():
//			return nil, ctx.Err()
//		}
//	}
//
//	if lastEvent == nil {
//		return nil, trace.Errorf("could not find any events for session %v", sessionID)
//	}
//
//	processed.lastEvent = lastEvent
//
//	if processed.missingEndEvent {
//		processed.details.Duration = int64(lastEvent.GetTime().Sub(processed.startEvent.GetTime()) / time.Millisecond)
//	}
//
//	randomThumbnail := getRandomThumbnail(processed.details.Thumbnails)
//	if randomThumbnail != nil {
//		processed.thumbnail = randomThumbnail
//	}
//
//	processed.details.Thumbnails = getEvenlySampledThumbnails(processed.details.Thumbnails, maxThumbnails)
//
//	return &processed, nil
//}
//
//func getRandomThumbnail(thumbnails []*types.SessionRecordingThumbnail) *types.SessionRecordingThumbnail {
//	if len(thumbnails) == 0 {
//		return nil
//	}
//
//	randomIndex := rand.Intn(len(thumbnails))
//
//	return thumbnails[randomIndex]
//}
//
//func getEvenlySampledThumbnails(thumbnails []*types.SessionRecordingThumbnail, maxItems int) []*types.SessionRecordingThumbnail {
//	if len(thumbnails) <= maxItems {
//		return thumbnails
//	}
//
//	result := make([]*types.SessionRecordingThumbnail, 0, maxItems)
//	step := float64(len(thumbnails)) / float64(maxItems)
//
//	for i := 0; i < maxItems && int(float64(i)*step) < len(thumbnails); i++ {
//		result = append(result, thumbnails[int(float64(i)*step)])
//	}
//
//	return result
//}
