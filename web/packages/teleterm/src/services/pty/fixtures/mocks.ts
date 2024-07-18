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

import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';
import {
  PtyProcessCreationStatus,
  PtyServiceClient,
  WindowsPty,
} from 'teleterm/services/pty';

export class MockPtyProcess implements IPtyProcess {
  start() {}

  write() {}

  resize() {}

  dispose() {}

  onData() {
    return () => {};
  }

  onExit() {
    return () => {};
  }

  onOpen() {
    return () => {};
  }

  onStartError() {
    return () => {};
  }

  getPid() {
    return 0;
  }

  getPtyId() {
    return '1234';
  }

  async getCwd() {
    return '';
  }
}

export class MockPtyServiceClient implements PtyServiceClient {
  createPtyProcess(): Promise<{
    process: IPtyProcess;
    creationStatus: PtyProcessCreationStatus;
    windowsPty: WindowsPty;
  }> {
    return Promise.resolve({
      process: new MockPtyProcess(),
      creationStatus: PtyProcessCreationStatus.Ok,
      windowsPty: undefined,
    });
  }
}
