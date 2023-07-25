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

import Emittery from 'emittery';
import * as grpc from '@grpc/grpc-js';
import * as api from 'gen-proto-js/teleport/lib/teleterm/v1/tshd_events_service_pb';
import * as apiService from 'gen-proto-js/teleport/lib/teleterm/v1/tshd_events_service_grpc_pb';

import * as uri from 'teleterm/ui/uri';
import Logger from 'teleterm/logger';
import { SubscribeToTshdEvent } from 'teleterm/types';

export interface ReloginRequest extends api.ReloginRequest.AsObject {
  rootClusterUri: uri.RootClusterUri;
  gatewayCertExpired?: GatewayCertExpired;
}
export interface GatewayCertExpired extends api.GatewayCertExpired.AsObject {
  gatewayUri: uri.GatewayUri;
  targetUri: uri.DatabaseUri;
}

export interface SendNotificationRequest
  extends api.SendNotificationRequest.AsObject {
  cannotProxyGatewayConnection?: CannotProxyGatewayConnection;
}
export interface CannotProxyGatewayConnection
  extends api.CannotProxyGatewayConnection.AsObject {
  gatewayUri: uri.GatewayUri;
  targetUri: uri.DatabaseUri;
}

/**
 * Starts tshd events server.
 * @return {Promise} Object containing the address the server is listening on and subscribeToEvent
 * function which lets UI code subscribe to events which are emitted when a client calls the server.
 */
export async function createTshdEventsServer(
  requestedAddress: string,
  credentials: grpc.ServerCredentials
): Promise<{
  resolvedAddress: string;
  subscribeToTshdEvent: SubscribeToTshdEvent;
}> {
  const logger = new Logger('tshd events');
  const { server, resolvedAddress } = await createServer(
    requestedAddress,
    credentials,
    logger
  );
  const { service, subscribeToTshdEvent } = createService(logger);

  server.addService(
    apiService.TshdEventsServiceService,
    // Whatever we use for generating protobufs generated wrong types. The types say that
    // server.addService expects an UntypedServiceImplementation as the second argument.
    // ITshdEventsServiceService does implement UntypedServiceImplementation.
    //
    // However, what we actually need to pass as the second argument needs to have the shape of
    // ITshdEventsServiceServer. That's why we ignore the error below.
    // @ts-expect-error The generated protobuf types seem to be wrong.
    service
  );

  return { resolvedAddress, subscribeToTshdEvent };
}

async function createServer(
  requestedAddress: string,
  credentials: grpc.ServerCredentials,
  logger: Logger
): Promise<{ server: grpc.Server; resolvedAddress: string }> {
  const server = new grpc.Server();

  // grpc-js requires us to pass localhost:port for TCP connections,
  const grpcServerAddress = requestedAddress.replace('tcp://', '');

  return new Promise((resolve, reject) => {
    try {
      server.bindAsync(grpcServerAddress, credentials, (error, port) => {
        if (error) {
          reject(error);
          return logger.error(error.message);
        }

        server.start();

        const resolvedAddress = requestedAddress.startsWith('tcp:')
          ? `localhost:${port}`
          : requestedAddress;

        logger.info(`tshd events server is listening on ${resolvedAddress}`);
        resolve({ server, resolvedAddress });
      });
    } catch (e) {
      logger.error('Could not start tshd events server', e);
      reject(e);
    }
  });
}

// createService creates a service that can be added to tshd events server. It also
// returns a function which lets UI code subscribe to events which are emitted when a client calls
// this service.
//
// Why do we need to use an event emitter? The gRPC server is created in the preload script but we
// need the UI to react to the events. We cannot create the service in the UI code because this
// would mean that the service would need to cross the contextBridge. This is simply impossible
// because the service is fed custom gRPC classes which can't be passed through the contextBridge.
//
// Instead, we create an event emitter and expose subscribeToEvent through the contextBridge.
// subscribeToEvent lets UI code register a callback for a specific event. That callback receives
// a simple JS object which can freely pass the contextBridge.
//
// # Async behavior
//
// The callback can return a promise. The service will not return a response until all callbacks
// resolve. This lets us model behavior where tshd calls the Electron app and then blocks until it
// receives a response, in case the Electron app needs to do some work before we want to unblock
// tshd.
//
// If any of the callbacks return an error, the service will return that error immediately, without
// waiting for other listeners.
function createService(logger: Logger): {
  service: apiService.ITshdEventsServiceServer;
  subscribeToTshdEvent: SubscribeToTshdEvent;
} {
  const emitter = new Emittery();

  const subscribeToTshdEvent: SubscribeToTshdEvent = (eventName, listener) => {
    emitter.on(eventName, listener);
  };

  const service: apiService.ITshdEventsServiceServer = {
    relogin: (call, callback) => {
      const request = call.request.toObject();

      logger.info('Emitting relogin', request);

      const onCancelled = (callback: () => void) => {
        call.on('cancelled', callback);
      };

      emitter.emit('relogin', { request, onCancelled }).then(
        () => {
          callback(null, new api.ReloginResponse());
        },
        error => {
          callback(error);
        }
      );
    },
    sendNotification: (call, callback) => {
      const request = call.request.toObject();

      logger.info('Emitting sendNotification', request);

      const onCancelled = (callback: () => void) => {
        call.on('cancelled', callback);
      };

      emitter.emit('sendNotification', { request, onCancelled }).then(
        () => {
          callback(null, new api.SendNotificationResponse());
        },
        error => {
          callback(error);
        }
      );
    },
    sendPendingHeadlessAuthentication: () => {
      // TODO (joerger): Handle pending headless authentications with an
      // approve/deny modal, followed by an MFA prompt for approval.
      logger.info('Received pending headless authentication');
    },
  };

  return { service, subscribeToTshdEvent };
}
