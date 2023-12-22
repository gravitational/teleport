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
