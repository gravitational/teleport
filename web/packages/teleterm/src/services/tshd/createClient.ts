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

import {
  CloneableAbortSignal,
  cloneUnaryCall,
  cloneDuplexStreamingCall,
  cloneServerStreamingCall,
} from './cloneableClient';
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
  const tshd = new TerminalServiceClient(transport);

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
      const getKubes = cloneUnaryCall(tshd.getKubes.bind(tshd));
      const { response } = await getKubes({
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
      const getApps = cloneUnaryCall(tshd.getApps.bind(tshd));
      const { response } = await getApps({
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
      const listGateways = cloneUnaryCall(tshd.listGateways.bind(tshd));
      const { response } = await listGateways({});

      return response.gateways as types.Gateway[];
    },

    async listLeafClusters(clusterUri: uri.RootClusterUri) {
      const listLeafClusters = cloneUnaryCall(tshd.listLeafClusters.bind(tshd));
      const { response } = await listLeafClusters({ clusterUri });

      return response.clusters as types.Cluster[];
    },

    async listRootClusters(abortSignal?: CloneableAbortSignal) {
      const listRootClusters = cloneUnaryCall(tshd.listRootClusters.bind(tshd));
      const { response } = await listRootClusters(
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
      const getDatabases = cloneUnaryCall(tshd.getDatabases.bind(tshd));
      const { response } = await getDatabases({
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
      const listDatabaseUsers = cloneUnaryCall(
        tshd.listDatabaseUsers.bind(tshd)
      );
      const { response } = await listDatabaseUsers({ dbUri });

      return response.users;
    },

    async getAccessRequest(clusterUri: uri.RootClusterUri, requestId: string) {
      const getAccessRequest = cloneUnaryCall(tshd.getAccessRequest.bind(tshd));
      const { response } = await getAccessRequest({
        clusterUri,
        accessRequestId: requestId,
      });

      return response.request;
    },

    async getAccessRequests(clusterUri: uri.RootClusterUri) {
      const getAccessRequests = cloneUnaryCall(
        tshd.getAccessRequests.bind(tshd)
      );
      const { response } = await getAccessRequests({ clusterUri });

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
      const getServers = cloneUnaryCall(tshd.getServers.bind(tshd));
      const { response } = await getServers({
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
      const createAccessRequest = cloneUnaryCall(
        tshd.createAccessRequest.bind(tshd)
      );
      const { response } = await createAccessRequest({
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
      const deleteAccessRequest = cloneUnaryCall(
        tshd.deleteAccessRequest.bind(tshd)
      );
      await deleteAccessRequest({
        rootClusterUri: clusterUri,
        accessRequestId: requestId,
      });
    },

    async assumeRole(
      clusterUri: uri.RootClusterUri,
      requestIds: string[],
      dropIds: string[]
    ) {
      const assumeRole = cloneUnaryCall(tshd.assumeRole.bind(tshd));
      await assumeRole({
        rootClusterUri: clusterUri,
        accessRequestIds: requestIds,
        dropRequestIds: dropIds,
      });
    },

    async reviewAccessRequest(
      clusterUri: uri.RootClusterUri,
      params: types.ReviewAccessRequestParams
    ) {
      const reviewAccessRequest = cloneUnaryCall(
        tshd.reviewAccessRequest.bind(tshd)
      );
      const { response } = await reviewAccessRequest({
        rootClusterUri: clusterUri,
        accessRequestId: params.id,
        state: params.state,
        reason: params.reason,
        roles: params.roles,
      });

      return response.request;
    },

    async getRequestableRoles(params: types.GetRequestableRolesParams) {
      const getRequestableRoles = cloneUnaryCall(
        tshd.getRequestableRoles.bind(tshd)
      );
      const { response } = await getRequestableRoles({
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
      const addCluster = cloneUnaryCall(tshd.addCluster.bind(tshd));
      const { response } = await addCluster({ name: addr });
      return response as types.Cluster;
    },

    async getCluster(uri: uri.RootClusterUri) {
      const getCluster = cloneUnaryCall(tshd.getCluster.bind(tshd));
      const { response } = await getCluster({ clusterUri: uri });
      return response as types.Cluster;
    },

    async loginLocal(
      params: types.LoginLocalParams,
      abortSignal?: CloneableAbortSignal
    ) {
      const login = cloneUnaryCall(tshd.login.bind(tshd));
      await login(
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
      const login = cloneUnaryCall(tshd.login.bind(tshd));
      await login(
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
      const loginPasswordless = cloneDuplexStreamingCall(
        tshd.loginPasswordless.bind(tshd)
      );
      return new Promise<void>((resolve, reject) => {
        const stream = loginPasswordless({
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
      const getAuthSettings = cloneUnaryCall(tshd.getAuthSettings.bind(tshd));
      const { response } = await getAuthSettings({ clusterUri });
      return response;
    },

    async createGateway(params: types.CreateGatewayParams) {
      const createGateway = cloneUnaryCall(tshd.createGateway.bind(tshd));
      const { response } = await createGateway({
        targetUri: params.targetUri,
        targetUser: params.user,
        localPort: params.port,
        targetSubresourceName: params.subresource_name,
      });

      return response as types.Gateway;
    },

    async removeCluster(clusterUri: uri.RootClusterUri) {
      const removeCluster = cloneUnaryCall(tshd.removeCluster.bind(tshd));
      await removeCluster({ clusterUri });
    },

    async removeGateway(gatewayUri: uri.GatewayUri) {
      const removeGateway = cloneUnaryCall(tshd.removeGateway.bind(tshd));
      await removeGateway({ gatewayUri });
    },

    async setGatewayTargetSubresourceName(
      gatewayUri: uri.GatewayUri,
      targetSubresourceName = ''
    ) {
      const setGatewayTargetSubresourceName = cloneUnaryCall(
        tshd.setGatewayTargetSubresourceName.bind(tshd)
      );
      const { response } = await setGatewayTargetSubresourceName({
        gatewayUri,
        targetSubresourceName,
      });

      return response as types.Gateway;
    },

    async setGatewayLocalPort(gatewayUri: uri.GatewayUri, localPort: string) {
      const setGatewayLocalPort = cloneUnaryCall(
        tshd.setGatewayLocalPort.bind(tshd)
      );
      const { response } = await setGatewayLocalPort({
        gatewayUri,
        localPort,
      });

      return response as types.Gateway;
    },

    transferFile(
      req: types.FileTransferRequest,
      abortSignal: CloneableAbortSignal
    ) {
      const transferFile = cloneServerStreamingCall(
        tshd.transferFile.bind(tshd)
      );

      const stream = transferFile(req, {
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
      const updateTshdEventsServerAddress = cloneUnaryCall(
        tshd.updateTshdEventsServerAddress.bind(tshd)
      );
      await updateTshdEventsServerAddress({ address });
    },

    reportUsageEvent: cloneUnaryCall(tshd.reportUsageEvent.bind(tshd)),

    async createConnectMyComputerRole(rootClusterUri: uri.RootClusterUri) {
      const createConnectMyComputerRole = cloneUnaryCall(
        tshd.createConnectMyComputerRole.bind(tshd)
      );
      const { response } = await createConnectMyComputerRole({
        rootClusterUri,
      });

      return response;
    },

    async createConnectMyComputerNodeToken(uri: uri.RootClusterUri) {
      const createConnectMyComputerNodeToken = cloneUnaryCall(
        tshd.createConnectMyComputerNodeToken.bind(tshd)
      );
      const { response } = await createConnectMyComputerNodeToken({
        rootClusterUri: uri,
      });

      return response;
    },

    async waitForConnectMyComputerNodeJoin(
      uri: uri.RootClusterUri,
      abortSignal: CloneableAbortSignal
    ) {
      const waitForConnectMyComputerNodeJoin = cloneUnaryCall(
        tshd.waitForConnectMyComputerNodeJoin.bind(tshd)
      );
      const { response } = await waitForConnectMyComputerNodeJoin(
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
      const deleteConnectMyComputerNode = cloneUnaryCall(
        tshd.deleteConnectMyComputerNode.bind(tshd)
      );
      await deleteConnectMyComputerNode({
        rootClusterUri: uri,
      });
    },

    async getConnectMyComputerNodeName(uri: uri.RootClusterUri) {
      const getConnectMyComputerNodeName = cloneUnaryCall(
        tshd.getConnectMyComputerNodeName.bind(tshd)
      );
      const { response } = await getConnectMyComputerNodeName({
        rootClusterUri: uri,
      });

      return response.name as uri.ServerUri;
    },

    async updateHeadlessAuthenticationState(
      params: UpdateHeadlessAuthenticationStateParams,
      abortSignal?: CloneableAbortSignal
    ) {
      const updateHeadlessAuthenticationState = cloneUnaryCall(
        tshd.updateHeadlessAuthenticationState.bind(tshd)
      );
      await updateHeadlessAuthenticationState(params, {
        abort: abortSignal,
      });
    },

    async listUnifiedResources(
      params: types.ListUnifiedResourcesRequest,
      abortSignal?: CloneableAbortSignal
    ) {
      const listUnifiedResources = cloneUnaryCall(
        tshd.listUnifiedResources.bind(tshd)
      );
      const { response } = await listUnifiedResources(
        {
          clusterUri: params.clusterUri,
          limit: params.limit,
          kinds: params.kinds,
          startKey: params.startKey || '',
          search: params.search,
          query: params.query,
          pinnedOnly: params.pinnedOnly,
          searchAsRoles: params.searchAsRoles,
          sortBy: params.sortBy || { field: 'name', isDesc: false },
        },
        {
          abort: abortSignal,
        }
      );
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
      const getUserPreferences = cloneUnaryCall(
        tshd.getUserPreferences.bind(tshd)
      );
      const { response } = await getUserPreferences(params, {
        abort: abortSignal,
      });

      return response.userPreferences;
    },
    async updateUserPreferences(
      params: api.UpdateUserPreferencesRequest,
      abortSignal?: CloneableAbortSignal
    ): Promise<api.UserPreferences> {
      const updateUserPreferences = cloneUnaryCall(
        tshd.updateUserPreferences.bind(tshd)
      );
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

      const { response } = await updateUserPreferences(
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
      const promoteAccessRequest = cloneUnaryCall(
        tshd.promoteAccessRequest.bind(tshd)
      );
      const { response } = await promoteAccessRequest(params, {
        abort: abortSignal,
      });
      return response.request;
    },

    async getSuggestedAccessLists(
      params: api.GetSuggestedAccessListsRequest,
      abortSignal?: CloneableAbortSignal
    ): Promise<types.AccessList[]> {
      const getSuggestedAccessLists = cloneUnaryCall(
        tshd.getSuggestedAccessLists.bind(tshd)
      );
      const { response } = await getSuggestedAccessLists(params, {
        abort: abortSignal,
      });

      return response.accessLists;
    },
  };

  return client;
}
