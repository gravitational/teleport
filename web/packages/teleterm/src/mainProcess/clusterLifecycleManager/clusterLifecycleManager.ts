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

import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import Logger from 'teleterm/logger';
import { AwaitableSender } from 'teleterm/mainProcess/awaitableSender';
import { RendererIpc } from 'teleterm/mainProcess/types';
import type { WindowsManager } from 'teleterm/mainProcess/windowsManager';
import { AppUpdater } from 'teleterm/services/appUpdater';
import { isTshdRpcError, TshdClient } from 'teleterm/services/tshd';
import { mergeClusterProfileWithDetails } from 'teleterm/services/tshd/cluster';
import { RootClusterUri } from 'teleterm/ui/uri';

import { ClusterStore } from '../clusterStore';
import { ProfileChangeSet } from '../profileWatcher';

/** Describes a lifecycle event related to a cluster. */
export interface ClusterLifecycleEvent {
  uri: RootClusterUri;
  /**
   * The lifecycle operation type.
   *
   * Operations prefixed with `will-` occur before the corresponding action
   * in the main process and can interrupt it when they return an error.
   *
   * Operations prefixed with `did-` occur after the action has already happened
   * in the main process, so they cannot prevent it.
   */
  op: 'did-add-cluster' | 'will-logout' | 'will-logout-and-remove';
}

export interface ProfileWatcherError {
  error: unknown;
  reason: 'processing-error' | 'exited';
}

/**
 * Manages the lifecycle of clusters by handling both UI actions that update them
 * (e.g., adding a cluster, logging out) and profile watcher events.
 *
 * When handling an action or event requires additional work on the renderer side
 * (for example, cleaning up before logging out), a handler registered
 * in `rendererEventHandler` is invoked.
 * Then, the cluster store is updated (the exact order of these steps depends
 * on the specific case).
 *
 * It is important to always call a method of `ClusterLifecycleManager` rather
 * than interacting directly with `ClusterStore` whenever the action involves
 * additional work on the renderer side.
 */
export class ClusterLifecycleManager {
  private readonly logger = new Logger('ClusterLifecycleManager');
  private rendererEventHandler:
    | AwaitableSender<ClusterLifecycleEvent>
    | undefined;
  private watcherStarted = false;

  constructor(
    private readonly clusterStore: ClusterStore,
    private readonly getTshdClient: () => Promise<TshdClient>,
    private readonly appUpdater: AppUpdater,
    private readonly windowsManager: Pick<WindowsManager, 'getWindow'>,
    private readonly profileWatcher: AsyncIterable<ProfileChangeSet>
  ) {}

  setRendererEventHandler(
    handler: AwaitableSender<ClusterLifecycleEvent>
  ): void {
    if (this.rendererEventHandler) {
      this.logger.error(
        'Only one renderer lifecycle event handler can be registered at a time'
      );
      return;
    }

    this.logger.info('Renderer lifecycle event handler registered');
    this.rendererEventHandler = handler;
    this.rendererEventHandler.whenDisposed().then(() => {
      this.logger.info('Renderer lifecycle event handler unregistered');
      this.rendererEventHandler = undefined;
    });
  }

  async addCluster(proxyAddress: string): Promise<Cluster> {
    const cluster = await this.clusterStore.add(proxyAddress);
    await this.rendererEventHandler.send({
      op: 'did-add-cluster',
      uri: cluster.uri,
    });
    return cluster;
  }

  async logoutAndRemoveCluster(uri: RootClusterUri): Promise<void> {
    await this.rendererEventHandler.send({ op: 'will-logout-and-remove', uri });
    this.onBeforeRemove(uri);
    await this.clusterStore.logoutAndRemove(uri);
  }

  async syncRootClustersAndStartProfileWatcher(): Promise<void> {
    await this.clusterStore.syncRootClusters();
    if (!this.watcherStarted) {
      this.watcherStarted = true;
      void this.watchProfileChanges();
    }
  }

  private onBeforeRemove(uri: RootClusterUri): void {
    // Do not wait for this promise to finish as we don't want to block logout
    // on checking app updates.
    this.appUpdater.maybeRemoveManagingCluster(uri).catch(error => {
      this.logger.error('Failed to remove managing cluster', error);
    });
  }

  /**
   * If the cluster is connected, try to sync it to get the full profile with details.
   * Otherwise, update the cluster with the profile read from disk.
   */
  private async syncOrUpdateCluster(cluster: Cluster): Promise<void> {
    if (cluster.connected) {
      try {
        return this.clusterStore.sync(cluster.uri);
      } catch (e) {
        // Theoretically, the cert could just expire and result in an error
        // resolvable with relogin when trying to sync the cluster.
        // In that case, only update the store.
        if (!(isTshdRpcError(e) && e.isResolvableWithRelogin)) {
          throw e;
        }
      }
    }
    const existing = this.clusterStore.getState().get(cluster.uri);
    await this.clusterStore.set(
      mergeClusterProfileWithDetails({
        profile: cluster,
        details: existing || Cluster.create(),
      })
    );
  }

  /**
   * Watches for changes in the `tsh` directory.
   *
   * Some file system events require notifying the renderer (e.g., to
   * remove a workspace before a cluster store update is sent).
   */
  private async watchProfileChanges(): Promise<void> {
    try {
      for await (const changes of this.profileWatcher) {
        this.logger.info('Detected profile changes', changes);

        for (const change of changes) {
          try {
            switch (change.op) {
              case 'added':
                await this.handleClusterAdded(change.cluster);
                break;
              case 'changed':
                await this.handleClusterChanged(change.previous, change.next);
                break;
              case 'removed':
                await this.handleClusterRemoved(change.cluster);
                break;
            }
          } catch (error) {
            this.logger.error('Error while processing cluster event', error);
            this.handleWatcherError({ error, reason: 'processing-error' });
          }
        }
      }
    } catch (error) {
      this.logger.error('Profile watcher exited with error', error);
      this.handleWatcherError({ error, reason: 'exited' });
    }
  }

  private async handleClusterAdded(cluster: Cluster): Promise<void> {
    await this.syncOrUpdateCluster(cluster);
    await this.rendererEventHandler.send({
      op: 'did-add-cluster',
      uri: cluster.uri,
    });
  }

  private async handleClusterChanged(
    previous: Cluster,
    next: Cluster
  ): Promise<void> {
    const wasLoggedIn = previous.loggedInUser?.name;
    const isLoggedIn = next.loggedInUser?.name;
    const hasLoggedOut = wasLoggedIn && !isLoggedIn;

    if (hasLoggedOut) {
      await this.handleClusterLogout(next);
    } else {
      await this.syncOrUpdateCluster(next);
    }
  }

  private async handleClusterRemoved(cluster: Cluster): Promise<void> {
    await this.rendererEventHandler.send({
      op: 'will-logout-and-remove',
      uri: cluster.uri,
    });
    this.onBeforeRemove(cluster.uri);
    await this.clusterStore.logoutAndRemove(cluster.uri);
  }

  private async handleClusterLogout(cluster: Cluster): Promise<void> {
    await this.rendererEventHandler.send({
      op: 'will-logout',
      uri: cluster.uri,
    });
    const client = await this.getTshdClient();
    await client.logout({ clusterUri: cluster.uri, removeProfile: false });
    await this.syncOrUpdateCluster(cluster);
  }

  private handleWatcherError(error: ProfileWatcherError): void {
    this.windowsManager
      .getWindow()
      .webContents.send(RendererIpc.ProfileWatcherError, error);
  }
}
