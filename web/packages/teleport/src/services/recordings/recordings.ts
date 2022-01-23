import cfg from 'teleport/config';
import api from 'teleport/services/api';
import { RecordingsQuery, RecordingsResponse } from './types';
import { formatters, eventCodes } from 'teleport/services/audit';

import { makeRecording } from './makeRecording';
export default class RecordingsService {
  maxFetchLimit = 5000;

  fetchRecordings(
    clusterId: string,
    params: RecordingsQuery
  ): Promise<RecordingsResponse> {
    const start = params.from.toISOString();
    const end = params.to.toISOString();

    const url = cfg.getClusterEventsUrl(clusterId, {
      start,
      end,
      limit: this.maxFetchLimit,
      startKey: params.startKey || undefined,
      include: `${formatters[eventCodes.SESSION_END].type},${
        formatters[eventCodes.DESKTOP_SESSION_ENDED].type
      }`,
    });

    return api.get(url).then(json => {
      const events = json.events || [];

      return { recordings: events.map(makeRecording), startKey: json.startKey };
    });
  }
}
