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

import * as api from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';

import { ITerminalServiceClient } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb.client';

import Logger from 'teleterm/logger';
import * as uri from 'teleterm/ui/uri';

import {
  resourceOneOfIsServer,
  resourceOneOfIsDatabase,
  resourceOneOfIsApp,
  resourceOneOfIsKube,
} from 'teleterm/helpers';

import { createFileTransferStream } from './createFileTransferStream';
import * as types from './types';
import {
  ReportUsageEventRequest,
  UpdateHeadlessAuthenticationStateParams,
  UnifiedResourceResponse,
} from './types';
import createAbortController from './createAbortController';

export function createTshdClient(
  tshd: ITerminalServiceClient
): types.TshdClient {
  const logger = new Logger('tshd');

  // Create a client instance that could be shared with the  renderer (UI) via Electron contextBridge
  const client = {
    createAbortController,
    async logout(clusterUri: uri.RootClusterUri) {
      const req: api.LogoutRequest = { clusterUri };
      await tshd.logout(req);
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
      return (
        await tshd.getKubes({
          clusterUri,
          searchAsRoles,
          startKey,
          search,
          query,
          limit,
          sortBy: sort ? `${sort.fieldName}:${sort.dir.toLowerCase()}` : '',
        })
      ).response as types.GetKubesResponse;
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
      return (
        await tshd.getApps({
          clusterUri,
          searchAsRoles,
          startKey,
          search,
          query,
          limit,
          sortBy: sort ? `${sort.fieldName}:${sort.dir.toLowerCase()}` : '',
        })
      ).response as types.GetAppsResponse;
    },

    async listGateways() {
      return (await tshd.listGateways({}).response).gateways as types.Gateway[];
    },

    async listLeafClusters(clusterUri: uri.RootClusterUri) {
      return (await tshd.listLeafClusters({ clusterUri }).response)
        .clusters as types.Cluster[];
    },

    async listRootClusters(abortSignal?: types.TshAbortSignal) {
      return (
        await tshd.listRootClusters(
          {},
          {
            abort: abortSignal,
          }
        ).response
      ).clusters as types.Cluster[];
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
      return (await tshd.getDatabases({
        clusterUri,
        searchAsRoles,
        startKey,
        search,
        query,
        limit,
        sortBy: sort ? `${sort.fieldName}:${sort.dir.toLowerCase()}` : '',
      }).response) as types.GetDatabasesResponse;
    },

    async listDatabaseUsers(dbUri: uri.DatabaseUri) {
      return (await tshd.listDatabaseUsers({ dbUri })).response.users;
    },

    async getAccessRequest(clusterUri: uri.RootClusterUri, requestId: string) {
      return (
        await tshd.getAccessRequest({
          clusterUri,
          accessRequestId: requestId,
        })
      ).response.request;
    },

    async getAccessRequests(clusterUri: uri.RootClusterUri) {
      return (await tshd.getAccessRequests({ clusterUri })).response.requests;
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
      return (await tshd.getServers({
        clusterUri,
        searchAsRoles,
        startKey,
        search,
        query,
        limit,
        sortBy: sort ? `${sort.fieldName}:${sort.dir.toLowerCase()}` : '',
      }).response) as types.GetServersResponse;
    },

    async createAccessRequest(params: types.CreateAccessRequestParams) {
      return (
        await tshd.createAccessRequest({
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
        }).response
      ).request;
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
      return (
        await tshd.reviewAccessRequest({
          rootClusterUri: clusterUri,
          accessRequestId: params.id,
          state: params.state,
          reason: params.reason,
          roles: params.roles,
        }).response
      ).request;
    },

    async getRequestableRoles(params: types.GetRequestableRolesParams) {
      return await tshd.getRequestableRoles({
        clusterUri: params.rootClusterUri,
        resourceIds: params.resourceIds!.map(({ id, clusterName, kind }) => ({
          name: id,
          clusterName,
          kind,
          subResourceName: '',
        })),
      }).response;
    },

    async addRootCluster(addr: string) {
      return (await tshd.addCluster({ name: addr })).response as types.Cluster;
    },

    async getCluster(uri: uri.RootClusterUri) {
      return (await tshd.getCluster({ clusterUri: uri })
        .response) as types.Cluster;
    },

    async loginLocal(
      params: types.LoginLocalParams,
      abortSignal?: types.TshAbortSignal
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
      abortSignal?: types.TshAbortSignal
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
      abortSignal?: types.TshAbortSignal
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
              const pinResponse = pin => {
                const pinRes =
                  api.LoginPasswordlessRequest_LoginPasswordlessPINResponse.create(
                    { pin }
                  );
                stream.requests.send(
                  api.LoginPasswordlessRequest.create({
                    request: {
                      oneofKind: 'pin',
                      pin: pinRes,
                    },
                  })
                );
              };

              params.onPromptCallback({
                type: 'pin',
                onUserResponse: pinResponse,
              });
              return;

            case api.PasswordlessPrompt.CREDENTIAL:
              const credResponse = index => {
                const credRes: api.LoginPasswordlessRequest_LoginPasswordlessCredentialResponse =
                  { index };
                stream.requests.send({
                  request: {
                    oneofKind: 'credential',
                    credential: credRes,
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
      const req: api.GetAuthSettingsRequest = { clusterUri };
      const res = await tshd.getAuthSettings(req);
      return res.response;
    },

    async createGateway(params: types.CreateGatewayParams) {
      const req: api.CreateGatewayRequest = {
        targetUri: params.targetUri,
        targetUser: params.user,
        localPort: params.port,
        targetSubresourceName: params.subresource_name,
      };
      return (await tshd.createGateway(req).response) as types.Gateway;
    },

    async removeCluster(clusterUri: uri.RootClusterUri) {
      const req: api.RemoveClusterRequest = { clusterUri };
      await tshd.removeCluster(req);
    },

    async removeGateway(gatewayUri: uri.GatewayUri) {
      const req: api.RemoveGatewayRequest = { gatewayUri };
      await tshd.removeGateway(req);
    },

    async setGatewayTargetSubresourceName(
      gatewayUri: uri.GatewayUri,
      targetSubresourceName = ''
    ) {
      return (await tshd.setGatewayTargetSubresourceName({
        gatewayUri,
        targetSubresourceName,
      }).response) as types.Gateway;
    },

    async setGatewayLocalPort(gatewayUri: uri.GatewayUri, localPort: string) {
      return (await tshd.setGatewayLocalPort({
        gatewayUri,
        localPort,
      }).response) as types.Gateway;
    },

    transferFile(
      req: types.FileTransferRequest,
      abortSignal: types.TshAbortSignal
    ) {
      return createFileTransferStream(
        tshd.transferFile(req, {
          abort: abortSignal,
        })
      );
    },

    async updateTshdEventsServerAddress(address: string) {
      await tshd.updateTshdEventsServerAddress({ address });
    },

    async reportUsageEvent(event: ReportUsageEventRequest) {
      await tshd.reportUsageEvent(event);
    },

    async createConnectMyComputerRole(rootClusterUri: uri.RootClusterUri) {
      return await tshd.createConnectMyComputerRole({
        rootClusterUri,
      }).response;
    },

    async createConnectMyComputerNodeToken(uri: uri.RootClusterUri) {
      return await tshd.createConnectMyComputerNodeToken({
        rootClusterUri: uri,
      }).response;
    },

    async waitForConnectMyComputerNodeJoin(
      uri: uri.RootClusterUri,
      abortSignal: types.TshAbortSignal
    ) {
      return (await tshd.waitForConnectMyComputerNodeJoin(
        {
          rootClusterUri: uri,
        },
        {
          abort: abortSignal,
        }
      ).response) as types.WaitForConnectMyComputerNodeJoinResponse;
    },

    async deleteConnectMyComputerNode(uri: uri.RootClusterUri) {
      await tshd.deleteConnectMyComputerNode({
        rootClusterUri: uri,
      });
    },

    async getConnectMyComputerNodeName(uri: uri.RootClusterUri) {
      return (
        await tshd.getConnectMyComputerNodeName({
          rootClusterUri: uri,
        }).response
      ).name as uri.ServerUri;
    },

    async updateHeadlessAuthenticationState(
      params: UpdateHeadlessAuthenticationStateParams,
      abortSignal?: types.TshAbortSignal
    ) {
      await tshd.updateHeadlessAuthenticationState(params, {
        abort: abortSignal,
      });
    },

    async listUnifiedResources(
      params: types.ListUnifiedResourcesRequest,
      abortSignal?: types.TshAbortSignal
    ) {
      const req: api.ListUnifiedResourcesRequest = {
        clusterUri: params.clusterUri,
        limit: params.limit,
        kinds: params.kinds,
        startKey: params.startKey || '',
        search: params.search,
        query: params.query,
        pinnedOnly: params.pinnedOnly,
        searchAsRoles: params.searchAsRoles,
        sortBy: params.sortBy || { field: 'name', isDesc: false },
      };
      const res = await tshd.listUnifiedResources(req, {
        abort: abortSignal,
      }).response;
      return {
        nextKey: res.nextKey,
        resources: res.resources
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
      abortSignal?: types.TshAbortSignal
    ): Promise<api.UserPreferences> {
      return (
        await tshd.getUserPreferences(params, {
          abort: abortSignal,
        }).response
      ).userPreferences;
    },
    async updateUserPreferences(
      params: api.UpdateUserPreferencesRequest,
      abortSignal?: types.TshAbortSignal
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

      const req: api.UpdateUserPreferencesRequest = {
        clusterUri: params.clusterUri,
        userPreferences,
      };
      return (
        await tshd.updateUserPreferences(req, {
          abort: abortSignal,
        }).response
      ).userPreferences;
    },
    async promoteAccessRequest(
      params: api.PromoteAccessRequestRequest,
      abortSignal?: types.TshAbortSignal
    ): Promise<types.AccessRequest> {
      return (
        await tshd.promoteAccessRequest(params, {
          abort: abortSignal,
        }).response
      ).request;
    },
    async getSuggestedAccessLists(
      params: api.GetSuggestedAccessListsRequest,
      abortSignal?: types.TshAbortSignal
    ): Promise<types.AccessList[]> {
      return (
        await tshd.getSuggestedAccessLists(params, {
          abort: abortSignal,
        }).response
      ).accessLists;
    },
  };

  return client;
}
