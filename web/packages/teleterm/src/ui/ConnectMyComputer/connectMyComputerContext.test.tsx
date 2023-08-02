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
