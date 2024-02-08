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
  >,
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
