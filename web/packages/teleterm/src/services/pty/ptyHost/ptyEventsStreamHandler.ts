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

import { ClientDuplexStream } from '@grpc/grpc-js';

import Logger from 'teleterm/logger';

import {
  PtyClientEvent,
  PtyEventData,
  PtyEventResize,
  PtyEventStart,
  PtyServerEvent,
} from 'teleterm/sharedProcess/ptyHost';

export class PtyEventsStreamHandler {
  private logger: Logger;

  constructor(
    private readonly stream: ClientDuplexStream<PtyClientEvent, PtyServerEvent>,
    ptyId: string
  ) {
    this.logger = new Logger(`PtyEventsStreamHandler ${ptyId}`);
  }

  /**
   * Client -> Server stream events
   */

  start(columns: number, rows: number): void {
    this.logger.info('Start');

    this.writeOrThrow(
      new PtyClientEvent().setStart(
        new PtyEventStart().setColumns(columns).setRows(rows)
      )
    );
  }

  write(data: string): void {
    this.writeOrThrow(
      new PtyClientEvent().setData(new PtyEventData().setMessage(data))
    );
  }

  resize(columns: number, rows: number): void {
    this.writeOrThrow(
      new PtyClientEvent().setResize(
        new PtyEventResize().setColumns(columns).setRows(rows)
      )
    );
  }

  dispose(): void {
    this.logger.info('Dispose');

    this.stream.end();
    this.stream.removeAllListeners();
  }

  /**
   * Stream -> Client stream events
   */

  onOpen(callback: () => void): RemoveListenerFunction {
    return this.addDataListenerAndReturnRemovalFunction(
      (event: PtyServerEvent) => {
        if (event.hasOpen()) {
          callback();
        }
      }
    );
  }

  onData(callback: (data: string) => void): RemoveListenerFunction {
    return this.addDataListenerAndReturnRemovalFunction(
      (event: PtyServerEvent) => {
        if (event.hasData()) {
          callback(event.getData().getMessage());
        }
      }
    );
  }

  onExit(
    callback: (reason: { exitCode: number; signal?: number }) => void
  ): RemoveListenerFunction {
    return this.addDataListenerAndReturnRemovalFunction(
      (event: PtyServerEvent) => {
        if (event.hasExit()) {
          this.logger.info('On exit', event.getExit().toObject());
          callback(event.getExit().toObject());
        }
      }
    );
  }

  onStartError(callback: (message: string) => void): RemoveListenerFunction {
    return this.addDataListenerAndReturnRemovalFunction(
      (event: PtyServerEvent) => {
        if (event.hasStartError()) {
          this.logger.info(
            'On start error',
            event.getStartError().toObject().message
          );
          callback(event.getStartError().toObject().message);
        }
      }
    );
  }

  private writeOrThrow(event: PtyClientEvent) {
    return this.stream.write(event, (error: Error | undefined) => {
      if (error) {
        throw error;
      }
    });
  }

  private addDataListenerAndReturnRemovalFunction(
    callback: (event: PtyServerEvent) => void
  ) {
    this.stream.addListener('data', callback);

    return () => {
      this.stream.removeListener('data', callback);
    };
  }
}

type RemoveListenerFunction = () => void;
