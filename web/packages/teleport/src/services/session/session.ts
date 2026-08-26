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

import makeSession, { makeParticipant } from './makeSession';
import { ParticipantList } from './types';

const service = {
  fetchSessions(clusterId) {
    return api
      .get(cfg.getActiveAndPendingSessionsUrl({ clusterId }))
      .then(response => {
        if (response && response.sessions) {
          return response.sessions.map(makeSession);
        }

        return [];
      });
  },

  fetchParticipants({ clusterId }: { clusterId: string }) {
    // Because given session might not be available right away,
    // we query for all active session to find this session participants.
    // This is to avoid 404 errors.
    return api
      .get(cfg.getActiveAndPendingSessionsUrl({ clusterId }))
      .then(json => {
        if (!json && !json.sessions) {
          return {};
        }

        const parties: ParticipantList = {};
        json.sessions.forEach(s => {
          parties[s.id] = s.parties.map(makeParticipant);
        });

        return parties;
      });
  },
};

export default service;
