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

import api from 'teleport/services/api';
import cfg from 'teleport/config';

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
