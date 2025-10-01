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

import { DuplexStreamingCall } from '@protobuf-ts/runtime-rpc';

import {
  PtyClientEvent,
  PtyEventData,
  PtyEventResize,
  PtyEventStart,
  PtyServerEvent,
} from 'gen-proto-ts/teleport/web/teleterm/ptyhost/v1/pty_host_service_pb';

import {
  ptyEventOneOfIsData,
  ptyEventOneOfIsExit,
  ptyEventOneOfIsStartError,
} from 'teleterm/helpers';
import Logger from 'teleterm/logger';

export class PtyEventsStreamHandler {
  private logger: Logger;

  constructor(
    private readonly stream: DuplexStreamingCall<
      PtyClientEvent,
      PtyServerEvent
    >,
    ptyId: string
  ) {
    this.logger = new Logger(`PtyEventsStreamHandler ${ptyId}`);
  }

  /**
   * Client -> Server stream events
   */

  async start(columns: number, rows: number): Promise<void> {
    this.logger.info('Start');

    await this._write(
      PtyClientEvent.create({
        event: {
          oneofKind: 'start',
          start: PtyEventStart.create({ columns, rows }),
        },
      })
    );
  }

  async write(data: string): Promise<void> {
    this._write(
      PtyClientEvent.create({
        event: {
          oneofKind: 'data',
          data: PtyEventData.create({ message: data }),
        },
      })
    );
  }

  async resize(columns: number, rows: number): Promise<void> {
    this._write(
      PtyClientEvent.create({
        event: {
          oneofKind: 'resize',
          resize: PtyEventResize.create({ columns, rows }),
        },
      })
    );
  }

  async dispose(): Promise<void> {
    this.logger.info('Dispose');

    await this.stream.requests.complete();
  }

  /**
   * Stream -> Client stream events
   */

  onOpen(callback: () => void): RemoveListenerFunction {
    return this.addDataListenerAndReturnRemovalFunction(
      (event: PtyServerEvent) => {
        if (event.event.oneofKind === 'open') {
          callback();
        }
      }
    );
  }

  onData(callback: (data: string) => void): RemoveListenerFunction {
    return this.addDataListenerAndReturnRemovalFunction(
      (event: PtyServerEvent) => {
        if (ptyEventOneOfIsData(event.event)) {
          callback(event.event.data.message);
        }
      }
    );
  }

  onExit(
    callback: (reason: { exitCode: number; signal?: number }) => void
  ): RemoveListenerFunction {
    return this.addDataListenerAndReturnRemovalFunction(
      (event: PtyServerEvent) => {
        if (ptyEventOneOfIsExit(event.event)) {
          this.logger.info('On exit', event.event.exit);
          callback(event.event.exit);
        }
      }
    );
  }

  onStartError(callback: (message: string) => void): RemoveListenerFunction {
    return this.addDataListenerAndReturnRemovalFunction(
      (event: PtyServerEvent) => {
        if (ptyEventOneOfIsStartError(event.event)) {
          this.logger.info('On start error', event.event.startError.message);
          callback(event.event.startError.message);
        }
      }
    );
  }

  private _write(event: PtyClientEvent): Promise<void> {
    return this.stream.requests.send(event);
  }

  private addDataListenerAndReturnRemovalFunction(
    callback: (event: PtyServerEvent) => void
  ) {
    return this.stream.responses.onMessage(callback);
  }
}

type RemoveListenerFunction = () => void;
