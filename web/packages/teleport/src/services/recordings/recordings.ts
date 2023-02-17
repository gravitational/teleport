/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import { RecordingsQuery, RecordingsResponse } from './types';
import { makeRecording } from './makeRecording';

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
}
