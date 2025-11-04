/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Patch, Producer, produceWithPatches } from 'immer';

import {
  Cluster,
  ShowResources,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import Logger from 'teleterm/logger';
import { TshdClient } from 'teleterm/services/tshd';
import { ClusterUri, RootClusterUri, routing } from 'teleterm/ui/uri';

import { AwaitableSender } from '../awaitableSender';
import type { WindowsManager } from '../windowsManager';

export type State = ReadonlyMap<ClusterUri, Cluster>;

export type ClusterStoreUpdate =
  /**
   * Patches allow the other side to keep reference stability
   * so that not the whole state is recreated.
   */
  | { kind: 'patches'; value: Patch[] }
  /** The full state, useful to get the initial state on the other side. */
  | { kind: 'state'; value: State };

export class ClusterStore {
  private senders = new Set<AwaitableSender<ClusterStoreUpdate>>();
  private state: State = new Map();
  private logger = new Logger('ClusterStore');

  constructor(
    private readonly getTshdClient: () => Promise<TshdClient>,
    private readonly windowsManager: Pick<WindowsManager, 'crashWindow'>
  ) {}

  /**
   * Adds a cluster.
   * Should only be called via ClusterLifecycleManager.
   */
  async add(proxyAddress: string): Promise<Cluster> {
    const client = await this.getTshdClient();
    const { response } = await client.addCluster({
      name: proxyAddress,
    });

    await this.update(draft => {
      // Do not overwrite the existing cluster;
      // otherwise we may lose properties fetched from the auth server.
      // Consider separating properties read from profile and those
      // fetched from the auth server at the RPC message level.
      if (draft.has(response.uri)) {
        return;
      }
      draft.set(response.uri, response);
    });

    return response;
  }

  /**
   * Logs out of the cluster and removes its profile.
   * Should only be called via ClusterLifecycleManager.
   */
  async logoutAndRemove(uri: RootClusterUri): Promise<void> {
    const client = await this.getTshdClient();
    await client.logout({ clusterUri: uri, removeProfile: true });
    await this.update(draft => {
      for (let d of draft.values()) {
        if (routing.belongsToProfile(uri, d.uri)) {
          draft.delete(d.uri);
        }
      }
    });
  }

  /**
   * Synchronizes the root clusters.
   * Does not make a network call, only reads profiles from the disk.
   */
  async syncRootClusters(): Promise<void> {
    const client = await this.getTshdClient();
    const { response } = await client.listRootClusters({});
    await this.update(draft => {
      draft.clear();
      response.clusters.forEach(cluster => {
        draft.set(cluster.uri, cluster);
      });
    });
  }

  /**
   * Synchronizes a root cluster.
   * Makes network calls to get cluster details and its leaf clusters.
   */
  async sync(uri: RootClusterUri): Promise<void> {
    let cluster: Cluster;
    let leafs: Cluster[];
    const client = await this.getTshdClient();
    try {
      const clusterAndLeafs = await getClusterAndLeafs(client, uri);
      cluster = clusterAndLeafs.cluster;
      leafs = clusterAndLeafs.leafs;
    } catch (error) {
      await this.update(draft => {
        const cluster = draft.get(uri);
        if (cluster) {
          // TODO(gzdunek): We should rather store the cluster synchronization status,
          // so the callsites could check it before reading the field.
          // The workaround is to update the field in case of a failure,
          // so the places that wait for showResources !== UNSPECIFIED don't get stuck indefinitely.
          cluster.showResources = ShowResources.ACCESSIBLE_ONLY;
        }
      });
      throw error;
    }

    await this.update(draft => {
      draft.set(cluster.uri, cluster);
      leafs.forEach(leaf => {
        draft.set(leaf.uri, leaf);
      });
    });
  }

  async set(cluster: Cluster): Promise<void> {
    await this.update(draft => {
      draft.set(cluster.uri, cluster);
    });
  }

  getRootClusters(): Cluster[] {
    return this.state
      .values()
      .toArray()
      .filter(c => !c.leaf);
  }

  getState(): State {
    return this.state;
  }

  /**
   * Registers an `AwaitableSender` to send updates and automatically unregisters
   * it when disposed.
   *
   * Upon registration, the current state is sent immediately as the initial
   * message to the sender.
   */
  registerSender(sender: AwaitableSender<ClusterStoreUpdate>): void {
    this.logger.info('Sender registered');
    this.senders.add(sender);
    const send = this.withErrorHandling(update => sender.send(update));
    void send({
      kind: 'state',
      value: this.state,
    });
    sender.whenDisposed().then(() => {
      this.senders.delete(sender);
      this.logger.info('Sender unregistered');
    });
  }

  private async update(producer: Producer<State>): Promise<void> {
    const [state, patches] = produceWithPatches(this.state, producer);
    this.state = state;
    await Promise.all(
      this.senders.values().map(sender => {
        const send = this.withErrorHandling(update => sender.send(update));
        return send({
          kind: 'patches',
          value: patches,
        });
      })
    );
  }

  /**
   * Wraps a cluster store update sender function with error handling.
   *
   * Any error indicates that the renderer state may be out of sync with the cluster store.
   * Applying further updates may fail.
   * Prompt the user to reload the window or quit the app.
   */
  private withErrorHandling(
    sender: (update: ClusterStoreUpdate) => Promise<void>
  ): (update: ClusterStoreUpdate) => Promise<void> {
    return async update => {
      try {
        await sender(update);
      } catch (e) {
        await this.windowsManager.crashWindow(e);
      }
    };
  }
}

async function getClusterAndLeafs(tshdClient: TshdClient, uri: RootClusterUri) {
  const resolved = await Promise.all([
    tshdClient.getCluster({
      clusterUri: uri,
    }),
    tshdClient.listLeafClusters({
      clusterUri: uri,
    }),
  ]);

  return {
    cluster: resolved[0].response,
    leafs: resolved[1].response.clusters,
  };
}
