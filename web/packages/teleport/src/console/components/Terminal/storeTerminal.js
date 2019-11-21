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
import { getAccessToken } from 'teleport/services/api';
import service, { SessionStateEnum } from 'teleport/services/termsessions';

export default class StoreTermimal extends Store {
  state = {
    session: null,
    status: SessionStateEnum.LOADING,
    statusText: '',
  };

  init({ serverId, clusterId, hostname, login, sid }) {
    if (!sid) {
      return this.create({
        serverId,
        clusterId,
        hostname,
        login,
      });
    }

    return this.join({ clusterId, sid });
  }

  create({ serverId, clusterId, login }) {
    return service
      .create({
        serverId,
        clusterId,
        login,
      })
      .then(session => {
        this.setState({
          session,
          status: SessionStateEnum.CONNECTED,
        });

        return session;
      })
      .catch(err => {
        this.setState({
          status: SessionStateEnum.ERROR,
          statusText: err.message,
        });

        throw err;
      });
  }

  join({ clusterId, sid }) {
    return service
      .fetchSession({ clusterId, sid })
      .then(session => {
        this.setState({
          session,
          status: SessionStateEnum.CONNECTED,
        });

        return session;
      })
      .catch(err => {
        this.setState({
          status: SessionStateEnum.NOT_FOUND,
          statusText: err.message,
        });

        throw err;
      });
  }

  isLoading() {
    return this.state.status === SessionStateEnum.LOADING;
  }

  isNotFound() {
    return this.state.status === SessionStateEnum.NOT_FOUND;
  }

  isError() {
    return this.state.status === SessionStateEnum.ERROR;
  }

  isConnected() {
    return this.state.status === SessionStateEnum.CONNECTED;
  }

  getSessionUrl() {
    return cfg.getConsoleSessionRoute({ sid: this.state.session.sid });
  }

  getTtyConfig() {
    const { login, sid, serverId, clusterId } = this.state.session;
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

  setStatus({ status, statusText }) {
    this.setState({
      status,
      statusText,
    });
  }

  getServerLabel() {
    const { hostname, serverId, login } = this.state.session;
    if (hostname) {
      return `${login}@${hostname}`;
    }

    if (serverId) {
      return `${login}@${serverId}`;
    }

    return 'Connecting...';
  }
}

function getHostName() {
  return location.hostname + (location.port ? ':' + location.port : '');
}
