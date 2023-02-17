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

import { MockMainProcessClient } from 'teleterm/mainProcess/fixtures/mocks';
import { MockTshClient } from 'teleterm/services/tshd/fixtures/mocks';
import { MockPtyServiceClient } from 'teleterm/services/pty/fixtures/mocks';
import AppContext from 'teleterm/ui/appContext';

export class MockAppContext extends AppContext {
  constructor() {
    const mainProcessClient = new MockMainProcessClient();
    const tshdClient = new MockTshClient();
    const ptyServiceClient = new MockPtyServiceClient();
    const loggerService = createLoggerService();

    super({
      loggerService,
      mainProcessClient,
      tshClient: tshdClient,
      ptyServiceClient,
    });
  }
}

function createLoggerService() {
  return {
    pipeProcessOutputIntoLogger() {},
    createLogger() {
      return {
        error: () => {},
        warn: () => {},
        info: () => {},
      };
    },
  };
}
