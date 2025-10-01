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

import { IPtyProcess, PtyHostClient } from '../types';

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

    async start(columns: number, rows: number): Promise<void> {
      await exchangeEventsStream.start(columns, rows);
    },

    async write(data: string): Promise<void> {
      await exchangeEventsStream.write(data);
    },

    async resize(columns: number, rows: number): Promise<void> {
      await exchangeEventsStream.resize(columns, rows);
    },

    async dispose(): Promise<void> {
      await exchangeEventsStream.dispose();
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
