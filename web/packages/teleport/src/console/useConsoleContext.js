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

import React from 'react';
import { StoreParties, StoreDocs, StoreDialogs } from './stores';
import { useStore } from 'shared/libs/stores';
import cfg from 'teleport/config';
import service, { SessionStateEnum } from 'teleport/services/termsessions';

/**
 * Console is the main controller which manages the global state
 */
export class Console {
  storeDocs = new StoreDocs();
  storeDialogs = new StoreDialogs();
  storeParties = new StoreParties();

  init({ clusterId }) {
    cfg.setClusterId(clusterId);
    this.storeDocs = new StoreDocs();
    this.storeDialogs = new StoreDialogs();
    this.storeParties = new StoreParties();
    this.storeDocs.add({
      title: `Cluster - ${clusterId}`,
      url: cfg.getConsoleRoute(clusterId),
    });

    return Promise.resolve();
  }

  closeTab(id) {
    const nextId = this.storeDocs.getNext(id);
    const items = this.storeDocs.filter(id);
    this.storeDocs.setState({
      items,
      active: -1,
    });

    return this.storeDocs.find(nextId);
  }

  makeActiveByUrl(url) {
    const doc = this.storeDocs.state.items.find(i => i.url === url);
    if (doc) {
      this.storeDocs.state.active = doc.id;
    }

    return doc;
  }

  addTerminalTab({ login, serverId, sid }) {
    const title = login && serverId ? `${login}@${serverId}` : sid;
    const url = sid
      ? cfg.getConsoleSessionRoute({ sid })
      : cfg.getConsoleConnectRoute({
          serverId,
          login,
        });

    return this.storeDocs.add({
      title,
      type: 'terminal',
      clusterId: cfg.clusterName,
      serverId,
      login,
      sid,
      url,
    });
  }

  updateConnectedTerminalTab(id, { serverId, login, sid, hostname }) {
    this.storeDocs.updateItem(id, {
      title: `${login}@${hostname}`,
      url: cfg.getConsoleSessionRoute({ sid }),
      status: SessionStateEnum.CONNECTED,
      sid,
      serverId,
      login,
    });

    return this.storeDocs.find(id);
  }

  refreshParties(clusterId) {
    return service.fetchParticipants({ clusterId }).then(parties => {
      this.storeParties.setParties(parties);
    });
  }
}

export const ConsoleContext = React.createContext(new Console());

export default function useConsoleContext() {
  const value = React.useContext(ConsoleContext);
  window.teleconsole = value;
  return value;
}

export function useStoreDocs() {
  const console = React.useContext(ConsoleContext);
  return useStore(console.storeDocs);
}

export function useStoreDialogs() {
  const teleconsole = React.useContext(ConsoleContext);
  return useStore(teleconsole.storeDialogs);
}

export function useParties() {
  const teleconsole = React.useContext(ConsoleContext);
  return useStore(teleconsole.storeParties);
}

export { SessionStateEnum };
