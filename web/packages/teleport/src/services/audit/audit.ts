/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import moment from 'moment';
import api from 'teleport/services/api';
import cfg from 'teleport/config';
import makeEvent from './makeEvent';

export const EVENT_MAX_LIMIT = 9999;

const service = {
  maxLimit: EVENT_MAX_LIMIT,

  fetchLatest() {
    const start = moment(new Date())
      .startOf('day')
      .toDate();
    const end = moment(new Date())
      .endOf('day')
      .toDate();

    return service.fetchEvents({ start, end });
  },

  fetchEvents(params: { end: Date; start: Date }) {
    const start = params.start.toISOString();
    const end = params.end.toISOString();
    const url = cfg.getClusterEventsUrl({
      start,
      end,
      limit: EVENT_MAX_LIMIT + 1,
    });
    return api
      .get(url)
      .then(json => {
        const events = json.events as any[];
        return events.map(makeEvent);
      })
      .then(events => {
        const overflow = events.length > EVENT_MAX_LIMIT;
        events = events.splice(0, EVENT_MAX_LIMIT - 1);
        return {
          overflow,
          events,
        };
      });
  },
};

export default service;
