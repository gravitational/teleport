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
