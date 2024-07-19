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

import { ServerDuplexStream } from '@grpc/grpc-js';

import Logger from 'teleterm/logger';

import {
  PtyClientEvent,
  PtyEventData,
  PtyEventExit,
  PtyEventOpen,
  PtyEventResize,
  PtyEventStart,
  PtyEventStartError,
  PtyServerEvent,
} from '../api/protogen/ptyHostService_pb';

import { PtyProcess } from './ptyProcess';

export class PtyEventsStreamHandler {
  private readonly ptyId: string;
  private readonly ptyProcess: PtyProcess;
  private readonly logger: Logger;

  constructor(
    private readonly stream: ServerDuplexStream<PtyClientEvent, PtyServerEvent>,
    private readonly ptyProcesses: Map<string, PtyProcess>
  ) {
    this.ptyId = stream.metadata.get('ptyId')[0].toString();
    this.ptyProcess = ptyProcesses.get(this.ptyId);
    this.logger = new Logger(`PtyEventsStreamHandler (id: ${this.ptyId})`);

    stream.addListener('data', event => this.handleStreamData(event));
    stream.addListener('error', event => this.handleStreamError(event));
    stream.addListener('end', () => this.handleStreamEnd());
  }

  private handleStreamData(event: PtyClientEvent): void {
    switch (event.getEventCase()) {
      case PtyClientEvent.EventCase.START:
        return this.handleStartEvent(event.getStart());
      case PtyClientEvent.EventCase.DATA:
        return this.handleDataEvent(event.getData());
      case PtyClientEvent.EventCase.RESIZE:
        return this.handleResizeEvent(event.getResize());
    }
  }

  private handleStartEvent(event: PtyEventStart): void {
    this.ptyProcess.onData(data =>
      this.stream.write(
        new PtyServerEvent().setData(new PtyEventData().setMessage(data))
      )
    );
    this.ptyProcess.onOpen(() =>
      this.stream.write(new PtyServerEvent().setOpen(new PtyEventOpen()))
    );
    this.ptyProcess.onExit(({ exitCode, signal }) =>
      this.stream.write(
        new PtyServerEvent().setExit(
          new PtyEventExit().setExitCode(exitCode).setSignal(signal)
        )
      )
    );
    this.ptyProcess.onStartError(message => {
      this.stream.write(
        new PtyServerEvent().setStartError(
          new PtyEventStartError().setMessage(message)
        )
      );
    });
    // PtyProcess.prototype.start always returns a fulfilled promise. If an error is caught during
    // start, it's reported through PtyProcess.prototype.onStartError. Similarly, the information
    // about a successful start is also conveyed through an emitted event rather than the method
    // returning with no error. Hence why we can ignore what this promise returns.
    void this.ptyProcess.start(event.getColumns(), event.getRows()).then(() => {
      this.logger.info(`stream has started`);
    });
  }

  private handleDataEvent(event: PtyEventData): void {
    this.ptyProcess.write(event.getMessage());
  }

  private handleResizeEvent(event: PtyEventResize): void {
    this.ptyProcess.resize(event.getColumns(), event.getRows());
  }

  private handleStreamError(error: Error): void {
    this.logger.error(`stream has ended with error`, error);
    this.cleanResources();
  }

  private handleStreamEnd(): void {
    this.logger.info(`stream has ended`);
    this.cleanResources();
  }

  private cleanResources(): void {
    this.ptyProcess.dispose();
    if (this.ptyId) {
      this.ptyProcesses.delete(this.ptyId);
    }
    this.stream.destroy();
    this.stream.removeAllListeners();
  }
}
