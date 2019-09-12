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
import { keyBy, values } from 'lodash';
import { Store } from 'shared/libs/stores';
import service from 'teleport/services/events';

export const EVENT_MAX_LIMIT = 9999;

export default class StoreEvents extends Store {
  state = {
    overflow: false,
    events: {},
  };

  mergeEvents(events) {
    return {
      ...this.state.events,
      ...keyBy(events, 'id'),
    };
  }

  getEvents() {
    return values(this.state.events);
  }

  getMaxLimit() {
    return EVENT_MAX_LIMIT;
  }

  fetchLatest() {
    const start = moment(new Date())
      .startOf('day')
      .toDate()
      .toISOString();
    const end = moment(new Date())
      .endOf('day')
      .toDate()
      .toISOString();
    return service.fetchEvents({ start, end }).then(events => {
      const keyed = keyEvents(events);
      this.setState({
        events: {
          ...this.state.events,
          ...keyed,
        },
      });
    });
  }

  fetchEvents({ end, start }) {
    start = start.toISOString();
    end = end.toISOString();
    return service
      .fetchEvents({ start, end, limit: EVENT_MAX_LIMIT + 1 })
      .then(events => {
        const overflow = events.length > EVENT_MAX_LIMIT;
        events = events.splice(0, EVENT_MAX_LIMIT - 1);
        const keyed = keyEvents(events);
        this.setState({
          overflow,
          events: {
            ...this.state.events,
            ...keyed,
          },
        });
      });
  }
}

function keyEvents(events) {
  return {
    ...keyBy(events, 'id'),
  };
}
