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

import { ITshdEventsService } from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb.grpc-server';

import { sendUnaryData, ServerUnaryCall } from 'grpc';

import { Logger, LoggerService } from 'teleterm/services/logger/types';
import { FileStorage } from 'teleterm/services/fileStorage';
import { MainProcessClient, RuntimeSettings } from 'teleterm/mainProcess/types';
import { PtyServiceClient } from 'teleterm/services/pty';
import { TshdClient } from 'teleterm/services/tshd/createClient';

export type {
  Logger,
  LoggerService,
  FileStorage,
  RuntimeSettings,
  MainProcessClient,
};

/**
 * TshdEventContextBridgeService is the part of the tshd events service running in the browser. It
 * is a version of ITshdEventsServiceServer with properties mapped to values that can be passed
 * through the context bridge.
 *
 * The tshd events gRPC service itself is set up in preload.ts. A typical implementation of a
 * handler for an RPC called "foo" of ITshdEventsServiceServer looks something like this:
 *
 * {
 *   foo: (call, callback) => {
 *     const result = processRequest(call.request)
 *     callback(null, new api.FooResponse().setBar(result.bar))
 *   }
 * }
 *
 * However, we want the actual logic of tshd event handlers to interact with UI elements in the app.
 * This means that the gRPC handlers need to run in the browser context, which in turns means that
 * the gRPC handlers need to call functions that cross the context bridge.
 *
 * grpc-js expects that `call.request` and the second argument to `callback` are class instances.
 * Unfortunately, class instances cannot be passed through the context bridge. This means that we
 * have to cast them to simple objects first.
 *
 * This means that the gRPC handler on the preload.ts side needs to do something like this:
 *
 * {
 *   foo: (call, callback) => {
 *     const result = processRequestInBrowser(call.request.toObject())
 *     callback(null, new api.FooResponse().setBar(result.bar))
 *   }
 * }
 *
 * …so that the implementation of TshdEventContextBridgeService can look like this:
 *
 * {
 *   foo: async ({ request, onCancelled }) => {
 *     doSomething(request)
 *     return { bar: 1234 }
 *   }
 * }
 *
 * The functions always need to return something, even if technically the handler is not going to
 * utilize the returned object. This helps with keeping type safety in cases where the handler
 * actually uses the returned object.
 */
export type TshdEventContextBridgeService = {
  [RpcName in keyof ITshdEventsService]: (args: {
    /**
     * request is the result of calling call.request.toObject() in a gRPC handler.
     */
    request: ExtractRequestType<Parameters<ITshdEventsService[RpcName]>[0]>;
    /**
     * onRequestCancelled sets up a callback that is called when the request gets canceled by the
     * client (tshd in this case).
     */
    onRequestCancelled: (callback: () => void) => void;
  }) => Promise<
    // The following type maps to the object version of the response type expected as the second
    // argument to the callback function in a gRPC handler.
    ExtractResponseType<Parameters<ITshdEventsService[RpcName]>[1]>
  >;
};

export type ExtractRequestType<T> =
  T extends ServerUnaryCall<infer Req, any> ? Req : never;

export type ExtractResponseType<T> =
  T extends sendUnaryData<infer Res> ? Res : never;

export type ElectronGlobals = {
  readonly mainProcessClient: MainProcessClient;
  readonly tshClient: TshdClient;
  readonly ptyServiceClient: PtyServiceClient;
  readonly setupTshdEventContextBridgeService: (
    listener: TshdEventContextBridgeService
  ) => void;
};
