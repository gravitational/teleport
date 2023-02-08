/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { ChannelCredentials, ClientDuplexStream } from '@grpc/grpc-js';

import { TerminalServiceClient } from 'teleterm/services/tshd/v1/service_grpc_pb';
import * as api from 'teleterm/services/tshd/v1/service_pb';
import * as types from 'teleterm/services/tshd/types';
import Logger from 'teleterm/logger';

import middleware, { withLogging } from './middleware';
import createAbortController from './createAbortController';

export default function createClient(
  addr: string,
  credentials: ChannelCredentials
) {
  const logger = new Logger('tshd');
  const tshd = middleware(new TerminalServiceClient(addr, credentials), [
    withLogging(logger),
  ]);

  // Create a client instance that could be shared with the  renderer (UI) via Electron contextBridge
  const client = {
    createAbortController,

    async logout(clusterUri: string) {
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

    async listApps(clusterUri: string) {
      const req = new api.ListAppsRequest().setClusterUri(clusterUri);
      return new Promise<types.Application[]>((resolve, reject) => {
        tshd.listApps(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().appsList);
          }
        });
      });
    },

    async listKubes(clusterUri: string) {
      const req = new api.ListKubesRequest().setClusterUri(clusterUri);
      return new Promise<types.Kube[]>((resolve, reject) => {
        tshd.listKubes(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().kubesList);
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
            resolve(response.toObject().gatewaysList);
          }
        });
      });
    },

    async listLeafClusters(clusterUri: string) {
      const req = new api.ListLeafClustersRequest().setClusterUri(clusterUri);
      return new Promise<types.Cluster[]>((resolve, reject) => {
        tshd.listLeafClusters(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().clustersList);
          }
        });
      });
    },

    async listRootClusters() {
      const req = new api.ListClustersRequest();
      return new Promise<types.Cluster[]>((resolve, reject) => {
        tshd.listRootClusters(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().clustersList);
          }
        });
      });
    },

    async listDatabases(clusterUri: string) {
      const req = new api.ListDatabasesRequest().setClusterUri(clusterUri);
      return new Promise<types.Database[]>((resolve, reject) => {
        tshd.listDatabases(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().databasesList);
          }
        });
      });
    },

    async listDatabaseUsers(dbUri: string) {
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

    async listServers(clusterUri: string) {
      const req = new api.ListServersRequest().setClusterUri(clusterUri);
      return new Promise<types.Server[]>((resolve, reject) => {
        tshd.listServers(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject().serversList);
          }
        });
      });
    },

    async addRootCluster(addr: string) {
      const req = new api.AddClusterRequest().setName(addr);
      return new Promise<types.Cluster>((resolve, reject) => {
        tshd.addCluster(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject());
          }
        });
      });
    },

    async getCluster(uri: string) {
      const req = new api.GetClusterRequest().setClusterUri(uri);
      return new Promise<types.Cluster>((resolve, reject) => {
        tshd.getCluster(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject());
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
          const stream = callRef.current as ClientDuplexStream<
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

    async getAuthSettings(clusterUri = '') {
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
            resolve(response.toObject());
          }
        });
      });
    },

    async removeCluster(clusterUri = '') {
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

    async removeGateway(gatewayUri = '') {
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

    async restartGateway(gatewayUri = '') {
      const req = new api.RestartGatewayRequest().setGatewayUri(gatewayUri);
      return new Promise<void>((resolve, reject) => {
        tshd.restartGateway(req, err => {
          if (err) {
            reject(err);
          } else {
            resolve();
          }
        });
      });
    },

    async setGatewayTargetSubresourceName(
      gatewayUri = '',
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
            resolve(response.toObject());
          }
        });
      });
    },

    async setGatewayLocalPort(gatewayUri: string, localPort: string) {
      const req = new api.SetGatewayLocalPortRequest()
        .setGatewayUri(gatewayUri)
        .setLocalPort(localPort);
      return new Promise<types.Gateway>((resolve, reject) => {
        tshd.setGatewayLocalPort(req, (err, response) => {
          if (err) {
            reject(err);
          } else {
            resolve(response.toObject());
          }
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
