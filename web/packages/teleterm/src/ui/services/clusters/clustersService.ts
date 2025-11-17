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

import { applyPatches, castDraft, enablePatches } from 'immer';

import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import {
  CreateAccessRequestRequest,
  CreateGatewayRequest,
  PromoteAccessRequestRequest,
  ReviewAccessRequestRequest,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import { useStore } from 'shared/libs/stores';
import { AbortError, isAbortError } from 'shared/utils/error';

import type { State as ClustersState } from 'teleterm/mainProcess/clusterStore';
import { MainProcessClient } from 'teleterm/mainProcess/types';
import { TshdClient } from 'teleterm/services/tshd';
import { getGatewayTargetUriKind } from 'teleterm/services/tshd/gateway';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { UsageService } from 'teleterm/ui/services/usage';
import * as uri from 'teleterm/ui/uri';

import { ImmutableStore } from '../immutableStore';

const { routing } = uri;

type ClustersServiceState = {
  /**
   * `clusters` is a local mirror of the `ClusterStore` state from the main process.
   * This state is read-only and must not be updated manually — it is managed exclusively
   * through `subscribeToClusterStore`.
   */
  clusters: ClustersState;
  gateways: Map<uri.GatewayUri, Gateway>;
};

enablePatches();

export function createClusterServiceState(): ClustersServiceState {
  return {
    clusters: new Map(),
    gateways: new Map(),
  };
}

export class ClustersService extends ImmutableStore<ClustersServiceState> {
  state: ClustersServiceState = createClusterServiceState();

  constructor(
    public client: TshdClient,
    private mainProcessClient: MainProcessClient,
    private notificationsService: NotificationsService,
    private usageService: UsageService
  ) {
    super();
    this.subscribeToClusterStore();
  }

  async authenticateWebDevice(
    rootClusterUri: uri.RootClusterUri,
    {
      id,
      token,
    }: {
      id: string;
      token: string;
    }
  ) {
    return await this.client.authenticateWebDevice({
      rootClusterUri,
      deviceWebToken: {
        id,
        token,
        // empty fields, ignore
        webSessionId: '',
        browserIp: '',
        browserUserAgent: '',
        user: '',
        expectedDeviceIds: [],
      },
    });
  }

  /**
   * Synchronizes the cluster state and starts a headless watcher for it.
   * It shows errors as notifications.
   */
  async syncAndWatchRootClusterWithErrorHandling(
    clusterUri: uri.RootClusterUri
  ) {
    try {
      await this.mainProcessClient.syncCluster(clusterUri);
    } catch (e) {
      const cluster = this.findCluster(clusterUri);
      const clusterName =
        cluster?.name ||
        routing.parseClusterUri(clusterUri).params.rootClusterId;

      const notificationId = this.notificationsService.notifyError({
        title: `Could not synchronize cluster ${clusterName}`,
        description: e.message,
        action: {
          content: 'Retry',
          onClick: () => {
            this.notificationsService.removeNotification(notificationId);
            this.syncAndWatchRootClusterWithErrorHandling(clusterUri);
          },
        },
      });
      // only start the watcher if the cluster was synchronized successfully.
      return;
    }

    try {
      await this.client.startHeadlessWatcher({ rootClusterUri: clusterUri });
    } catch (e) {
      const cluster = this.findCluster(clusterUri);
      const clusterName =
        cluster?.name ||
        routing.parseClusterUri(clusterUri).params.rootClusterId;

      const notificationId = this.notificationsService.notifyError({
        title: `Could not start headless requests watcher for ${clusterName}`,
        description: e.message,
        action: {
          content: 'Retry',
          onClick: () => {
            this.notificationsService.removeNotification(notificationId);
            // retry the entire call
            this.syncAndWatchRootClusterWithErrorHandling(clusterUri);
          },
        },
      });
    }
  }

  /**
   * syncRootCluster is useful in situations where we want to sync the cluster _and_ propagate any
   * errors up.
   */
  async syncRootCluster(clusterUri: uri.RootClusterUri) {
    await this.mainProcessClient.syncCluster(clusterUri);
  }

  /**
   * Synchronizes root clusters.
   *
   * This should only be called before creating workspaces.
   * If called afterward, a cluster might be removed without first removing
   * its associated workspace, resulting in an invalid state.
   */
  async syncRootClustersAndCatchErrors(abortSignal?: AbortSignal) {
    //TODO(gzdunek): Implement passing abort signals over IPC.
    // In this particular case it's fine to discard waiting for the result.
    const abortPromise =
      abortSignal &&
      new Promise<never>((_, reject) => {
        if (abortSignal.aborted) {
          reject(new AbortError());
          return;
        }
        abortSignal.addEventListener('abort', () => reject(new AbortError()), {
          once: true,
        });
      });
    try {
      await Promise.race([
        abortPromise,
        await this.mainProcessClient.syncRootClusters(),
      ]);
    } catch (error) {
      if (isAbortError(error)) {
        this.logger.info('Listing root clusters aborted');
        return;
      }
      const notificationId = this.notificationsService.notifyError({
        title: 'Could not fetch root clusters',
        description: error.message,
        action: {
          content: 'Retry',
          onClick: () => {
            this.notificationsService.removeNotification(notificationId);
            this.syncRootClustersAndCatchErrors();
          },
        },
      });
      return;
    }

    // Sync root clusters and resume headless watchers for any active login sessions.
    this.getRootClusters()
      .filter(c => c.connected)
      .forEach(c => this.syncAndWatchRootClusterWithErrorHandling(c.uri));
  }

  async syncGatewaysAndCatchErrors() {
    try {
      const { response } = await this.client.listGateways({});
      this.setState(draft => {
        draft.gateways = new Map(response.gateways.map(g => [g.uri, g]));
      });
    } catch (error) {
      const notificationId = this.notificationsService.notifyError({
        title: 'Could not synchronize database connections',
        description: error.message,
        action: {
          content: 'Retry',
          onClick: () => {
            this.notificationsService.removeNotification(notificationId);
            this.syncGatewaysAndCatchErrors();
          },
        },
      });
    }
  }

  /** Assumes roles for the given requests. */
  async assumeRoles(
    rootClusterUri: uri.RootClusterUri,
    requestIds: string[]
  ): Promise<void> {
    await this.client.assumeRole({
      rootClusterUri,
      accessRequestIds: requestIds,
      dropRequestIds: [],
    });
    this.usageService.captureAccessRequestAssumeRole(rootClusterUri);
    await this.syncRootCluster(rootClusterUri);
  }

  /** Drops roles for the given requests. */
  async dropRoles(
    rootClusterUri: uri.RootClusterUri,
    requestIds: string[]
  ): Promise<void> {
    await this.client.assumeRole({
      rootClusterUri,
      accessRequestIds: [],
      dropRequestIds: requestIds,
    });
    await this.syncRootCluster(rootClusterUri);
  }

  async reviewAccessRequest(params: ReviewAccessRequestRequest) {
    const { response } = await this.client.reviewAccessRequest(params);
    this.usageService.captureAccessRequestReview(params.rootClusterUri);
    return response.request;
  }

  async promoteAccessRequest(params: PromoteAccessRequestRequest) {
    const { response } = await this.client.promoteAccessRequest(params);
    this.usageService.captureAccessRequestReview(params.rootClusterUri);
    return response.request;
  }

  async createAccessRequest(params: CreateAccessRequestRequest) {
    const response = await this.client.createAccessRequest(params);
    if (!params.dryRun) {
      this.usageService.captureAccessRequestCreate(
        params.rootClusterUri,
        params.roles.length ? 'role' : 'resource'
      );
    }
    return response;
  }

  // TODO(ravicious): Create a single RPC for this rather than sending a separate request for each
  // gateway.
  async removeClusterGateways(clusterUri: uri.RootClusterUri) {
    for (const [, gateway] of this.state.gateways) {
      if (routing.belongsToProfile(clusterUri, gateway.targetUri)) {
        try {
          await this.removeGateway(gateway.uri);
        } catch {
          // Ignore errors as removeGateway already creates a notification for each error.
          // Any gateways that we failed to remove will be forcibly closed on tshd exit.
        }
      }
    }
  }

  async createGateway(params: CreateGatewayRequest) {
    const { response: gateway } = await this.client.createGateway(params);
    this.setState(draft => {
      draft.gateways.set(gateway.uri, gateway);
    });
    return gateway;
  }

  async removeGateway(gatewayUri: uri.GatewayUri) {
    try {
      await this.client.removeGateway({ gatewayUri });
      this.setState(draft => {
        draft.gateways.delete(gatewayUri);
      });
    } catch (error) {
      const gateway = this.findGateway(gatewayUri);
      const gatewayDescription = gateway
        ? `for ${gateway.targetUser}@${gateway.targetName}`
        : gatewayUri;
      const title = `Could not close the database connection ${gatewayDescription}`;

      const notificationId = this.notificationsService.notifyError({
        title,
        description: error.message,
        action: {
          content: 'Retry',
          onClick: () => {
            this.notificationsService.removeNotification(notificationId);
            this.removeGateway(gatewayUri);
          },
        },
      });
      throw error;
    }
  }

  // DELETE IN 15.0.0 (gzdunek),
  // since we will no longer have to support old kube connections.
  // See call in `trackedConnectionOperationsFactory.ts` for more details.
  async removeKubeGateway(kubeUri: uri.KubeUri) {
    const gateway = this.findGatewayByConnectionParams({ targetUri: kubeUri });
    if (gateway) {
      await this.removeGateway(gateway.uri);
    }
  }

  async setGatewayTargetSubresourceName(
    gatewayUri: uri.GatewayUri,
    targetSubresourceName: string
  ) {
    if (!this.findGateway(gatewayUri)) {
      throw new Error(`Could not find gateway ${gatewayUri}`);
    }

    const { response: gateway } =
      await this.client.setGatewayTargetSubresourceName({
        gatewayUri,
        targetSubresourceName,
      });

    this.setState(draft => {
      draft.gateways.set(gatewayUri, gateway);
    });

    return gateway;
  }

  async setGatewayLocalPort(gatewayUri: uri.GatewayUri, localPort: string) {
    if (!this.findGateway(gatewayUri)) {
      throw new Error(`Could not find gateway ${gatewayUri}`);
    }

    const { response: gateway } = await this.client.setGatewayLocalPort({
      gatewayUri,
      localPort,
    });

    this.setState(draft => {
      draft.gateways.set(gatewayUri, gateway);
    });

    return gateway;
  }

  findCluster(clusterUri: uri.ClusterUri) {
    return this.state.clusters.get(clusterUri);
  }

  findGateway(gatewayUri: uri.GatewayUri) {
    return this.state.gateways.get(gatewayUri);
  }

  findGatewayByConnectionParams({
    targetUri,
    targetUser,
    targetSubresourceName,
  }: {
    targetUri: uri.GatewayTargetUri;
    targetUser?: string;
    targetSubresourceName?: string;
  }): Gateway | undefined {
    const targetKind = getGatewayTargetUriKind(targetUri);

    for (const gateway of this.state.gateways.values()) {
      if (gateway.targetUri !== targetUri) {
        continue;
      }

      switch (targetKind) {
        case 'db': {
          if (gateway.targetUser === targetUser) {
            return gateway;
          }
          break;
        }
        case 'kube': {
          // Kube gateways match only on targetUri.
          return gateway;
        }
        case 'app': {
          if (gateway.targetSubresourceName === targetSubresourceName) {
            return gateway;
          }
          break;
        }
        default: {
          targetKind satisfies never;
        }
      }
    }
  }

  /**
   * Returns a root cluster or a leaf cluster to which the given resource belongs to.
   */
  findClusterByResource(uri: uri.ClusterOrResourceUri) {
    const parsed = routing.parseClusterUri(uri);
    if (!parsed) {
      return null;
    }

    const clusterUri = routing.getClusterUri(parsed.params);
    return this.findCluster(clusterUri);
  }

  findRootClusterByResource(uri: string) {
    const parsed = routing.parseClusterUri(uri);
    if (!parsed) {
      return null;
    }

    const rootClusterUri = routing.getClusterUri({
      rootClusterId: parsed.params.rootClusterId,
    });
    return this.findCluster(rootClusterUri);
  }

  getClusters() {
    return [...this.state.clusters.values()];
  }

  getClustersCount() {
    return this.state.clusters.size;
  }

  getRootClusters() {
    return this.getClusters().filter(c => !c.leaf);
  }

  useState() {
    return useStore(this).state;
  }

  private subscribeToClusterStore(): void {
    this.mainProcessClient.subscribeToClusterStore(e => {
      this.setState(c => {
        if (e.kind === 'state') {
          c.clusters = castDraft(e.value);
          return;
        }
        applyPatches(c.clusters, e.value);
      });
    });
  }
}
