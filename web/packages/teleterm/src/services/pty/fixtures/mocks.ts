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
import {
  PtyProcessCreationStatus,
  PtyServiceClient,
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
  }> {
    return Promise.resolve({
      process: new MockPtyProcess(),
      creationStatus: PtyProcessCreationStatus.Ok,
    });
  }
}
