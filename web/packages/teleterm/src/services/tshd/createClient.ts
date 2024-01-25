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
import * as api from 'gen-proto-js/teleport/lib/teleterm/v1/service_pb';
import { UserPreferences } from 'gen-proto-js/teleport/userpreferences/v1/userpreferences_pb';
import {
  ClusterUserPreferences,
  PinnedResourcesUserPreferences,
} from 'gen-proto-js/teleport/userpreferences/v1/cluster_preferences_pb';
import { UnifiedResourcePreferences } from 'gen-proto-js/teleport/userpreferences/v1/unified_resource_preferences_pb';
import { TerminalServiceClient } from 'gen-proto-js/teleport/lib/teleterm/v1/service_grpc_pb';
import {
  AccessRequest,
  ResourceID,
} from 'gen-proto-js/teleport/lib/teleterm/v1/access_request_pb';

import Logger from 'teleterm/logger';
import * as uri from 'teleterm/ui/uri';

import { createFileTransferStream } from './createFileTransferStream';
import middleware, { withLogging } from './middleware';
import * as types from './types';
import createAbortController from './createAbortController';
import { mapUsageEvent } from './mapUsageEvent';
import {
  ReportUsageEventRequest,
  UpdateHeadlessAuthenticationStateParams,
} from './types';

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
      const req = new api.LogoutRequest().setClusterUri(clusterUri);
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
      const req = new api.GetKubesRequest()
        .setClusterUri(clusterUri)
        .setSearchAsRoles(searchAsRoles)
        .setStartKey(startKey)
        .setSearch(search)
        .setQuery(query)
        .setLimit(limit);

      if (sort) {
        req.setSortBy(`${sort.fieldName}:${sort.dir.toLowerCase()}`);
      }

      return new Promise<types.GetKubesResponse>((resolve, reject) => {
        tshd.getKubes(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject() as types.GetKubesResponse);
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
      const req = new api.GetAppsRequest()
        .setClusterUri(clusterUri)
        .setSearchAsRoles(searchAsRoles)
        .setStartKey(startKey)
        .setSearch(search)
        .setQuery(query)
        .setLimit(limit);

      if (sort) {
        req.setSortBy(`${sort.fieldName}:${sort.dir.toLowerCase()}`);
      }

      return new Promise<types.GetAppsResponse>((resolve, reject) => {
        tshd.getApps(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject() as types.GetAppsResponse);
          }
        });
      });
    },

    async listGateways() {
      const req = new api.ListGatewaysRequest();
      return new Promise<types.Gateway[]>((resolve, reject) => {
        tshd.listGateways(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().gatewaysList as types.Gateway[]);
          }
        });
      });
    },

    async listLeafClusters(clusterUri: uri.RootClusterUri) {
      const req = new api.ListLeafClustersRequest().setClusterUri(clusterUri);
      return new Promise<types.Cluster[]>((resolve, reject) => {
        tshd.listLeafClusters(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().clustersList as types.Cluster[]);
          }
        });
      });
    },

    async listRootClusters(abortSignal?: types.TshAbortSignal) {
      return withAbort(abortSignal, callRef => {
        return new Promise<types.Cluster[]>((resolve, reject) => {
          callRef.current = tshd.listRootClusters(
            new api.ListClustersRequest(),
            (err, response) => {
              if (err) {
                reject(err);
              } else {
                resolve(response.toObject().clustersList as types.Cluster[]);
              }
            }
          );
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
      const req = new api.GetDatabasesRequest()
        .setClusterUri(clusterUri)
        .setSearchAsRoles(searchAsRoles)
        .setStartKey(startKey)
        .setSearch(search)
        .setQuery(query)
        .setLimit(limit);

      if (sort) {
        req.setSortBy(`${sort.fieldName}:${sort.dir.toLowerCase()}`);
      }

      return new Promise<types.GetDatabasesResponse>((resolve, reject) => {
        tshd.getDatabases(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject() as types.GetDatabasesResponse);
          }
        });
      });
    },

    async listDatabaseUsers(dbUri: uri.DatabaseUri) {
      const req = new api.ListDatabaseUsersRequest().setDbUri(dbUri);
      return new Promise<string[]>((resolve, reject) => {
        tshd.listDatabaseUsers(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().usersList);
          }
        });
      });
    },

    async getAccessRequest(clusterUri: uri.RootClusterUri, requestId: string) {
      const req = new api.GetAccessRequestRequest()
        .setClusterUri(clusterUri)
        .setAccessRequestId(requestId);
      return new Promise<types.AccessRequest>((resolve, reject) => {
        tshd.getAccessRequest(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().request);
          }
        });
      });
    },

    async getAccessRequests(clusterUri: uri.RootClusterUri) {
      const req = new api.GetAccessRequestsRequest().setClusterUri(clusterUri);
      return new Promise<types.AccessRequest[]>((resolve, reject) => {
        tshd.getAccessRequests(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().requestsList);
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
      const req = new api.GetServersRequest()
        .setClusterUri(clusterUri)
        .setSearchAsRoles(searchAsRoles)
        .setStartKey(startKey)
        .setSearch(search)
        .setQuery(query)
        .setLimit(limit);

      if (sort) {
        req.setSortBy(`${sort.fieldName}:${sort.dir.toLowerCase()}`);
      }

      return new Promise<types.GetServersResponse>((resolve, reject) => {
        tshd.getServers(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject() as types.GetServersResponse);
          }
        });
      });
    },

    async createAccessRequest(params: types.CreateAccessRequestParams) {
      const req = new api.CreateAccessRequestRequest()
        .setRootClusterUri(params.rootClusterUri)
        .setSuggestedReviewersList(params.suggestedReviewers)
        .setRolesList(params.roles)
        .setResourceIdsList(
          params.resourceIds.map(({ id, clusterName, kind }) => {
            const resourceId = new ResourceID();
            resourceId.setName(id);
            resourceId.setClusterName(clusterName);
            resourceId.setKind(kind);
            return resourceId;
          })
        )
        .setReason(params.reason);
      return new Promise<AccessRequest.AsObject>((resolve, reject) => {
        tshd.createAccessRequest(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().request);
          }
        });
      });
    },

    async deleteAccessRequest(
      clusterUri: uri.RootClusterUri,
      requestId: string
    ) {
      const req = new api.DeleteAccessRequestRequest()
        .setRootClusterUri(clusterUri)
        .setAccessRequestId(requestId);
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
      const req = new api.AssumeRoleRequest()
        .setRootClusterUri(clusterUri)
        .setAccessRequestIdsList(requestIds)
        .setDropRequestIdsList(dropIds);
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
      const req = new api.ReviewAccessRequestRequest()
        .setRootClusterUri(clusterUri)
        .setAccessRequestId(params.id)
        .setState(params.state)
        .setReason(params.reason)
        .setRolesList(params.roles);
      return new Promise<types.AccessRequest>((resolve, reject) => {
        tshd.reviewAccessRequest(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().request);
          }
        });
      });
    },

    async getRequestableRoles(params: types.GetRequestableRolesParams) {
      const req = new api.GetRequestableRolesRequest()
        .setClusterUri(params.rootClusterUri)
        .setResourceIdsList(
          params.resourceIds.map(({ id, clusterName, kind }) => {
            const resourceId = new ResourceID();
            resourceId.setName(id);
            resourceId.setClusterName(clusterName);
            resourceId.setKind(kind);
            return resourceId;
          })
        );
      return new Promise<types.GetRequestableRolesResponse>(
        (resolve, reject) => {
          tshd.getRequestableRoles(req, (err, response) => {
            if (err) {
              reject(err);
            } else {
              resolve(response.toObject());
            }
          });
        }
      );
    },

    async addRootCluster(addr: string) {
      const req = new api.AddClusterRequest().setName(addr);
      return new Promise<types.Cluster>((resolve, reject) => {
        tshd.addCluster(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject() as types.Cluster);
          }
        });
      });
    },

    async getCluster(uri: uri.RootClusterUri) {
      const req = new api.GetClusterRequest().setClusterUri(uri);
      return new Promise<types.Cluster>((resolve, reject) => {
        tshd.getCluster(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject() as types.Cluster);
          }
        });
      });
    },

    async loginLocal(
      params: types.LoginLocalParams,
      abortSignal?: types.TshAbortSignal
    ) {
      const localParams = new api.LoginRequest.LocalParams()
        .setToken(params.token)
        .setUser(params.username)
        .setPassword(params.password);

      return withAbort(abortSignal, callRef => {
        const req = new api.LoginRequest().setClusterUri(params.clusterUri);
        req.setLocal(localParams);

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
      const ssoParams = new api.LoginRequest.SsoParams()
        .setProviderName(params.providerName)
        .setProviderType(params.providerType);

      return withAbort(abortSignal, callRef => {
        const req = new api.LoginRequest().setClusterUri(params.clusterUri);
        req.setSso(ssoParams);

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
          new api.LoginPasswordlessRequest.LoginPasswordlessRequestInit().setClusterUri(
            params.clusterUri
          );
        const streamReq = new api.LoginPasswordlessRequest().setInit(
          streamInitReq
        );

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
            const req = response.toObject();

            switch (req.prompt) {
              case api.PasswordlessPrompt.PASSWORDLESS_PROMPT_PIN:
                const pinResponse = pin => {
                  const pinRes =
                    new api.LoginPasswordlessRequest.LoginPasswordlessPINResponse().setPin(
                      pin
                    );
                  stream.write(
                    new api.LoginPasswordlessRequest().setPin(pinRes)
                  );
                };

                params.onPromptCallback({
                  type: 'pin',
                  onUserResponse: pinResponse,
                });
                return;

              case api.PasswordlessPrompt.PASSWORDLESS_PROMPT_CREDENTIAL:
                const credResponse = index => {
                  const credRes =
                    new api.LoginPasswordlessRequest.LoginPasswordlessCredentialResponse().setIndex(
                      index
                    );
                  stream.write(
                    new api.LoginPasswordlessRequest().setCredential(credRes)
                  );
                };

                params.onPromptCallback({
                  type: 'credential',
                  onUserResponse: credResponse,
                  data: { credentials: req.credentialsList || [] },
                });
                return;

              case api.PasswordlessPrompt.PASSWORDLESS_PROMPT_TAP:
                if (hasDeviceBeenTapped) {
                  params.onPromptCallback({ type: 'retap' });
                } else {
                  hasDeviceBeenTapped = true;
                  params.onPromptCallback({ type: 'tap' });
                }
                return;

              // Following cases should never happen but just in case?
              case api.PasswordlessPrompt.PASSWORDLESS_PROMPT_UNSPECIFIED:
                stream.cancel();
                return reject(
                  new Error('no passwordless prompt was specified')
                );

              default:
                stream.cancel();
                return reject(
                  new Error(`passwordless prompt '${req.prompt}' not supported`)
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
      const req = new api.GetAuthSettingsRequest().setClusterUri(clusterUri);
      return new Promise<types.AuthSettings>((resolve, reject) => {
        tshd.getAuthSettings(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject());
          }
        });
      });
    },

    async createGateway(params: types.CreateGatewayParams) {
      const req = new api.CreateGatewayRequest()
        .setTargetUri(params.targetUri)
        .setTargetUser(params.user)
        .setLocalPort(params.port)
        .setTargetSubresourceName(params.subresource_name);
      return new Promise<types.Gateway>((resolve, reject) => {
        tshd.createGateway(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject() as types.Gateway);
          }
        });
      });
    },

    async removeCluster(clusterUri: uri.RootClusterUri) {
      const req = new api.RemoveClusterRequest().setClusterUri(clusterUri);
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
      const req = new api.RemoveGatewayRequest().setGatewayUri(gatewayUri);
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
      const req = new api.SetGatewayTargetSubresourceNameRequest()
        .setGatewayUri(gatewayUri)
        .setTargetSubresourceName(targetSubresourceName);
      return new Promise<types.Gateway>((resolve, reject) => {
        tshd.setGatewayTargetSubresourceName(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject() as types.Gateway);
          }
        });
      });
    },

    async setGatewayLocalPort(gatewayUri: uri.GatewayUri, localPort: string) {
      const req = new api.SetGatewayLocalPortRequest()
        .setGatewayUri(gatewayUri)
        .setLocalPort(localPort);
      return new Promise<types.Gateway>((resolve, reject) => {
        tshd.setGatewayLocalPort(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject() as types.Gateway);
          }
        });
      });
    },

    transferFile(
      options: types.FileTransferRequest,
      abortSignal: types.TshAbortSignal
    ) {
      const req = new api.FileTransferRequest()
        .setServerUri(options.serverUri)
        .setLogin(options.login)
        .setSource(options.source)
        .setDestination(options.destination)
        .setDirection(options.direction);

      return createFileTransferStream(tshd.transferFile(req), abortSignal);
    },

    updateTshdEventsServerAddress(address: string) {
      const req = new api.UpdateTshdEventsServerAddressRequest().setAddress(
        address
      );
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
      const req =
        new api.CreateConnectMyComputerRoleRequest().setRootClusterUri(
          rootClusterUri
        );

      return new Promise<types.CreateConnectMyComputerRoleResponse>(
        (resolve, reject) => {
          tshd.createConnectMyComputerRole(req, (err, response) => {
            if (err) {
              reject(err);
            } else {
              resolve(response.toObject());
            }
          });
        }
      );
    },

    createConnectMyComputerNodeToken(uri: uri.RootClusterUri) {
      return new Promise<types.CreateConnectMyComputerNodeTokenResponse>(
        (resolve, reject) => {
          tshd.createConnectMyComputerNodeToken(
            new api.CreateConnectMyComputerNodeTokenRequest().setRootClusterUri(
              uri
            ),
            (err, response) => {
              if (err) {
                reject(err);
              } else {
                resolve(response.toObject());
              }
            }
          );
        }
      );
    },

    deleteConnectMyComputerToken(uri: uri.RootClusterUri, token: string) {
      return new Promise<void>((resolve, reject) => {
        tshd.deleteConnectMyComputerToken(
          new api.DeleteConnectMyComputerTokenRequest()
            .setRootClusterUri(uri)
            .setToken(token),
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
      const req =
        new api.WaitForConnectMyComputerNodeJoinRequest().setRootClusterUri(
          uri
        );

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
                      response.toObject() as types.WaitForConnectMyComputerNodeJoinResponse
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
          new api.DeleteConnectMyComputerNodeRequest().setRootClusterUri(uri),
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
          new api.GetConnectMyComputerNodeNameRequest().setRootClusterUri(uri),
          (err, response) => {
            if (err) {
              reject(err);
            } else {
              resolve(response.getName() as uri.ServerUri);
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
        const req = new api.UpdateHeadlessAuthenticationStateRequest()
          .setRootClusterUri(params.rootClusterUri)
          .setHeadlessAuthenticationId(params.headlessAuthenticationId)
          .setState(params.state);

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
        const req = new api.ListUnifiedResourcesRequest()
          .setClusterUri(params.clusterUri)
          .setLimit(params.limit)
          .setKindsList(params.kindsList)
          .setStartKey(params.startKey)
          .setSearch(params.search)
          .setQuery(params.query)
          .setPinnedOnly(params.pinnedOnly)
          .setSearchAsRoles(params.searchAsRoles);
        if (params.sortBy) {
          req.setSortBy(
            new api.SortBy()
              .setField(params.sortBy.field)
              .setIsDesc(params.sortBy.isDesc)
          );
        }

        return new Promise<types.ListUnifiedResourcesResponse>(
          (resolve, reject) => {
            callRef.current = tshd.listUnifiedResources(req, (err, res) => {
              if (err) {
                reject(err);
              } else {
                resolve({
                  nextKey: res.getNextKey(),
                  resources: res
                    .getResourcesList()
                    .map(p => {
                      switch (p.getResourceCase()) {
                        case api.PaginatedResource.ResourceCase.SERVER:
                          return {
                            kind: 'server' as const,
                            resource: p.getServer().toObject() as types.Server,
                          };
                        case api.PaginatedResource.ResourceCase.DATABASE:
                          return {
                            kind: 'database' as const,
                            resource: p
                              .getDatabase()
                              .toObject() as types.Database,
                          };
                        case api.PaginatedResource.ResourceCase.KUBE:
                          return {
                            kind: 'kube' as const,
                            resource: p.getKube().toObject() as types.Kube,
                          };
                        case api.PaginatedResource.ResourceCase.APP:
                          return {
                            kind: 'app' as const,
                            resource: p.getApp().toObject() as types.App,
                          };
                        default:
                          logger.info(
                            `Ignoring unsupported resource ${JSON.stringify(
                              p.toObject()
                            )}.`
                          );
                      }
                    })
                    .filter(Boolean),
                });
              }
            });
          }
        );
      });
    },
    getUserPreferences(
      params: api.GetUserPreferencesRequest.AsObject,
      abortSignal?: types.TshAbortSignal
    ): Promise<api.UserPreferences.AsObject> {
      return withAbort(abortSignal, callRef => {
        const req = new api.GetUserPreferencesRequest().setClusterUri(
          params.clusterUri
        );

        return new Promise((resolve, reject) => {
          callRef.current = tshd.getUserPreferences(req, (err, response) => {
            if (err) {
              reject(err);
            } else {
              const res = response.toObject();
              resolve(res.userPreferences);
            }
          });
        });
      });
    },
    updateUserPreferences(
      params: api.UpdateUserPreferencesRequest.AsObject,
      abortSignal?: types.TshAbortSignal
    ): Promise<api.UserPreferences.AsObject> {
      const userPreferences = new UserPreferences();
      if (params.userPreferences.clusterPreferences) {
        userPreferences.setClusterPreferences(
          new ClusterUserPreferences().setPinnedResources(
            new PinnedResourcesUserPreferences().setResourceIdsList(
              params.userPreferences.clusterPreferences.pinnedResources
                .resourceIdsList
            )
          )
        );
      }

      if (params.userPreferences.unifiedResourcePreferences) {
        userPreferences.setUnifiedResourcePreferences(
          new UnifiedResourcePreferences()
            .setDefaultTab(
              params.userPreferences.unifiedResourcePreferences.defaultTab
            )
            .setViewMode(
              params.userPreferences.unifiedResourcePreferences.viewMode
            )
            .setLabelsViewMode(
              params.userPreferences.unifiedResourcePreferences.labelsViewMode
            )
        );
      }

      return withAbort(abortSignal, callRef => {
        const req = new api.UpdateUserPreferencesRequest()
          .setClusterUri(params.clusterUri)
          .setUserPreferences(userPreferences);

        return new Promise((resolve, reject) => {
          callRef.current = tshd.updateUserPreferences(req, (err, response) => {
            if (err) {
              reject(err);
            } else {
              const res = response.toObject();
              resolve(res.userPreferences);
            }
          });
        });
      });
    },
    promoteAccessRequest(
      params: api.PromoteAccessRequestRequest.AsObject,
      abortSignal?: types.TshAbortSignal
    ): Promise<types.AccessRequest> {
      return withAbort(abortSignal, callRef => {
        const req = new api.PromoteAccessRequestRequest()
          .setRootClusterUri(params.rootClusterUri)
          .setAccessRequestId(params.accessRequestId)
          .setAccessListId(params.accessListId)
          .setReason(params.reason);

        return new Promise((resolve, reject) => {
          callRef.current = tshd.promoteAccessRequest(req, (err, response) => {
            if (err) {
              reject(err);
            } else {
              const res = response.toObject();
              resolve(res.request);
            }
          });
        });
      });
    },
    getSuggestedAccessLists(
      params: api.GetSuggestedAccessListsRequest.AsObject,
      abortSignal?: types.TshAbortSignal
    ): Promise<types.AccessList[]> {
      return withAbort(abortSignal, callRef => {
        const req = new api.GetSuggestedAccessListsRequest()
          .setRootClusterUri(params.rootClusterUri)
          .setAccessRequestId(params.accessRequestId);

        return new Promise((resolve, reject) => {
          callRef.current = tshd.getSuggestedAccessLists(
            req,
            (err, response) => {
              if (err) {
                reject(err);
              } else {
                const res = response.toObject();
                resolve(res.accessListsList);
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
  current?: {
    cancel(): void;
  };
};

async function withAbort<T>(
  sig: types.TshAbortSignal,
  cb: (ref: CallRef) => Promise<T>
) {
  const ref = {
    current: null,
  };

  const abort = () => {
    ref?.current.cancel();
  };

  sig?.addEventListener(abort);

  return cb(ref).finally(() => {
    sig?.removeEventListener(abort);
  });
}
