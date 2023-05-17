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

import {
  PtyClientEvent,
  PtyEventData,
  PtyEventResize,
  PtyEventStart,
  PtyServerEvent,
} from 'teleterm/sharedProcess/ptyHost';

export class PtyEventsStreamHandler {
  constructor(
    private readonly stream: ClientDuplexStream<PtyClientEvent, PtyServerEvent>
  ) {}

  /**
   * Client -> Server stream events
   */

  start(columns: number, rows: number): void {
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
    this.stream.end();
    this.stream.removeAllListeners();
  }

  /**
   * Stream -> Client stream events
   */

  onOpen(callback: () => void): void {
    this.stream.addListener('data', (event: PtyServerEvent) => {
      if (event.hasOpen()) {
        callback();
      }
    });
  }

  onData(callback: (data: string) => void): void {
    this.stream.addListener('data', (event: PtyServerEvent) => {
      if (event.hasData()) {
        callback(event.getData().getMessage());
      }
    });
  }

  onExit(
    callback: (reason: { exitCode: number; signal?: number }) => void
  ): void {
    this.stream.addListener('data', (event: PtyServerEvent) => {
      if (event.hasExit()) {
        callback(event.getExit().toObject());
      }
    });
  }

  onStartError(callback: (message: string) => void): void {
    this.stream.addListener('data', (event: PtyServerEvent) => {
      if (event.hasStartError()) {
        callback(event.getStartError().toObject().message);
      }
    });
  }

  private writeOrThrow(event: PtyClientEvent) {
    return this.stream.write(event, (error: Error | undefined) => {
      if (error) {
        throw error;
      }
    });
  }
}
