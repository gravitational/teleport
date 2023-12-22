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

import { ITshdEventsServiceServer } from 'gen-proto-js/teleport/lib/teleterm/v1/tshd_events_service_grpc_pb';

import { TshClient } from 'teleterm/services/tshd/types';
import { PtyServiceClient } from 'teleterm/services/pty';
import { RuntimeSettings, MainProcessClient } from 'teleterm/mainProcess/types';
import { FileStorage } from 'teleterm/services/fileStorage';
import { Logger, LoggerService } from 'teleterm/services/logger/types';

export type {
  Logger,
  LoggerService,
  FileStorage,
  RuntimeSettings,
  MainProcessClient,
};

/**
 * SubscribeToTshdEvent is a type of the subscribeToTshdEvent function which gets exposed to the
 * renderer through the context bridge.
 *
 * A typical implementation of a gRPC service looks something like this:
 *
 *     {
 *       nameOfTheRpc: (call, callback) => {
 *         call.onCancelled(() => { … })
 *         const request = call.request.toObject()
 *         // Do something with the request fields…
 *       }
 *     }
 *
 * subscribeToTshdEvent lets you add a listener that's going to be called every time a client makes
 * a particular RPC to the tshd events service. The listener receives the request converted to a
 * simple JS object since classes cannot be passed through the context bridge.
 *
 * The SubscribeToTshdEvent type expresses all of this so that our subscribeToTshdEvent can stay
 * type safe.
 */
export type SubscribeToTshdEvent = <
  RpcName extends keyof ITshdEventsServiceServer,
  RpcHandler extends ITshdEventsServiceServer[RpcName],
  RpcHandlerServerCall extends Parameters<RpcHandler>[0],
  RpcHandlerRequestObject extends ReturnType<
    RpcHandlerServerCall['request']['toObject']
  >
>(
  eventName: RpcName,
  listener: (eventData: {
    request: RpcHandlerRequestObject;
    onCancelled: (callback: () => void) => void;
  }) => void | Promise<void>
) => void;

export type ElectronGlobals = {
  readonly mainProcessClient: MainProcessClient;
  readonly tshClient: TshClient;
  readonly ptyServiceClient: PtyServiceClient;
  readonly subscribeToTshdEvent: SubscribeToTshdEvent;
};
