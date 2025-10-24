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
import {
  MOCK_EVENTS,
  MOCK_THUMBNAILS,
} from 'teleport/SessionRecordings/list/mock';

export function withMockThumbnails() {
  return http.get(cfg.api.sessionRecording.thumbnail, ({ params }) => {
    const index = MOCK_EVENTS.findIndex(e => e.sid === params.sessionId);

    if (index === -1) {
      return HttpResponse.json({ message: 'not found' }, { status: 404 });
    }

    return HttpResponse.json(MOCK_THUMBNAILS[index]);
  });
}

export function withMockCluster() {
  return http.get(cfg.api.clustersPath, () =>
    HttpResponse.json([
      {
        name: 'teleport',
        lastConnected: '2025-08-14T14:36:07.976470934Z',
        status: 'online',
        publicURL: '',
        authVersion: '',
        proxyVersion: '',
      },
    ])
  );
}

export function withMockEvents() {
  return http.get(cfg.api.clusterEventsRecordingsPath, () => {
    return HttpResponse.json({
      events: MOCK_EVENTS,
    });
  });
}
