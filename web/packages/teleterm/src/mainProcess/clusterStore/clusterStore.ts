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

  constructor(private readonly tshdClient: TshdClient) {}

  /** Adds a cluster. */
  async add(proxyAddress: string): Promise<Cluster> {
    const { response } = await this.tshdClient.addCluster({
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

  /** Logs out of the cluster and removes its profile.*/
  async logout(uri: RootClusterUri): Promise<void> {
    // TODO(gzdunek): logout and removeCluster should be combined into
    //  a single acton in tshd.
    await this.tshdClient.logout({ clusterUri: uri });
    await this.tshdClient.removeCluster({ clusterUri: uri });
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
    const { response } = await this.tshdClient.listRootClusters({});
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
    try {
      const { cluster, leafs } = await getClusterAndLeafs(this.tshdClient, uri);

      await this.update(draft => {
        draft.set(cluster.uri, cluster);
        leafs.forEach(leaf => {
          draft.set(leaf.uri, leaf);
        });
      });
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
   * Registers a sender.
   * Sends the full current state as the initial message, and then patches.
   * This method blocks until the sender is disposed.
   */
  async useSender(sender: AwaitableSender<ClusterStoreUpdate>): Promise<void> {
    this.logger.info('Sender added');
    this.senders.add(sender);
    await sender.send({ kind: 'state', value: this.state });
    await sender.whenDisposed();
    this.senders.delete(sender);
    this.logger.info('Sender removed');
  }

  private async update(producer: Producer<State>): Promise<void> {
    const [state, patches] = produceWithPatches(this.state, producer);
    this.state = state;
    await Promise.all(
      this.senders
        .values()
        .map(sender => sender.send({ kind: 'patches', value: patches }))
    );
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
    leafs: new Map(resolved[1].response.clusters.map(c => [c.uri, c])),
  };
}
