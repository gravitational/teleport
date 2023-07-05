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

import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';

import { PtyHostClient } from '../types';

export function createPtyProcess(
  ptyHostClient: PtyHostClient,
  ptyId: string
): IPtyProcess {
  const exchangeEventsStream = ptyHostClient.exchangeEvents(ptyId);

  return {
    getPtyId() {
      return ptyId;
    },

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

    onData(callback: (data: string) => void) {
      return exchangeEventsStream.onData(callback);
    },

    onOpen(callback: () => void) {
      return exchangeEventsStream.onOpen(callback);
    },

    onExit(callback: (reason: { exitCode: number; signal?: number }) => void) {
      return exchangeEventsStream.onExit(callback);
    },

    onStartError(callback: (message: string) => void) {
      return exchangeEventsStream.onStartError(callback);
    },

    /**
     * Unary calls
     */

    getCwd(): Promise<string> {
      return ptyHostClient.getCwd(ptyId);
    },
  };
}
