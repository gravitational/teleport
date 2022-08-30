import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';

import { PtyHostClient } from '../types';

export function createPtyProcess(
  ptyHostClient: PtyHostClient,
  ptyId: string
): IPtyProcess {
  const exchangeEventsStream = ptyHostClient.exchangeEvents(ptyId);

  return {
    /**
     * Client -> Server stream events
     */

    start(columns: number, rows: number): void {
      exchangeEventsStream.start(columns, rows);
    },

    write(data: string): void {
      exchangeEventsStream.write(data);
    },

    resize(columns: number, rows: number): void {
      exchangeEventsStream.resize(columns, rows);
    },

    dispose(): void {
      exchangeEventsStream.dispose();
    },

    /**
     * Server -> Client stream events
     */

    onData(callback: (data: string) => void): void {
      exchangeEventsStream.onData(callback);
    },

    onOpen(callback: () => void): void {
      exchangeEventsStream.onOpen(callback);
    },

    onExit(
      callback: (reason: { exitCode: number; signal?: number }) => void
    ): void {
      exchangeEventsStream.onExit(callback);
    },

    /**
     * Unary calls
     */

    getCwd(): Promise<string> {
      return ptyHostClient.getCwd(ptyId);
    },
  };
}
