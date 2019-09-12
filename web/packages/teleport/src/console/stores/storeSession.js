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

import { Store } from 'shared/libs/stores';
import cfg from 'teleport/config';
import api, { getAccessToken } from 'teleport/services/api';

const defaultStatus = {
  isReady: false,
  isLoading: false,
  isError: false,
  errorText: undefined,
};

export default class StoreSession extends Store {
  state = {
    status: {
      ...defaultStatus,
    },
    isNew: false,
    hostname: null,
    login: null,
    clusterId: null,
    serverId: null,
    sid: null,
    parties: [],
  };

  getClusterName() {
    return this.state.clusterId;
  }

  getTtyConfig() {
    const { login, sid, serverId, clusterId } = this.state;
    const ttyUrl = cfg.api.ttyWsAddr
      .replace(':fqdm', getHostName())
      .replace(':token', getAccessToken())
      .replace(':clusterId', clusterId);
    return {
      ttyUrl,
      ttyParams: {
        login,
        sid,
        server_id: serverId,
      },
    };
  }

  setStatus(json) {
    this.setState({
      status: {
        ...defaultStatus,
        ...json,
      },
    });
  }

  getServerLabel() {
    const { hostname, serverId, login } = this.state;
    if (hostname) {
      return `${login}@${hostname}`;
    }

    if (serverId) {
      return `${login}@${serverId}`;
    }

    return 'Connecting...';
  }

  createSession({ serverId, clusterId, hostname, login }) {
    const request = {
      session: {
        login,
      },
    };

    return api
      .post(cfg.getTerminalSessionUrl(clusterId), request)
      .then(json => {
        const sid = json.session.id;
        this.setState({
          isNew: true,
          serverId,
          clusterId,
          login,
          hostname,
          sid,
          status: {
            ...defaultStatus,
            isReady: true,
          },
        });

        return sid;
      });
  }

  fetchParticipants() {
    const { clusterId, sid } = this.state;
    // because given session might not be available right away,
    // fetch all session to avoid 404 errors.
    return api.get(cfg.getTerminalSessionUrl(clusterId)).then(json => {
      if (!json && !json.sessions) {
        return;
      }

      const session = json.sessions.find(s => s.id === sid);
      if (!session) {
        return;
      }

      this.setState({
        parties: session.parties,
      });
    });
  }

  joinSession(clusterId, sid) {
    this.setState({
      status: {
        ...defaultStatus,
        isLoading: true,
      },
    });

    Promise.all([
      api.get(cfg.getTerminalSessionUrl({ clusterId, sid })),
      api.get(cfg.getClusterNodesUrl(clusterId)),
    ])
      .then(response => {
        const [session, servers] = response;
        const server = servers.items.find(s => s.id === session.server_id);
        this.setState({
          sid,
          clusterId,
          login: session.login,
          serverId: session.server_id,
          hostname: server.hostname,
          status: {
            ...defaultStatus,
            isReady: true,
          },
        });
      })
      .catch(err => {
        this.setState({
          status: {
            ...defaultStatus,
            isError: true,
            errorText: err.message,
          },
        });
      });
  }
}

function getHostName() {
  return location.hostname + (location.port ? ':' + location.port : '');
}
