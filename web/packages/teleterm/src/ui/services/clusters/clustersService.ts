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

import { AccessRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';
import {
  Cluster,
  ShowResources,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import {
  CreateAccessRequestRequest,
  CreateGatewayRequest,
  PasswordlessPrompt,
  PromoteAccessRequestRequest,
  ReviewAccessRequestRequest,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import { useStore } from 'shared/libs/stores';
import { isAbortError } from 'shared/utils/abortError';
import { pipe } from 'shared/utils/pipe';

import { MainProcessClient } from 'teleterm/mainProcess/types';
import {
  CloneableAbortSignal,
  cloneAbortSignal,
  TshdClient,
} from 'teleterm/services/tshd';
import { getGatewayTargetUriKind } from 'teleterm/services/tshd/gateway';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { UsageService } from 'teleterm/ui/services/usage';
import * as uri from 'teleterm/ui/uri';

import { ImmutableStore } from '../immutableStore';
import type * as types from './types';

const { routing } = uri;

export function createClusterServiceState(): types.ClustersServiceState {
  return {
    clusters: new Map(),
    gateways: new Map(),
  };
}

export class ClustersService extends ImmutableStore<types.ClustersServiceState> {
  state: types.ClustersServiceState = createClusterServiceState();

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
        draft.clusters.set(
          cluster.uri,
          this.removeInternalLoginsFromCluster(cluster)
        );
      });
    }

    return cluster;
  }

  /**
   * Logs out of the cluster and removes the profile.
   * Does not remove the cluster from the state, but sets the cluster and its leafs as disconnected.
   * It needs to be done, because some code can operate on the cluster the intermediate period between logout
   * and actually removing it from the state.
   * A code that operates on that intermediate state is in `useClusterLogout.tsx`.
   * After invoking `logout()`, it looks for the next workspace to switch to. If we hadn't marked the cluster as disconnected,
   * the method might have returned us the same cluster we wanted to log out of.
   */
  async logout(clusterUri: uri.RootClusterUri) {
    // TODO(gzdunek): logout and removeCluster should be combined into a single acton in tshd
    await this.client.logout({ clusterUri });
    await this.client.removeCluster({ clusterUri });

    this.setState(draft => {
      draft.clusters.forEach(cluster => {
        if (routing.belongsToProfile(clusterUri, cluster.uri)) {
          cluster.connected = false;
        }
      });
    });
  }

  async loginLocal(
    params: types.LoginLocalParams,
    abortSignal: CloneableAbortSignal
  ) {
    await this.client.login(
      {
        clusterUri: params.clusterUri,
        params: {
          oneofKind: 'local',
          local: {
            user: params.username,
            password: params.password,
            token: params.token,
          },
        },
      },
      { abort: abortSignal }
    );
    // We explicitly use the `andCatchErrors` variant here. If loginLocal succeeds but syncing the
    // cluster fails, we don't want to stop the user on the failed modal – we want to open the
    // workspace and show an error state within the workspace.
    await this.syncAndWatchRootClusterWithErrorHandling(params.clusterUri);
    this.usageService.captureUserLogin(params.clusterUri, 'local');
  }

  async loginSso(
    params: types.LoginSsoParams,
    abortSignal: CloneableAbortSignal
  ) {
    await this.client.login(
      {
        clusterUri: params.clusterUri,
        params: {
          oneofKind: 'sso',
          sso: {
            providerType: params.providerType,
            providerName: params.providerName,
          },
        },
      },
      { abort: abortSignal }
    );
    await this.syncAndWatchRootClusterWithErrorHandling(params.clusterUri);
    this.usageService.captureUserLogin(params.clusterUri, params.providerType);
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

  async loginPasswordless(
    params: types.LoginPasswordlessParams,
    abortSignal: CloneableAbortSignal
  ) {
    await new Promise<void>((resolve, reject) => {
      const stream = this.client.loginPasswordless({
        abort: abortSignal,
      });

      let hasDeviceBeenTapped = false;

      // Init the stream.
      stream.requests.send({
        request: {
          oneofKind: 'init',
          init: {
            clusterUri: params.clusterUri,
          },
        },
      });

      stream.responses.onMessage(function (response) {
        switch (response.prompt) {
          case PasswordlessPrompt.PIN:
            const pinResponse = (pin: string) => {
              stream.requests.send({
                request: {
                  oneofKind: 'pin',
                  pin: { pin },
                },
              });
            };

            params.onPromptCallback({
              type: 'pin',
              onUserResponse: pinResponse,
            });
            return;

          case PasswordlessPrompt.CREDENTIAL:
            const credResponse = (index: number) => {
              stream.requests.send({
                request: {
                  oneofKind: 'credential',
                  credential: { index: BigInt(index) },
                },
              });
            };

            params.onPromptCallback({
              type: 'credential',
              onUserResponse: credResponse,
              data: { credentials: response.credentials || [] },
            });
            return;

          case PasswordlessPrompt.TAP:
            if (hasDeviceBeenTapped) {
              params.onPromptCallback({ type: 'retap' });
            } else {
              hasDeviceBeenTapped = true;
              params.onPromptCallback({ type: 'tap' });
            }
            return;

          // Following cases should never happen but just in case?
          case PasswordlessPrompt.UNSPECIFIED:
            stream.requests.complete();
            return reject(new Error('no passwordless prompt was specified'));

          default:
            stream.requests.complete();
            return reject(
              new Error(
                `passwordless prompt '${response.prompt}' not supported`
              )
            );
        }
      });

      stream.responses.onComplete(function () {
        resolve();
      });

      stream.responses.onError(function (err: Error) {
        reject(err);
      });
    });

    await this.syncAndWatchRootClusterWithErrorHandling(params.clusterUri);
    this.usageService.captureUserLogin(params.clusterUri, 'passwordless');
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
      draft.clusters = new Map(
        clusters.map(c => [c.uri, this.removeInternalLoginsFromCluster(c)])
      );
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
        draft.clusters.set(
          leaf.uri,
          this.removeInternalLoginsFromCluster(leaf)
        );
      }
    });

    return response.clusters;
  }

  getAssumedRequests(
    rootClusterUri: uri.RootClusterUri
  ): Record<string, AccessRequest> {
    return getAssumedRequests(this.state, rootClusterUri);
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

  async getAccessRequest(
    rootClusterUri: uri.RootClusterUri,
    requestId: string
  ) {
    const { response } = await this.client.getAccessRequest({
      clusterUri: rootClusterUri,
      accessRequestId: requestId,
    });

    return response.request;
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

  /** Removes cluster, its leafs and other resources. */
  async removeClusterAndResources(clusterUri: uri.RootClusterUri) {
    this.setState(draft => {
      draft.clusters.forEach(cluster => {
        if (routing.belongsToProfile(clusterUri, cluster.uri)) {
          draft.clusters.delete(cluster.uri);
        }
      });
    });
    await this.removeClusterKubeConfigs(clusterUri);
    await this.removeClusterGateways(clusterUri);
  }

  // TODO(ravicious): Create a single RPC for this rather than sending a separate request for each
  // gateway.
  private async removeClusterGateways(clusterUri: uri.RootClusterUri) {
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

  async getAuthSettings(clusterUri: uri.RootClusterUri) {
    const { response } = await this.client.getAuthSettings({ clusterUri });
    return response as types.AuthSettings;
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

  async removeClusterKubeConfigs(clusterUri: string): Promise<void> {
    const {
      params: { rootClusterId },
    } = routing.parseClusterUri(clusterUri);
    return this.mainProcessClient.removeKubeConfig({
      relativePath: rootClusterId,
      isDirectory: true,
    });
  }

  async removeKubeConfig(kubeConfigRelativePath: string): Promise<void> {
    return this.mainProcessClient.removeKubeConfig({
      relativePath: kubeConfigRelativePath,
    });
  }

  useState() {
    return useStore(this).state;
  }

  private async syncClusterInfo(clusterUri: uri.RootClusterUri) {
    try {
      const { response: cluster } = await this.client.getCluster({
        clusterUri,
      });
      // TODO: this information should eventually be gathered by getCluster
      const assumedRequests = cluster.loggedInUser
        ? await this.fetchClusterAssumedRequests(
            cluster.loggedInUser.activeRequests,
            clusterUri
          )
        : undefined;
      const mergeAssumedRequests = (cluster: Cluster) => ({
        ...cluster,
        loggedInUser: cluster.loggedInUser && {
          ...cluster.loggedInUser,
          assumedRequests,
        },
      });
      const processCluster = pipe(
        this.removeInternalLoginsFromCluster,
        mergeAssumedRequests
      );

      this.setState(draft => {
        draft.clusters.set(clusterUri, processCluster(cluster));
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

  private async fetchClusterAssumedRequests(
    activeRequestsList: string[],
    clusterUri: uri.RootClusterUri
  ) {
    return (
      await Promise.all(
        activeRequestsList.map(requestId =>
          this.getAccessRequest(clusterUri, requestId)
        )
      )
    ).reduce((requestsMap, request) => {
      requestsMap[request.id] = request;
      return requestsMap;
    }, {});
  }

  // temporary fix for https://github.com/gravitational/webapps.e/issues/294
  // remove when it will get fixed in `tsh`
  // alternatively, show only valid logins basing on RBAC check
  private removeInternalLoginsFromCluster(cluster: Cluster): Cluster {
    return {
      ...cluster,
      loggedInUser: cluster.loggedInUser && {
        ...cluster.loggedInUser,
        sshLogins: cluster.loggedInUser.sshLogins.filter(
          login => !login.startsWith('-')
        ),
      },
    };
  }
}

// A workaround to always return the same object so useEffect that relies on it
// doesn't go into an endless loop.
const EMPTY_ASSUMED_REQUESTS = {};

export function getAssumedRequests(
  state: types.ClustersServiceState,
  rootClusterUri: uri.RootClusterUri
): Record<string, AccessRequest> {
  const cluster = state.clusters.get(rootClusterUri);
  return cluster?.loggedInUser?.assumedRequests || EMPTY_ASSUMED_REQUESTS;
}
