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

import * as grpc from '@grpc/grpc-js';
import * as api from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';
import * as apiService from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb.grpc-server';

import * as uri from 'teleterm/ui/uri';
import Logger from 'teleterm/logger';
import {
  ExtractRequestType,
  ExtractResponseType,
  TshdEventContextBridgeService,
} from 'teleterm/types';
import { filterSensitiveProperties } from 'teleterm/services/tshd/interceptors';

export interface ReloginRequest extends api.ReloginRequest {
  rootClusterUri: uri.RootClusterUri;
}
export interface GatewayCertExpired extends api.GatewayCertExpired {
  gatewayUri: uri.GatewayUri;
  targetUri: uri.DatabaseUri;
}

export type SendNotificationRequest = api.SendNotificationRequest;
export interface CannotProxyGatewayConnection
  extends api.CannotProxyGatewayConnection {
  gatewayUri: uri.GatewayUri;
  targetUri: uri.DatabaseUri;
}
export type PromptMfaRequest = api.PromptMFARequest & {
  rootClusterUri: uri.RootClusterUri;
};

export interface SendPendingHeadlessAuthenticationRequest
  extends api.SendPendingHeadlessAuthenticationRequest {
  rootClusterUri: uri.RootClusterUri;
}

/**
 * Starts tshd events server.
 * @return {Promise} Object containing the address the server is listening on and
 * setupTshdEventContextBridgeService function which lets UI code subscribe to events which are
 * emitted when tshd calls the Electron app.
 */
export async function createTshdEventsServer(
  requestedAddress: string,
  credentials: grpc.ServerCredentials
): Promise<{
  resolvedAddress: string;
  setupTshdEventContextBridgeService: (
    listener: TshdEventContextBridgeService
  ) => void;
}> {
  const logger = new Logger('tshd events');
  const { server, resolvedAddress } = await createServer(
    requestedAddress,
    credentials,
    logger
  );
  const { service, setupTshdEventContextBridgeService } = createService(logger);

  server.addService(apiService.tshdEventsServiceDefinition, service);

  return { resolvedAddress, setupTshdEventContextBridgeService };
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

/**
 * createService creates a service for the tshd events server. It sets up the preload part of the
 * service and expects the UI to call setupTshdEventContextBridgeService to add the browser part of
 * the service which interacts with the UI.
 *
 * See the JSDoc for TshdEventContextBridgeService for more details.
 */
function createService(logger: Logger): {
  service: apiService.ITshdEventsService;
  setupTshdEventContextBridgeService: (
    listener: TshdEventContextBridgeService
  ) => void;
} {
  let contextBridgeService: TshdEventContextBridgeService;

  const setupTshdEventContextBridgeService = (
    newListener: TshdEventContextBridgeService
  ) => {
    contextBridgeService = newListener;
  };

  /**
   * processEvent is a helper function encapsulating common operations done before and after sending
   * the event to the browser through the context bridge.
   *
   * The last argument is a function which maps the response object (received from the browser
   * through the context bridge) to a class instance (as expected by grpc-js).
   */
  function processEvent<
    RpcName extends keyof apiService.ITshdEventsService,
    Request extends ExtractRequestType<
      Parameters<apiService.ITshdEventsService[RpcName]>[0]
    >,
    Response extends ExtractResponseType<
      Parameters<apiService.ITshdEventsService[RpcName]>[1]
    >,
  >(
    rpcName: RpcName,
    call: grpc.ServerUnaryCall<Request, Response>,
    callback: (error: Error | null, response: Response | null) => void,
    mapResponseObjectToResponseInstance: (responseObject: Response) => Response
  ) {
    const request = call.request;

    logger.info(`got ${rpcName}`, filterSensitiveProperties(request));

    call.on('cancelled', () => {
      logger.error(`canceled by client ${rpcName}`);
    });

    const onRequestCancelled = (callback: () => void) => {
      call.on('cancelled', callback);
    };

    if (!contextBridgeService) {
      throw new Error(
        'tshd events context bridge service has not been set up yet'
      );
    }

    const contextBridgeHandler = contextBridgeService[rpcName];

    if (!contextBridgeHandler) {
      throw new Error(`No context bridge handler for ${rpcName}`);
    }

    contextBridgeHandler({ request, onRequestCancelled }).then(
      response => {
        if (call.cancelled) {
          return;
        }

        callback(null, mapResponseObjectToResponseInstance(response));

        logger.info(
          `replied to ${rpcName}`,
          filterSensitiveProperties(response)
        );
      },
      error => {
        if (call.cancelled) {
          return;
        }

        let responseErr = error;
        // TODO(ravicious): This is just an example of how cross-context errors can be signalled.
        // A more elaborate implementation should use a TypeScript assertion function
        // (isCrossContextError) and them somehow automatically build common gRPC status errors.
        // https://github.com/gravitational/teleport.e/issues/853
        // https://github.com/gravitational/teleport/issues/30753
        if (error['isCrossContextError'] && error['name'] === 'AbortError') {
          responseErr = new grpc.StatusBuilder()
            .withCode(grpc.status.ABORTED)
            .withDetails(error['message']);
        }

        callback(responseErr, null);

        logger.error(`replied with error to ${rpcName}`, responseErr);
      }
    );
  }

  const service: apiService.ITshdEventsService = {
    relogin: (call, callback) =>
      processEvent('relogin', call, callback, () =>
        api.ReloginResponse.create()
      ),

    sendNotification: (call, callback) =>
      processEvent('sendNotification', call, callback, () =>
        api.SendNotificationResponse.create()
      ),

    sendPendingHeadlessAuthentication: (call, callback) =>
      processEvent('sendPendingHeadlessAuthentication', call, callback, () =>
        api.SendPendingHeadlessAuthenticationResponse.create()
      ),

    promptMFA: (call, callback) => {
      processEvent('promptMFA', call, callback, response =>
        api.PromptMFAResponse.create({ totpCode: response?.totpCode })
      );
    },
  };

  return { service, setupTshdEventContextBridgeService };
}
