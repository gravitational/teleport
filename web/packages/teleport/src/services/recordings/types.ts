/**
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

export enum RiskLevel {
  Low = 'RISK_LEVEL_LOW',
  Medium = 'RISK_LEVEL_MEDIUM',
  High = 'RISK_LEVEL_HIGH',
  Critical = 'RISK_LEVEL_CRITICAL',
  None = 'RISK_LEVEL_NONE',
  Unspecified = 'RISK_LEVEL_UNSPECIFIED',
}

export type RecordingsQuery = {
  from: Date;
  to: Date;
  limit?: number;
  startKey?: string;
  /**
   * Restricts the response to these session recording event types, e.g.
   * `[BEAM_SESSION_END_EVENT]`. Omit to return every recording type.
   */
  include?: string[];
};

/**
 * BEAM_SESSION_END_EVENT is the audit event type emitted when a beam is torn
 * down, and the only recording type a beam produces. It is the event *type*
 * rather than its code (`eventCodes.BEAM_SESSION_END`), because that is what
 * the recordings endpoint's `include` filter matches on.
 *
 * Keep in sync with `BeamSessionEndEvent` in `lib/events/api.go`.
 */
export const BEAM_SESSION_END_EVENT = 'beam.session.end';

export type RecordingsResponse = {
  recordings: Recording[];
  startKey: string;
};

export type RecordingType =
  | 'ssh'
  | 'desktop'
  | 'k8s'
  | 'database'
  | 'app'
  | 'beam';

export function validateRecordingType(
  value: RecordingType | string
): value is RecordingType {
  return (
    value === 'ssh' ||
    value === 'database' ||
    value === 'desktop' ||
    value === 'k8s' ||
    value === 'app' ||
    value === 'beam'
  );
}

export type Recording = {
  duration: number;
  durationText: string;
  sid: string;
  createdDate: Date;
  users: string;
  hostname: string;
  description: string;
  recordingType: RecordingType;
  playable: boolean;
  user: string;
};

export enum SessionRecordingMessageType {
  Thumbnail = 'thumbnail',
  Metadata = 'metadata',
  Error = 'error',
}

export enum SessionRecordingEventType {
  Inactivity = 'inactivity',
  Join = 'join',
  Resize = 'resize',
  Risk = 'risk',
}

export interface SessionRecordingThumbnail {
  svg: string;
  cols: number;
  rows: number;
  cursorX: number;
  cursorY: number;
  cursorVisible: boolean;
  startOffset: number;
  endOffset: number;
  png?: string;
  screenWidth?: number;
  screenHeight?: number;
}

export interface SessionRecordingMetadata {
  duration: number;
  events: SessionRecordingEvent[];
  startCols: number;
  startRows: number;
  startTime: number;
  endTime: number;
  clusterName: string;
  user: string;
  resourceName: string;
  type: RecordingType;
}

export interface SessionRecordingError {
  message: string;
}

// This is a wrapper type to match the structure of messages sent over the WebSocket.
type WrappedMessage<TType extends SessionRecordingMessageType, TData> = {
  type: TType;
  data: TData;
};

export type SessionRecordingMessage =
  | WrappedMessage<
      SessionRecordingMessageType.Thumbnail,
      SessionRecordingThumbnail
    >
  | WrappedMessage<
      SessionRecordingMessageType.Metadata,
      SessionRecordingMetadata
    >
  | WrappedMessage<SessionRecordingMessageType.Error, SessionRecordingError>;

interface BaseSessionRecordingEvent {
  startTime: number;
  endTime: number;
}

interface SessionRecordingInactivityEvent extends BaseSessionRecordingEvent {
  type: SessionRecordingEventType.Inactivity;
}

interface SessionRecordingJoinEvent extends BaseSessionRecordingEvent {
  type: SessionRecordingEventType.Join;
  user: string;
}

export interface SessionRecordingRiskEvent extends BaseSessionRecordingEvent {
  type: SessionRecordingEventType.Risk;
  riskLevel: RiskLevel;
  description: string;
  isError?: boolean;
}

export interface SessionRecordingResizeEvent extends BaseSessionRecordingEvent {
  type: SessionRecordingEventType.Resize;
  cols: number;
  rows: number;
}

export type SessionRecordingEvent =
  | SessionRecordingJoinEvent
  | SessionRecordingResizeEvent
  | SessionRecordingInactivityEvent
  | SessionRecordingRiskEvent;
