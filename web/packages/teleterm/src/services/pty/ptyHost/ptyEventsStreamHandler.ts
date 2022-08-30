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

  private writeOrThrow(event: PtyClientEvent) {
    return this.stream.write(event, (error: Error | undefined) => {
      if (error) {
        throw error;
      }
    });
  }
}
