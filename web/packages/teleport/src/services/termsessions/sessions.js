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

import { map } from 'lodash';
import api from 'teleport/services/api';
import cfg from 'teleport/config';
import makeSession, { makeParticipant } from './makeSession';

const service = {
  create({ serverId, clusterId, login }) {
    const request = {
      session: {
        login,
      },
    };

    return api
      .post(cfg.getTerminalSessionUrl(clusterId), request)
      .then(response => {
        const session = makeSession(response.session);
        return {
          ...session,
          hostname: serverId,
          serverId,
          clusterId,
        };
      });
  },

  fetchSession({ clusterId, sid }) {
    return Promise.all([
      api.get(cfg.getTerminalSessionUrl({ sid })),
      api.get(cfg.getClusterNodesUrl(clusterId)),
    ]).then(response => {
      const [sessionJson, serversJson] = response;
      const server = serversJson.items.find(
        s => s.id === sessionJson.server_id
      );
      const session = makeSession(sessionJson);
      session.hostname = server.hostname;
      session.clusterId = clusterId;
      return session;
    });
  },

  fetchSessions() {
    return api.get(cfg.getTerminalSessionUrl()).then(response => {
      if (response && response.sessions) {
        return map(response.sessions, makeSession);
      }

      return [];
    });
  },

  fetchParticipants({ clusterId }) {
    // Because given session might not be available right away,
    // we query for all active session to find this session participants.
    // This is to avoid 404 errors.
    return api.get(cfg.getTerminalSessionUrl(clusterId)).then(json => {
      if (!json && !json.sessions) {
        return [];
      }

      const parties = {};
      json.sessions.forEach(s => {
        parties[s.id] = map(s.parties, makeParticipant);
      });

      return parties;
    });
  },
};

export const SessionStateEnum = {
  CONNECTED: 'connected',
  DISCONNECTED: 'disconnected',
  LOADING: 'loading',
  JOINING: 'joining',
  NOT_FOUND: 'notfound',
  ERROR: 'error',
};

export default service;
