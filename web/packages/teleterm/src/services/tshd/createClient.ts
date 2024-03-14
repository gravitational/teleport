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

import grpc from '@grpc/grpc-js';
import { GrpcTransport } from '@protobuf-ts/grpc-transport';
import * as api from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import { TerminalServiceClient } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb.client';

import Logger from 'teleterm/logger';
import * as uri from 'teleterm/ui/uri';

import {
  resourceOneOfIsApp,
  resourceOneOfIsDatabase,
  resourceOneOfIsKube,
  resourceOneOfIsServer,
} from 'teleterm/helpers';

import { CloneableAbortSignal, cloneClient } from './cloneableClient';
import * as types from './types';
import {
  UpdateHeadlessAuthenticationStateParams,
  UnifiedResourceResponse,
} from './types';
import { loggingInterceptor } from './interceptors';

export function createTshdClient(
  addr: string,
  credentials: grpc.ChannelCredentials
): types.TshdClient {
  const logger = new Logger('tshd');
  const transport = new GrpcTransport({
    host: addr,
    channelCredentials: credentials,
    interceptors: [loggingInterceptor(logger)],
  });
  const tshd = cloneClient(new TerminalServiceClient(transport));

  // Create a client instance that could be shared with the  renderer (UI) via Electron contextBridge
  const client = {
    async logout(clusterUri: uri.RootClusterUri) {
      await tshd.logout({ clusterUri });
    },

    async getKubes({
      clusterUri,
      search,
      sort,
      query,
      searchAsRoles,
      startKey,
      limit,
    }: types.GetResourcesParams) {
      const { response } = await tshd.getKubes({
        clusterUri,
        searchAsRoles,
        startKey,
        search,
        query,
        limit,
        sortBy: sort ? `${sort.fieldName}:${sort.dir.toLowerCase()}` : '',
      });

      return response as types.GetKubesResponse;
    },

    async getApps({
      clusterUri,
      search,
      sort,
      query,
      searchAsRoles,
      startKey,
      limit,
    }: types.GetResourcesParams) {
      const { response } = await tshd.getApps({
        clusterUri,
        searchAsRoles,
        startKey,
        search,
        query,
        limit,
        sortBy: sort ? `${sort.fieldName}:${sort.dir.toLowerCase()}` : '',
      });

      return response as types.GetAppsResponse;
    },

    async listGateways() {
      const { response } = await tshd.listGateways({});

      return response.gateways as types.Gateway[];
    },

    async listLeafClusters(clusterUri: uri.RootClusterUri) {
      const { response } = await tshd.listLeafClusters({ clusterUri });

      return response.clusters as types.Cluster[];
    },

    async listRootClusters(abortSignal?: CloneableAbortSignal) {
      const { response } = await tshd.listRootClusters(
        {},
        {
          abort: abortSignal,
        }
      );

      return response.clusters as types.Cluster[];
    },

    async getDatabases({
      clusterUri,
      search,
      sort,
      query,
      searchAsRoles,
      startKey,
      limit,
    }: types.GetResourcesParams) {
      const { response } = await tshd.getDatabases({
        clusterUri,
        searchAsRoles,
        startKey,
        search,
        query,
        limit,
        sortBy: sort ? `${sort.fieldName}:${sort.dir.toLowerCase()}` : '',
      });

      return response as types.GetDatabasesResponse;
    },

    async listDatabaseUsers(dbUri: uri.DatabaseUri) {
      const { response } = await tshd.listDatabaseUsers({ dbUri });

      return response.users;
    },

    async getAccessRequest(clusterUri: uri.RootClusterUri, requestId: string) {
      const { response } = await tshd.getAccessRequest({
        clusterUri,
        accessRequestId: requestId,
      });

      return response.request;
    },

    async getAccessRequests(clusterUri: uri.RootClusterUri) {
      const { response } = await tshd.getAccessRequests({ clusterUri });

      return response.requests;
    },

    async getServers({
      clusterUri,
      search,
      query,
      sort,
      searchAsRoles,
      startKey,
      limit,
    }: types.GetResourcesParams) {
      const { response } = await tshd.getServers({
        clusterUri,
        searchAsRoles,
        startKey,
        search,
        query,
        limit,
        sortBy: sort ? `${sort.fieldName}:${sort.dir.toLowerCase()}` : '',
      });
      return response as types.GetServersResponse;
    },

    async createAccessRequest(params: types.CreateAccessRequestParams) {
      const { response } = await tshd.createAccessRequest({
        rootClusterUri: params.rootClusterUri,
        suggestedReviewers: params.suggestedReviewers,
        roles: params.roles,
        reason: params.reason,
        resourceIds: params.resourceIds.map(({ id, clusterName, kind }) => ({
          name: id,
          clusterName,
          kind,
          subResourceName: '',
        })),
      });

      return response.request;
    },

    async deleteAccessRequest(
      clusterUri: uri.RootClusterUri,
      requestId: string
    ) {
      await tshd.deleteAccessRequest({
        rootClusterUri: clusterUri,
        accessRequestId: requestId,
      });
    },

    async assumeRole(
      clusterUri: uri.RootClusterUri,
      requestIds: string[],
      dropIds: string[]
    ) {
      await tshd.assumeRole({
        rootClusterUri: clusterUri,
        accessRequestIds: requestIds,
        dropRequestIds: dropIds,
      });
    },

    async reviewAccessRequest(
      clusterUri: uri.RootClusterUri,
      params: types.ReviewAccessRequestParams
    ) {
      const { response } = await tshd.reviewAccessRequest({
        rootClusterUri: clusterUri,
        accessRequestId: params.id,
        state: params.state,
        reason: params.reason,
        roles: params.roles,
      });

      return response.request;
    },

    async getRequestableRoles(params: types.GetRequestableRolesParams) {
      const { response } = await tshd.getRequestableRoles({
        clusterUri: params.rootClusterUri,
        resourceIds: params.resourceIds!.map(({ id, clusterName, kind }) => ({
          name: id,
          clusterName,
          kind,
          subResourceName: '',
        })),
      });

      return response;
    },

    async addRootCluster(addr: string) {
      const { response } = await tshd.addCluster({ name: addr });
      return response as types.Cluster;
    },

    async getCluster(uri: uri.RootClusterUri) {
      const { response } = await tshd.getCluster({ clusterUri: uri });
      return response as types.Cluster;
    },

    async loginLocal(
      params: types.LoginLocalParams,
      abortSignal?: CloneableAbortSignal
    ) {
      await tshd.login(
        {
          clusterUri: params.clusterUri,
          params: {
            oneofKind: 'local',
            local: {
              token: params.token,
              user: params.username,
              password: params.password,
            },
          },
        },
        {
          abort: abortSignal,
        }
      );
    },

    async loginSso(
      params: types.LoginSsoParams,
      abortSignal?: CloneableAbortSignal
    ) {
      await tshd.login(
        {
          clusterUri: params.clusterUri,
          params: {
            oneofKind: 'sso',
            sso: {
              providerName: params.providerName,
              providerType: params.providerType,
            },
          },
        },
        { abort: abortSignal }
      );
    },

    async loginPasswordless(
      params: types.LoginPasswordlessParams,
      abortSignal?: CloneableAbortSignal
    ) {
      return new Promise<void>((resolve, reject) => {
        const stream = tshd.loginPasswordless({
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
            case api.PasswordlessPrompt.PIN:
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

            case api.PasswordlessPrompt.CREDENTIAL:
              const credResponse = (index: number) => {
                stream.requests.send({
                  request: {
                    oneofKind: 'credential',
                    credential: { index },
                  },
                });
              };

              params.onPromptCallback({
                type: 'credential',
                onUserResponse: credResponse,
                data: { credentials: response.credentials || [] },
              });
              return;

            case api.PasswordlessPrompt.TAP:
              if (hasDeviceBeenTapped) {
                params.onPromptCallback({ type: 'retap' });
              } else {
                hasDeviceBeenTapped = true;
                params.onPromptCallback({ type: 'tap' });
              }
              return;

            // Following cases should never happen but just in case?
            case api.PasswordlessPrompt.UNSPECIFIED:
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
    },

    async getAuthSettings(clusterUri: uri.RootClusterUri) {
      const { response } = await tshd.getAuthSettings({ clusterUri });
      return response;
    },

    async createGateway(params: types.CreateGatewayParams) {
      const { response } = await tshd.createGateway({
        targetUri: params.targetUri,
        targetUser: params.user,
        localPort: params.port,
        targetSubresourceName: params.subresource_name,
      });

      return response as types.Gateway;
    },

    async removeCluster(clusterUri: uri.RootClusterUri) {
      await tshd.removeCluster({ clusterUri });
    },

    async removeGateway(gatewayUri: uri.GatewayUri) {
      await tshd.removeGateway({ gatewayUri });
    },

    async setGatewayTargetSubresourceName(
      gatewayUri: uri.GatewayUri,
      targetSubresourceName = ''
    ) {
      const { response } = await tshd.setGatewayTargetSubresourceName({
        gatewayUri,
        targetSubresourceName,
      });

      return response as types.Gateway;
    },

    async setGatewayLocalPort(gatewayUri: uri.GatewayUri, localPort: string) {
      const { response } = await tshd.setGatewayLocalPort({
        gatewayUri,
        localPort,
      });

      return response as types.Gateway;
    },

    transferFile(
      req: types.FileTransferRequest,
      abortSignal: CloneableAbortSignal
    ) {
      const stream = tshd.transferFile(req, {
        abort: abortSignal,
      });

      return {
        onProgress(callback: (progress: number) => void) {
          stream.responses.onMessage(data => callback(data.percentage));
        },
        onComplete: stream.responses.onComplete,
        onError: stream.responses.onError,
      };
    },

    async updateTshdEventsServerAddress(address: string) {
      await tshd.updateTshdEventsServerAddress({ address });
    },

    reportUsageEvent: tshd.reportUsageEvent,

    async createConnectMyComputerRole(rootClusterUri: uri.RootClusterUri) {
      const { response } = await tshd.createConnectMyComputerRole({
        rootClusterUri,
      });

      return response;
    },

    async createConnectMyComputerNodeToken(uri: uri.RootClusterUri) {
      const { response } = await tshd.createConnectMyComputerNodeToken({
        rootClusterUri: uri,
      });

      return response;
    },

    async waitForConnectMyComputerNodeJoin(
      uri: uri.RootClusterUri,
      abortSignal: CloneableAbortSignal
    ) {
      const { response } = await tshd.waitForConnectMyComputerNodeJoin(
        {
          rootClusterUri: uri,
        },
        {
          abort: abortSignal,
        }
      );

      return response as types.WaitForConnectMyComputerNodeJoinResponse;
    },

    async deleteConnectMyComputerNode(uri: uri.RootClusterUri) {
      await tshd.deleteConnectMyComputerNode({
        rootClusterUri: uri,
      });
    },

    async getConnectMyComputerNodeName(uri: uri.RootClusterUri) {
      const { response } = await tshd.getConnectMyComputerNodeName({
        rootClusterUri: uri,
      });

      return response.name as uri.ServerUri;
    },

    async updateHeadlessAuthenticationState(
      params: UpdateHeadlessAuthenticationStateParams,
      abortSignal?: CloneableAbortSignal
    ) {
      await tshd.updateHeadlessAuthenticationState(params, {
        abort: abortSignal,
      });
    },

    async listUnifiedResources(
      params: types.ListUnifiedResourcesRequest,
      abortSignal?: CloneableAbortSignal
    ) {
      const { response } = await tshd.listUnifiedResources(params, {
        abort: abortSignal,
      });
      return {
        nextKey: response.nextKey,
        resources: response.resources
          .map(p => {
            if (resourceOneOfIsServer(p.resource)) {
              return {
                kind: 'server',
                resource: p.resource.server,
              };
            }

            if (resourceOneOfIsDatabase(p.resource)) {
              return {
                kind: 'database',
                resource: p.resource.database,
              };
            }

            if (resourceOneOfIsApp(p.resource)) {
              return {
                kind: 'app',
                resource: p.resource.app,
              };
            }

            if (resourceOneOfIsKube(p.resource)) {
              return {
                kind: 'kube',
                resource: p.resource.kube,
              };
            }

            logger.info(`Ignoring unsupported resource ${JSON.stringify(p)}.`);
          })
          .filter(Boolean) as UnifiedResourceResponse[],
      };
    },
    async getUserPreferences(
      params: api.GetUserPreferencesRequest,
      abortSignal?: CloneableAbortSignal
    ): Promise<api.UserPreferences> {
      const { response } = await tshd.getUserPreferences(params, {
        abort: abortSignal,
      });

      return response.userPreferences;
    },
    async updateUserPreferences(
      params: api.UpdateUserPreferencesRequest,
      abortSignal?: CloneableAbortSignal
    ): Promise<api.UserPreferences> {
      const userPreferences: api.UserPreferences = {};
      if (params.userPreferences.clusterPreferences) {
        userPreferences.clusterPreferences = {
          pinnedResources: {
            resourceIds:
              params.userPreferences.clusterPreferences.pinnedResources
                ?.resourceIds,
          },
        };
      }

      if (params.userPreferences.unifiedResourcePreferences) {
        userPreferences.unifiedResourcePreferences = {
          defaultTab:
            params.userPreferences.unifiedResourcePreferences.defaultTab,
          viewMode: params.userPreferences.unifiedResourcePreferences.viewMode,
          labelsViewMode:
            params.userPreferences.unifiedResourcePreferences.labelsViewMode,
        };
      }

      const { response } = await tshd.updateUserPreferences(
        {
          clusterUri: params.clusterUri,
          userPreferences,
        },
        {
          abort: abortSignal,
        }
      );

      return response.userPreferences;
    },
    async promoteAccessRequest(
      params: api.PromoteAccessRequestRequest,
      abortSignal?: CloneableAbortSignal
    ): Promise<types.AccessRequest> {
      const { response } = await tshd.promoteAccessRequest(params, {
        abort: abortSignal,
      });
      return response.request;
    },

    async getSuggestedAccessLists(
      params: api.GetSuggestedAccessListsRequest,
      abortSignal?: CloneableAbortSignal
    ): Promise<types.AccessList[]> {
      const { response } = await tshd.getSuggestedAccessLists(params, {
        abort: abortSignal,
      });

      return response.accessLists;
    },
  };

  return client;
}
