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

import makeEvent from './makeEvent';
import { EventQuery, EventResponse } from './types';

class AuditService {
  maxFetchLimit = 5000;

  fetchEvents(clusterId: string, params: EventQuery): Promise<EventResponse> {
    const start = params.from.toISOString();
    const end = params.to.toISOString();

    const url = cfg.getClusterEventsUrl(clusterId, {
      start,
      end,
      limit: this.maxFetchLimit,
      include: params.filterBy || undefined,
      startKey: params.startKey || undefined,
    });

    return api.get(url).then(json => {
      const events = json.events || [];

      return { events: events.map(makeEvent), startKey: json.startKey };
    });
  }
}

export default AuditService;
