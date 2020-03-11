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

import { StoreParties, StoreDocs, DocumentSsh } from './stores';
import Logger from 'shared/libs/logger';
import session from 'teleport/services/session';
import history from 'teleport/services/history';
import cfg, { UrlSshParams } from 'teleport/config';
import { getAccessToken } from 'teleport/services/api';
import Tty from 'teleport/lib/term/tty';
import TtyAddressResolver from 'teleport/lib/term/ttyAddressResolver';
import serviceSsh, { Session, ParticipantList } from 'teleport/services/ssh';
import serviceNodes from 'teleport/services/nodes';
import serviceClusters from 'teleport/services/clusters';
import serviceUser from 'teleport/services/user';

const logger = Logger.create('teleport/console');

/**
 * A main controller which manages the console global state
 */
export default class ConsoleContext {
  storeDocs = new StoreDocs();
  storeParties = new StoreParties();

  constructor() {
    // always initialize the console with 1 document
    this.storeDocs.add({
      kind: 'blank',
      url: cfg.getConsoleRoute(cfg.proxyCluster),
      clusterId: cfg.proxyCluster,
    });
  }

  ensureActiveDoc(url: string) {
    const doc = this.storeDocs.state.items.find(i => i.url === url);
    if (doc) {
      this.storeDocs.state.active = doc.id;
    }

    return doc;
  }

  closeDocument(id: number) {
    const nextId = this.storeDocs.getNext(id);
    const items = this.storeDocs.filter(id);
    this.storeDocs.setState({
      items,
      active: -1,
    });

    return this.storeDocs.find(nextId);
  }

  updateSshDocument(id: number, partial: Partial<DocumentSsh>) {
    this.storeDocs.update(id, partial);
  }

  navigateTo({ url }: { url: string }, replace = false) {
    if (replace) {
      history.replace(url);
    } else {
      history.push(url);
    }
  }

  addNodeDocument(clusterId = cfg.proxyCluster) {
    return this.storeDocs.add({
      clusterId,
      title: `New session`,
      kind: 'nodes',
      url: cfg.getConsoleNodesRoute(clusterId),
    });
  }

  addSshDocument({ login, serverId, sid, clusterId }: UrlSshParams) {
    const title = login && serverId ? `${login}@${serverId}` : sid;
    const url = this.getSshDocumentUrl({
      clusterId,
      login,
      serverId,
      sid,
    });

    return this.storeDocs.add({
      kind: 'terminal',
      status: 'disconnected',
      clusterId,
      title,
      serverId,
      login,
      sid,
      url,
    });
  }

  getDocuments() {
    return this.storeDocs.state.items;
  }

  getNodeDocumentUrl(clusterId: string) {
    return cfg.getConsoleNodesRoute(clusterId);
  }

  getSshDocumentUrl(sshParams: UrlSshParams) {
    const { sid, clusterId } = sshParams;
    return sid
      ? cfg.getConsoleSessionRoute({ clusterId, sid })
      : cfg.getConsoleConnectRoute(sshParams);
  }

  refreshParties() {
    // Finds unique clusterIds from all active ssh sessions
    // and creates a separate API call per each.
    // After receiving the data, it updates the stores only once.
    const clusters = this.storeDocs
      .getSshDocuments()
      .filter(doc => doc.status === 'connected')
      .map(doc => doc.clusterId);

    const unique = [...new Set(clusters)];
    const requests = unique.map(clusterId =>
      // Fetch parties for a given cluster and in case of an error
      // return an empty object.
      serviceSsh.fetchParticipants({ clusterId }).catch(err => {
        logger.error('failed to refresh participants', err);
        const emptyResults: ParticipantList = {};
        return emptyResults;
      })
    );

    return Promise.all(requests).then(results => {
      let parties: ParticipantList = {};
      for (let i = 0; i < results.length; i++) {
        parties = {
          ...results[i],
        };
      }

      this.storeParties.setParties(parties);
    });
  }

  fetchNodes(clusterId: string) {
    return Promise.all([
      serviceUser.fetchUser(clusterId),
      serviceNodes.fetchNodes(clusterId),
    ]).then(values => {
      const [user, nodes] = values;
      return {
        logins: user.acl.logins,
        nodes,
      };
    });
  }

  fetchClusters() {
    return serviceClusters.fetchClusters();
  }

  fetchSshSession(clusterId: string, sid: string) {
    return serviceSsh.fetchSession({ clusterId, sid });
  }

  createSshSession(clusterId: string, serverId: string, login: string) {
    return serviceSsh.create({
      serverId,
      clusterId,
      login,
    });
  }

  logout() {
    session.logout();
  }

  createTty(session: Session): Tty {
    const { login, sid, serverId, clusterId } = session;
    const ttyUrl = cfg.api.ttyWsAddr
      .replace(':fqdm', getHostName())
      .replace(':token', getAccessToken())
      .replace(':clusterId', clusterId);

    const ttyConfig = {
      ttyUrl,
      ttyParams: {
        login,
        sid,
        server_id: serverId,
      },
    };

    const addressResolver = new TtyAddressResolver(ttyConfig);
    return new Tty(addressResolver);
  }
}

function getHostName() {
  return location.hostname + (location.port ? ':' + location.port : '');
}
