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

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import { makeRecording } from './makeRecording';
import { RecordingsQuery, RecordingsResponse } from './types';

export default class RecordingsService {
  maxFetchLimit = 5000;

  fetchRecordings(
    clusterId: string,
    params: RecordingsQuery
  ): Promise<RecordingsResponse> {
    const start = params.from.toISOString();
    const end = params.to.toISOString();

    const url = cfg.getClusterEventsRecordingsUrl(clusterId, {
      start,
      end,
      limit: this.maxFetchLimit,
      startKey: params.startKey || undefined,
    });

    return api.get(url).then(json => {
      const events = json.events || [];

      return { recordings: events.map(makeRecording), startKey: json.startKey };
    });
  }

  fetchRecordingDuration(
    clusterId: string,
    sessionId: string
  ): Promise<{ durationMs: number }> {
    return api.get(cfg.getSessionDurationUrl(clusterId, sessionId));
  }
}
