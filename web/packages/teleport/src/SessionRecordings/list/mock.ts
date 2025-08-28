/**
 * Teleport
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

import { http, HttpResponse } from 'msw';

import cfg from 'teleport/config';
import { eventCodes } from 'teleport/services/audit';
import type { SessionRecordingThumbnail } from 'teleport/services/recordings';

export function createMockSessionEndEvent(overrides?: Record<string, string>) {
  return {
    'addr.remote': '100.119.121.67:55655',
    cluster_name: 'teleport',
    code: eventCodes.SESSION_END,
    ei: 41,
    enhanced_recording: false,
    event: 'session.end',
    interactive: true,
    login: 'root',
    namespace: 'default',
    participants: ['admin'],
    private_key_policy: 'none',
    proto: 'ssh',
    server_hostname: 'server-1',
    server_id: 'dc247eee-742d-445b-bb30-04bfc4604061',
    server_labels: {
      hostname: 'instance-1',
    },
    server_version: '19.0.0-dev',
    session_recording: 'node',
    session_start: '2025-08-13T12:42:23.099789612Z',
    session_stop: '2025-08-13T12:42:41.151765713Z',
    sid: 'ed794e43-57a3-461c-a4a6-1a375a5bbbba',
    time: '2025-08-13T12:42:41.152Z',
    uid: '4a0fdcfb-fa14-4446-9c06-52c18797fdbe',
    user: 'admin',
    user_kind: 1,
    ...overrides,
  };
}

export const MOCK_EVENTS = [
  createMockSessionEndEvent({
    sid: 'session-001',
    user: 'alice',
    server_hostname: 'server-01',
    proto: 'ssh',
  }),
  createMockSessionEndEvent({
    sid: 'session-002',
    user: 'bob',
    desktop_name: 'desktop-02',
    code: eventCodes.DESKTOP_SESSION_ENDED,
    proto: 'desktop',
    time: '2025-01-15T11:00:00Z',
  }),
  createMockSessionEndEvent({
    sid: 'session-003',
    user: 'charlie',
    db_service: 'database-01',
    code: eventCodes.DATABASE_SESSION_ENDED,
    proto: 'database',
    time: '2025-01-15T12:00:00Z',
  }),
  createMockSessionEndEvent({
    sid: 'session-004',
    user: 'alice',
    kubernetes_cluster: 'k8s-cluster',
    proto: 'kube',
    time: '2025-01-15T13:00:00Z',
  }),
  createMockSessionEndEvent({
    sid: 'session-005',
    user: 'david',
    server_hostname: 'server-03',
    proto: 'ssh',
    time: '2025-01-15T14:00:00Z',
  }),
];

export const MOCK_THUMBNAIL: SessionRecordingThumbnail = {
  svg: '<svg><rect width="100" height="100" fill="blue"/></svg>',
  cursorX: 50,
  cursorY: 50,
  cursorVisible: true,
  cols: 100,
  rows: 100,
  startOffset: 0,
  endOffset: 0,
};

export function getThumbnail(thumbnail: SessionRecordingThumbnail) {
  return http.get(cfg.api.sessionRecording.thumbnail, () => {
    return HttpResponse.json(thumbnail);
  });
}

export function thumbnailError() {
  return http.get(cfg.api.sessionRecording.thumbnail, () => {
    return HttpResponse.json(
      { error: { message: 'Thumbnail not found' } },
      { status: 404 }
    );
  });
}
