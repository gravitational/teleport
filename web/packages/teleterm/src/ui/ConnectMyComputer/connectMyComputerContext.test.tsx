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

import { EventEmitter } from 'node:events';

import React from 'react';
import { renderHook } from '@testing-library/react-hooks';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { WorkspaceContextProvider } from 'teleterm/ui/Documents';
import { AgentProcessState } from 'teleterm/mainProcess/types';

import {
  ConnectMyComputerContextProvider,
  useConnectMyComputerContext,
} from './connectMyComputerContext';

test('runAgentAndWaitForNodeToJoin re-throws errors that are thrown while spawning the process', async () => {
  const mockedAppContext = new MockAppContext({});
  const eventEmitter = new EventEmitter();
  const errorStatus: AgentProcessState = { status: 'error', message: 'ENOENT' };
  jest
    .spyOn(mockedAppContext.mainProcessClient, 'getAgentState')
    .mockImplementation(() => errorStatus);
  jest
    .spyOn(mockedAppContext.connectMyComputerService, 'runAgent')
    .mockImplementation(async () => {
      // the error is emitted before the function resolves
      eventEmitter.emit('', errorStatus);
      return;
    });
  jest
    .spyOn(mockedAppContext.mainProcessClient, 'subscribeToAgentUpdate')
    .mockImplementation((rootClusterUri, listener) => {
      eventEmitter.on('', listener);
      return { cleanup: () => eventEmitter.off('', listener) };
    });

  const { result } = renderHook(() => useConnectMyComputerContext(), {
    wrapper: ({ children }) => (
      <MockAppContextProvider appContext={mockedAppContext}>
        <WorkspaceContextProvider value={null}>
          <ConnectMyComputerContextProvider rootClusterUri={'/clusters/a'}>
            {children}
          </ConnectMyComputerContextProvider>
        </WorkspaceContextProvider>
      </MockAppContextProvider>
    ),
  });

  await expect(result.current.runAgentAndWaitForNodeToJoin).rejects.toThrow(
    `Agent process failed to start.\nENOENT`
  );
});
