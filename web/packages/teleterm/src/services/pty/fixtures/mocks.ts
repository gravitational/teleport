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

import {
  PtyProcessCreationStatus,
  PtyServiceClient,
} from 'teleterm/services/pty';
import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';

export class MockPtyProcess implements IPtyProcess {
  start() {}

  write() {}

  resize() {}

  async dispose() {}

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
  createPtyProcess() {
    return Promise.resolve({
      process: new MockPtyProcess(),
      creationStatus: PtyProcessCreationStatus.Ok,
      windowsPty: undefined,
      shell: {
        id: 'zsh',
        friendlyName: 'zsh',
        binPath: '/bin/zsh',
        binName: 'zsh',
      },
    });
  }
}
