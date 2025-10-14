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

import {
  Cluster,
  ShowResources,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import {
  CreateAccessRequestRequest,
  CreateGatewayRequest,
  PromoteAccessRequestRequest,
  ReviewAccessRequestRequest,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import { useStore } from 'shared/libs/stores';
import { isAbortError } from 'shared/utils/error';

import { MainProcessClient } from 'teleterm/mainProcess/types';
import { cloneAbortSignal, TshdClient } from 'teleterm/services/tshd';
import { getGatewayTargetUriKind } from 'teleterm/services/tshd/gateway';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { UsageService } from 'teleterm/ui/services/usage';
import * as uri from 'teleterm/ui/uri';

import { ImmutableStore } from '../immutableStore';

const { routing } = uri;

type ClustersServiceState = {
  clusters: Map<uri.ClusterUri, Cluster>;
  gateways: Map<uri.GatewayUri, Gateway>;
};

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
  }

  async addRootCluster(addr: string) {
    const { response: cluster } = await this.client.addCluster({ name: addr });
    // Do not overwrite the existing cluster;
    // otherwise we may lose properties fetched from the auth server.
    // Consider separating properties read from profile and those
    // fetched from the auth server at the RPC message level.
    if (!this.state.clusters.has(cluster.uri)) {
      this.setState(draft => {
        draft.clusters.set(cluster.uri, cluster);
      });
    }

    return cluster;
  }

  /** Logs out of the cluster. */
  async logout(clusterUri: uri.RootClusterUri) {
    // TODO(gzdunek): logout and removeCluster should be combined into a single acton in tshd
    await this.client.logout({ clusterUri });
    await this.client.removeCluster({ clusterUri });

    this.setState(draft => {
      draft.clusters.forEach(cluster => {
        if (routing.belongsToProfile(clusterUri, cluster.uri)) {
          draft.clusters.delete(cluster.uri);
        }
      });
    });
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
      await this.syncRootCluster(clusterUri);
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
    await Promise.all([
      this.syncClusterInfo(clusterUri),
      this.syncLeafClustersList(clusterUri),
    ]);
  }

  /**
   * Synchronizes root clusters.
   *
   * This should only be called before creating workspaces.
   * If called afterward, a cluster might be removed without first removing
   * its associated workspace, resulting in an invalid state.
   */
  async syncRootClustersAndCatchErrors(abortSignal?: AbortSignal) {
    let clusters: Cluster[];

    try {
      const { response } = await this.client.listRootClusters(
        {},
        { abortSignal: abortSignal && cloneAbortSignal(abortSignal) }
      );
      clusters = response.clusters;
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

    this.setState(draft => {
      draft.clusters = new Map(clusters.map(c => [c.uri, c]));
    });

    // Sync root clusters and resume headless watchers for any active login sessions.
    clusters
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

  private async syncLeafClustersList(clusterUri: uri.RootClusterUri) {
    const { response } = await this.client.listLeafClusters({
      clusterUri,
    });

    this.setState(draft => {
      for (const leaf of response.clusters) {
        draft.clusters.set(leaf.uri, leaf);
      }
    });

    return response.clusters;
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

  private async syncClusterInfo(clusterUri: uri.RootClusterUri) {
    try {
      const { response: cluster } = await this.client.getCluster({
        clusterUri,
      });
      this.setState(draft => {
        draft.clusters.set(clusterUri, cluster);
      });
    } catch (error) {
      this.setState(draft => {
        const cluster = draft.clusters.get(clusterUri);
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
}
