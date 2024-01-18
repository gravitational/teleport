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
import * as api from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';
import { TerminalServiceClient } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb.grpc-client';
import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';
import {
  ClusterUserPreferences,
  PinnedResourcesUserPreferences,
} from 'gen-proto-ts/teleport/userpreferences/v1/cluster_preferences_pb';
import { UnifiedResourcePreferences } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import {
  AccessRequest,
  ResourceID,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';

import Logger from 'teleterm/logger';
import * as uri from 'teleterm/ui/uri';

import {
  resourceOneOfIsApp,
  resourceOneOfIsDatabase,
  resourceOneOfIsKube,
  resourceOneOfIsServer,
} from 'teleterm/helpers';

import { createFileTransferStream } from './createFileTransferStream';
import middleware, { withLogging } from './middleware';
import * as types from './types';
import {
  ReportUsageEventRequest,
  UnifiedResourceResponse,
  UpdateHeadlessAuthenticationStateParams,
} from './types';
import createAbortController from './createAbortController';
import { mapUsageEvent } from './mapUsageEvent';

export function createTshdClient(
  addr: string,
  credentials: grpc.ChannelCredentials
): types.TshdClient {
  const logger = new Logger('tshd');
  const tshd = middleware(new TerminalServiceClient(addr, credentials), [
    withLogging(logger),
  ]);

  // Create a client instance that could be shared with the  renderer (UI) via Electron contextBridge
  const client = {
    createAbortController,

    async logout(clusterUri: uri.RootClusterUri) {
      const req = api.LogoutRequest.create({ clusterUri });
      return new Promise<void>((resolve, reject) => {
        tshd.logout(req, err => {
          if (err) {
            reject(err);
          } else {
            resolve();
          }
        });
      });
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
      const req = api.GetKubesRequest.create({
        clusterUri,
        searchAsRoles,
        startKey,
        search,
        query,
        limit,
      });

      if (sort) {
        req.sortBy = `${sort.fieldName}:${sort.dir.toLowerCase()}`;
      }

      return new Promise<types.GetKubesResponse>((resolve, reject) => {
        tshd.getKubes(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response as types.GetKubesResponse);
          }
        });
      });
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
      const req = api.GetAppsRequest.create({
        clusterUri,
        searchAsRoles,
        startKey,
        search,
        query,
        limit,
      });

      if (sort) {
        req.sortBy = `${sort.fieldName}:${sort.dir.toLowerCase()}`;
      }

      return new Promise<types.GetAppsResponse>((resolve, reject) => {
        tshd.getApps(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response as types.GetAppsResponse);
          }
        });
      });
    },

    async listGateways() {
      const req = api.ListGatewaysRequest.create();
      return new Promise<types.Gateway[]>((resolve, reject) => {
        tshd.listGateways(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response!.gateways as types.Gateway[]);
          }
        });
      });
    },

    async listLeafClusters(clusterUri: uri.RootClusterUri) {
      const req = api.ListLeafClustersRequest.create({ clusterUri });
      return new Promise<types.Cluster[]>((resolve, reject) => {
        tshd.listLeafClusters(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response!.clusters as types.Cluster[]);
          }
        });
      });
    },

    async listRootClusters() {
      const req = api.ListClustersRequest.create();
      return new Promise<types.Cluster[]>((resolve, reject) => {
        tshd.listRootClusters(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response!.clusters as types.Cluster[]);
          }
        });
      });
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
      const req = api.GetDatabasesRequest.create({
        clusterUri,
        searchAsRoles,
        startKey,
        search,
        query,
        limit,
      });

      if (sort) {
        req.sortBy = `${sort.fieldName}:${sort.dir.toLowerCase()}`;
      }

      return new Promise<types.GetDatabasesResponse>((resolve, reject) => {
        tshd.getDatabases(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response as types.GetDatabasesResponse);
          }
        });
      });
    },

    async listDatabaseUsers(dbUri: uri.DatabaseUri) {
      const req = api.ListDatabaseUsersRequest.create({ dbUri });
      return new Promise<string[]>((resolve, reject) => {
        tshd.listDatabaseUsers(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response!.users!);
          }
        });
      });
    },

    async getAccessRequest(clusterUri: uri.RootClusterUri, requestId: string) {
      const req = api.GetAccessRequestRequest.create({
        clusterUri,
        accessRequestId: requestId,
      });
      return new Promise<types.AccessRequest>((resolve, reject) => {
        tshd.getAccessRequest(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response!.request!);
          }
        });
      });
    },

    async getAccessRequests(clusterUri: uri.RootClusterUri) {
      const req = api.GetAccessRequestsRequest.create({ clusterUri });
      return new Promise<types.AccessRequest[]>((resolve, reject) => {
        tshd.getAccessRequests(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response!.requests!);
          }
        });
      });
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
      const req = api.GetServersRequest.create({
        clusterUri,
        searchAsRoles,
        startKey,
        search,
        query,
        limit,
      });

      if (sort) {
        req.sortBy = `${sort.fieldName}:${sort.dir.toLowerCase()}`;
      }

      return new Promise<types.GetServersResponse>((resolve, reject) => {
        tshd.getServers(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response as types.GetServersResponse);
          }
        });
      });
    },

    async createAccessRequest(params: types.CreateAccessRequestParams) {
      const req = api.CreateAccessRequestRequest.create({
        rootClusterUri: params.rootClusterUri,
        roles: params.roles,
        resourceIds: params.resourceIds.map(({ id, clusterName, kind }) =>
          ResourceID.create({
            name: id,
            clusterName,
            kind,
          })
        ),
        reason: params.reason,
      });
      // .setRootClusterUri(params.rootClusterUri)
      // .setSuggestedReviewersList(params.suggestedReviewers)
      // .setRolesList(params.roles)
      // .setResourceIdsList(
      //   params.resourceIds.map(({ id, clusterName, kind }) => {
      //     const resourceId = new ResourceID();
      //     resourceId.setName(id);
      //     resourceId.setClusterName(clusterName);
      //     resourceId.setKind(kind);
      //     return resourceId;
      //   })
      // )
      // .setReason(params.reason);
      return new Promise<AccessRequest>((resolve, reject) => {
        tshd.createAccessRequest(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response!.request!);
          }
        });
      });
    },

    async deleteAccessRequest(
      clusterUri: uri.RootClusterUri,
      requestId: string
    ) {
      const req = api.DeleteAccessRequestRequest.create({
        rootClusterUri: clusterUri,
        accessRequestId: requestId,
      });
      return new Promise<void>((resolve, reject) => {
        tshd.deleteAccessRequest(req, err => {
          if (err) {
            reject(err);
          } else {
            resolve();
          }
        });
      });
    },

    async assumeRole(
      clusterUri: uri.RootClusterUri,
      requestIds: string[],
      dropIds: string[]
    ) {
      const req = api.AssumeRoleRequest.create({
        rootClusterUri: clusterUri,
        accessRequestIds: requestIds,
        dropRequestIds: dropIds,
      });
      return new Promise<void>((resolve, reject) => {
        tshd.assumeRole(req, err => {
          if (err) {
            reject(err);
          } else {
            resolve();
          }
        });
      });
    },

    async reviewAccessRequest(
      clusterUri: uri.RootClusterUri,
      params: types.ReviewAccessRequestParams
    ) {
      const req = api.ReviewAccessRequestRequest.create({
        rootClusterUri: clusterUri,
        accessRequestId: params.id,
        state: params.state,
        reason: params.reason,
        roles: params.roles,
      });
      return new Promise<types.AccessRequest>((resolve, reject) => {
        tshd.reviewAccessRequest(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response!.request!);
          }
        });
      });
    },

    async getRequestableRoles(params: types.GetRequestableRolesParams) {
      const req = api.GetRequestableRolesRequest.create({
        clusterUri: params.rootClusterUri,
        resourceIds: params.resourceIds!.map(({ id, clusterName, kind }) =>
          ResourceID.create({
            name: id,
            clusterName,
            kind,
          })
        ),
      });
      return new Promise<types.GetRequestableRolesResponse>(
        (resolve, reject) => {
          tshd.getRequestableRoles(req, (err, response) => {
            if (err) {
              reject(err);
            } else {
              resolve(response!);
            }
          });
        }
      );
    },

    async addRootCluster(addr: string) {
      const req = api.AddClusterRequest.create({ name: addr });
      return new Promise<types.Cluster>((resolve, reject) => {
        tshd.addCluster(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response as types.Cluster);
          }
        });
      });
    },

    async getCluster(uri: uri.RootClusterUri) {
      const req = api.GetClusterRequest.create({ clusterUri: uri });
      return new Promise<types.Cluster>((resolve, reject) => {
        tshd.getCluster(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response as types.Cluster);
          }
        });
      });
    },

    async loginLocal(
      params: types.LoginLocalParams,
      abortSignal?: types.TshAbortSignal
    ) {
      const localParams = api.LoginRequest_LocalParams.create({
        token: params.token,
        user: params.username,
        password: params.password,
      });
      return withAbort(abortSignal, callRef => {
        const req = api.LoginRequest.create({
          clusterUri: params.clusterUri,
          params: {
            oneofKind: 'local',
            local: localParams,
          },
        });
        return new Promise<void>((resolve, reject) => {
          callRef.current = tshd.login(req, err => {
            if (err) {
              reject(err);
            } else {
              resolve();
            }
          });
        });
      });
    },

    async loginSso(
      params: types.LoginSsoParams,
      abortSignal?: types.TshAbortSignal
    ) {
      const ssoParams = api.LoginRequest_SsoParams.create({
        providerName: params.providerName,
        providerType: params.providerType,
      });
      return withAbort(abortSignal, callRef => {
        const req = api.LoginRequest.create({
          clusterUri: params.clusterUri,
          params: {
            oneofKind: 'sso',
            sso: ssoParams,
          },
        });
        return new Promise<void>((resolve, reject) => {
          callRef.current = tshd.login(req, err => {
            if (err) {
              reject(err);
            } else {
              resolve();
            }
          });
        });
      });
    },

    async loginPasswordless(
      params: types.LoginPasswordlessParams,
      abortSignal?: types.TshAbortSignal
    ) {
      return withAbort(abortSignal, callRef => {
        const streamInitReq =
          api.LoginPasswordlessRequest_LoginPasswordlessRequestInit.create({
            clusterUri: params.clusterUri,
          });
        const streamReq = api.LoginPasswordlessRequest.create({
          request: {
            oneofKind: 'init',
            init: streamInitReq,
          },
        });
        return new Promise<void>((resolve, reject) => {
          callRef.current = tshd.loginPasswordless();
          const stream = callRef.current as grpc.ClientDuplexStream<
            api.LoginPasswordlessRequest,
            api.LoginPasswordlessResponse
          >;

          let hasDeviceBeenTapped = false;

          // Init the stream.
          stream.write(streamReq);

          stream.on('data', function (response: api.LoginPasswordlessResponse) {
            switch (response.prompt) {
              case api.PasswordlessPrompt.PIN:
                const pinResponse = pin => {
                  const pinRes =
                    api.LoginPasswordlessRequest_LoginPasswordlessPINResponse.create(
                      { pin }
                    );
                  stream.write(
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
                  const credRes =
                    api.LoginPasswordlessRequest_LoginPasswordlessCredentialResponse.create(
                      { index }
                    );
                  stream.write(
                    api.LoginPasswordlessRequest.create({
                      request: {
                        oneofKind: 'credential',
                        credential: credRes,
                      },
                    })
                  );
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
                stream.cancel();
                return reject(
                  new Error('no passwordless prompt was specified')
                );

              default:
                stream.cancel();
                return reject(
                  new Error(
                    `passwordless prompt '${response.prompt}' not supported`
                  )
                );
            }
          });

          stream.on('end', function () {
            resolve();
          });

          stream.on('error', function (err: Error) {
            reject(err);
          });
        });
      });
    },

    async getAuthSettings(clusterUri: uri.RootClusterUri) {
      const req = api.GetAuthSettingsRequest.create({ clusterUri });
      return new Promise<types.AuthSettings>((resolve, reject) => {
        tshd.getAuthSettings(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response!);
          }
        });
      });
    },

    async createGateway(params: types.CreateGatewayParams) {
      const req = api.CreateGatewayRequest.create({
        targetUri: params.targetUri,
        targetUser: params.user,
        localPort: params.port,
        targetSubresourceName: params.subresource_name,
      });
      return new Promise<types.Gateway>((resolve, reject) => {
        tshd.createGateway(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response as types.Gateway);
          }
        });
      });
    },

    async removeCluster(clusterUri: uri.RootClusterUri) {
      const req = api.RemoveClusterRequest.create({ clusterUri });
      return new Promise<void>((resolve, reject) => {
        tshd.removeCluster(req, err => {
          if (err) {
            reject(err);
          } else {
            resolve();
          }
        });
      });
    },

    async removeGateway(gatewayUri: uri.GatewayUri) {
      const req = api.RemoveGatewayRequest.create({ gatewayUri });
      return new Promise<void>((resolve, reject) => {
        tshd.removeGateway(req, err => {
          if (err) {
            reject(err);
          } else {
            resolve();
          }
        });
      });
    },

    async setGatewayTargetSubresourceName(
      gatewayUri: uri.GatewayUri,
      targetSubresourceName = ''
    ) {
      const req = api.SetGatewayTargetSubresourceNameRequest.create({
        gatewayUri,
        targetSubresourceName,
      });
      return new Promise<types.Gateway>((resolve, reject) => {
        tshd.setGatewayTargetSubresourceName(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response as types.Gateway);
          }
        });
      });
    },

    async setGatewayLocalPort(gatewayUri: uri.GatewayUri, localPort: string) {
      const req = api.SetGatewayLocalPortRequest.create({
        gatewayUri,
        localPort,
      });
      return new Promise<types.Gateway>((resolve, reject) => {
        tshd.setGatewayLocalPort(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response as types.Gateway);
          }
        });
      });
    },

    transferFile(
      options: types.FileTransferRequest,
      abortSignal: types.TshAbortSignal
    ) {
      const req = api.FileTransferRequest.create({
        serverUri: options.serverUri,
        login: options.login,
        source: options.source,
        destination: options.destination,
        direction: options.direction,
      });

      return createFileTransferStream(tshd.transferFile(req), abortSignal);
    },

    updateTshdEventsServerAddress(address: string) {
      const req = api.UpdateTshdEventsServerAddressRequest.create({ address });
      return new Promise<void>((resolve, reject) => {
        tshd.updateTshdEventsServerAddress(req, err => {
          if (err) {
            reject(err);
          } else {
            resolve();
          }
        });
      });
    },

    reportUsageEvent(event: ReportUsageEventRequest) {
      const req = mapUsageEvent(event);
      return new Promise<void>((resolve, reject) => {
        tshd.reportUsageEvent(req, err => {
          if (err) {
            reject(err);
          } else {
            resolve();
          }
        });
      });
    },

    createConnectMyComputerRole(rootClusterUri: uri.RootClusterUri) {
      const req = api.CreateConnectMyComputerRoleRequest.create({
        rootClusterUri,
      });

      return new Promise<types.CreateConnectMyComputerRoleResponse>(
        (resolve, reject) => {
          tshd.createConnectMyComputerRole(req, (err, response) => {
            if (err) {
              reject(err);
            } else {
              resolve(response!);
            }
          });
        }
      );
    },

    createConnectMyComputerNodeToken(uri: uri.RootClusterUri) {
      return new Promise<types.CreateConnectMyComputerNodeTokenResponse>(
        (resolve, reject) => {
          tshd.createConnectMyComputerNodeToken(
            api.CreateConnectMyComputerNodeTokenRequest.create({
              rootClusterUri: uri,
            }),
            (err, response) => {
              if (err) {
                reject(err);
              } else {
                resolve(response!);
              }
            }
          );
        }
      );
    },

    deleteConnectMyComputerToken(uri: uri.RootClusterUri, token: string) {
      return new Promise<void>((resolve, reject) => {
        tshd.deleteConnectMyComputerToken(
          api.DeleteConnectMyComputerTokenRequest.create({
            rootClusterUri: uri,
            token,
          }),
          err => {
            if (err) {
              reject(err);
            } else {
              resolve();
            }
          }
        );
      });
    },

    waitForConnectMyComputerNodeJoin(
      uri: uri.RootClusterUri,
      abortSignal: types.TshAbortSignal
    ) {
      const req = api.WaitForConnectMyComputerNodeJoinRequest.create({
        rootClusterUri: uri,
      });

      return withAbort(
        abortSignal,
        callRef =>
          new Promise<types.WaitForConnectMyComputerNodeJoinResponse>(
            (resolve, reject) => {
              callRef.current = tshd.waitForConnectMyComputerNodeJoin(
                req,
                (err, response) => {
                  if (err) {
                    reject(err);
                  } else {
                    resolve(
                      response as types.WaitForConnectMyComputerNodeJoinResponse
                    );
                  }
                }
              );
            }
          )
      );
    },

    deleteConnectMyComputerNode(uri: uri.RootClusterUri) {
      return new Promise<void>((resolve, reject) => {
        tshd.deleteConnectMyComputerNode(
          api.DeleteConnectMyComputerNodeRequest.create({
            rootClusterUri: uri,
          }),
          err => {
            if (err) {
              reject(err);
            } else {
              resolve();
            }
          }
        );
      });
    },

    getConnectMyComputerNodeName(uri: uri.RootClusterUri) {
      return new Promise<string>((resolve, reject) => {
        tshd.getConnectMyComputerNodeName(
          api.GetConnectMyComputerNodeNameRequest.create({
            rootClusterUri: uri,
          }),
          (err, response) => {
            if (err) {
              reject(err);
            } else {
              resolve(response!.name as uri.ServerUri);
            }
          }
        );
      });
    },

    updateHeadlessAuthenticationState(
      params: UpdateHeadlessAuthenticationStateParams,
      abortSignal?: types.TshAbortSignal
    ) {
      return withAbort(abortSignal, callRef => {
        const req = api.UpdateHeadlessAuthenticationStateRequest.create({
          rootClusterUri: params.rootClusterUri,
          headlessAuthenticationId: params.headlessAuthenticationId,
          state: params.state,
        });

        return new Promise<void>((resolve, reject) => {
          callRef.current = tshd.updateHeadlessAuthenticationState(req, err => {
            if (err) {
              reject(err);
            } else {
              resolve();
            }
          });
        });
      });
    },

    listUnifiedResources(
      params: types.ListUnifiedResourcesRequest,
      abortSignal?: types.TshAbortSignal
    ) {
      return withAbort(abortSignal, callRef => {
        const req = api.ListUnifiedResourcesRequest.create({
          clusterUri: params.clusterUri,
          limit: params.limit,
          kinds: params.kinds,
          startKey: params.startKey,
          search: params.search,
          query: params.query,
          pinnedOnly: params.pinnedOnly,
          searchAsRoles: params.searchAsRoles,
        });
        if (params.sortBy) {
          req.sortBy = api.SortBy.create({
            field: params.sortBy.field,
            isDesc: params.sortBy.isDesc,
          });
        }

        return new Promise<types.ListUnifiedResourcesResponse>(
          (resolve, reject) => {
            callRef.current = tshd.listUnifiedResources(req, (err, res) => {
              if (err) {
                reject(err);
              } else {
                resolve({
                  nextKey: res!.nextKey,
                  resources: res!.resources
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

                      logger.info(
                        `Ignoring unsupported resource ${JSON.stringify(p)}.`
                      );
                    })
                    .filter(Boolean) as UnifiedResourceResponse[],
                });
              }
            });
          }
        );
      });
    },
    getUserPreferences(
      params: api.GetUserPreferencesRequest,
      abortSignal?: types.TshAbortSignal
    ): Promise<api.UserPreferences> {
      return withAbort(abortSignal, callRef => {
        const req = api.GetUserPreferencesRequest.create({
          clusterUri: params.clusterUri,
        });

        return new Promise((resolve, reject) => {
          callRef.current = tshd.getUserPreferences(req, (err, response) => {
            if (err) {
              reject(err);
            } else {
              resolve(response!.userPreferences!);
            }
          });
        });
      });
    },
    updateUserPreferences(
      params: api.UpdateUserPreferencesRequest,
      abortSignal?: types.TshAbortSignal
    ): Promise<api.UserPreferences> {
      const userPreferences = UserPreferences.create();
      if (params.userPreferences) {
        if (params.userPreferences.clusterPreferences) {
          userPreferences.clusterPreferences = ClusterUserPreferences.create({
            pinnedResources: PinnedResourcesUserPreferences.create({
              resourceIds:
                params.userPreferences.clusterPreferences.pinnedResources
                  ?.resourceIds,
            }),
          });
        }

        if (params.userPreferences.unifiedResourcePreferences) {
          userPreferences.unifiedResourcePreferences =
            UnifiedResourcePreferences.create({
              defaultTab:
                params.userPreferences.unifiedResourcePreferences.defaultTab,
              viewMode:
                params.userPreferences.unifiedResourcePreferences.viewMode,
              labelsViewMode:
                params.userPreferences.unifiedResourcePreferences
                  .labelsViewMode,
            });
        }
      }

      return withAbort(abortSignal, callRef => {
        const req = api.UpdateUserPreferencesRequest.create({
          clusterUri: params.clusterUri,
          userPreferences: {
            unifiedResourcePreferences:
              userPreferences.unifiedResourcePreferences,
            clusterPreferences: userPreferences.clusterPreferences,
          },
        });

        return new Promise((resolve, reject) => {
          callRef.current = tshd.updateUserPreferences(req, (err, response) => {
            if (err) {
              reject(err);
            } else {
              resolve(response!.userPreferences!);
            }
          });
        });
      });
    },
    promoteAccessRequest(
      params: api.PromoteAccessRequestRequest,
      abortSignal?: types.TshAbortSignal
    ): Promise<types.AccessRequest> {
      return withAbort(abortSignal, callRef => {
        const req = api.PromoteAccessRequestRequest.create({
          rootClusterUri: params.rootClusterUri,
          accessRequestId: params.accessRequestId,
          accessListId: params.accessListId,
          reason: params.reason,
        });

        return new Promise((resolve, reject) => {
          callRef.current = tshd.promoteAccessRequest(req, (err, response) => {
            if (err) {
              reject(err);
            } else {
              resolve(response!.request!);
            }
          });
        });
      });
    },
    getSuggestedAccessLists(
      params: api.GetSuggestedAccessListsRequest,
      abortSignal?: types.TshAbortSignal
    ): Promise<types.AccessList[]> {
      return withAbort(abortSignal, callRef => {
        const req = api.GetSuggestedAccessListsRequest.create({
          rootClusterUri: params.rootClusterUri,
          accessRequestId: params.accessRequestId,
        });

        return new Promise((resolve, reject) => {
          callRef.current = tshd.getSuggestedAccessLists(
            req,
            (err, response) => {
              if (err) {
                reject(err);
              } else {
                resolve(response!.accessLists!);
              }
            }
          );
        });
      });
    },
  };

  return client;
}

type CallRef = {
  current: {
    cancel(): void;
  } | null;
};

async function withAbort<T>(
  sig: types.TshAbortSignal | undefined,
  cb: (ref: CallRef) => Promise<T>
) {
  const ref: CallRef = {
    current: null,
  };

  const abort = () => {
    ref?.current?.cancel();
  };

  sig?.addEventListener(abort);

  return cb(ref).finally(() => {
    sig?.removeEventListener(abort);
  });
}
