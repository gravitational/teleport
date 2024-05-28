import React from 'react';
import { render, screen } from '@testing-library/react';

import '@testing-library/jest-dom';

import { ThemeProvider } from 'styled-components';
import 'jest-canvas-mock';
import { darkTheme } from 'design/theme';

import { act } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { TestLayout } from 'teleport/Console/Console.story';
import ConsoleCtx from 'teleport/Console/consoleContext';
import { createTeleportContext } from 'teleport/mocks/contexts';

import useKubeExecSession, { Status } from './useKubeExecSession';

import DocumentKubeExec from './DocumentKubeExec';

import type { Session } from 'teleport/services/session';

jest.mock('./useKubeExecSession');

const mockUseKubeExecSession = useKubeExecSession as jest.MockedFunction<
  typeof useKubeExecSession
>;

describe('DocumentKubeExec', () => {
  const setup = (status: Status) => {
    mockUseKubeExecSession.mockReturnValue({
      tty: {
        sendKubeExecData: jest.fn(),
        on: jest.fn(),
        removeListener: jest.fn(),
        connect: jest.fn(),
        disconnect: jest.fn(),
        removeAllListeners: jest.fn(),
      },
      status,
      closeDocument: jest.fn(),
    });

    const { ctx, consoleCtx } = getContexts();

    render(
      <ThemeProvider theme={darkTheme}>
        <ContextProvider ctx={ctx}>
          <TestLayout ctx={consoleCtx}>
            <DocumentKubeExec doc={baseDoc} visible={true} />
          </TestLayout>
        </ContextProvider>
      </ThemeProvider>
    );
  };

  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: jest.fn().mockImplementation(query => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: jest.fn(), // Deprecated
      removeListener: jest.fn(), // Deprecated
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
      dispatchEvent: jest.fn(),
    })),
  });

  test('renders loading indicator when status is loading', async () => {
    jest.useFakeTimers();
    setup('loading');

    act(() => jest.runAllTimers());
    expect(screen.getByTestId('indicator')).toBeInTheDocument();
  });

  test('renders terminal window when status is initialized', () => {
    setup('initialized');

    expect(screen.getByTestId('terminal')).toBeInTheDocument();
  });

  test('renders data dialog when status is initialized', () => {
    setup('initialized');

    expect(screen.getByText('Exec into a pod')).toBeInTheDocument();
  });

  test('does not render data dialog when status is disconnected', () => {
    setup('disconnected');

    expect(screen.queryByText('Exec into a pod')).not.toBeInTheDocument();
  });
});

function getContexts() {
  const ctx = createTeleportContext();
  const consoleCtx = new ConsoleCtx();
  const tty = consoleCtx.createTty(baseSession);
  tty.connect = () => null;
  consoleCtx.createTty = () => tty;
  consoleCtx.storeUser = ctx.storeUser;

  return { ctx, consoleCtx };
}

const baseDoc = {
  kind: 'kubeExec',
  status: 'connected',
  sid: 'sid-value',
  clusterId: 'clusterId-value',
  serverId: 'serverId-value',
  login: 'login-value',
  kubeCluster: 'kubeCluster1',
  kubeNamespace: 'namespace1',
  pod: 'pod1',
  container: '',
  id: 3,
  url: 'fd',
  created: new Date(),
  command: '/bin/bash',
  isInteractive: true,
} as const;

const baseSession: Session = {
  kind: 'k8s',
  login: '123',
  sid: '456',
  namespace: '',
  created: new Date(),
  durationText: '',
  serverId: '',
  resourceName: '',
  clusterId: '',
  parties: [],
  addr: '',
  participantModes: [],
  moderated: false,
  isInteractive: true,
  command: '/bin/bash',
};
