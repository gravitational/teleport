/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import { screen } from '@testing-library/react';

import '@testing-library/jest-dom';
import 'jest-canvas-mock';

import { act, render } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { TestLayout } from 'teleport/Console/Console.story';
import ConsoleCtx from 'teleport/Console/consoleContext';
import Tty from 'teleport/lib/term/tty';
import { createTeleportContext } from 'teleport/mocks/contexts';
import type { Session } from 'teleport/services/session';

import DocumentKubeExec from './DocumentKubeExec';
import useKubeExecSession, { Status } from './useKubeExecSession';

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
      } as unknown as Tty,
      status,
      closeDocument: jest.fn(),
      sendKubeExecData: jest.fn(),
      session: baseSession,
    });

    const { ctx, consoleCtx } = getContexts();

    render(
      <ContextProvider ctx={ctx}>
        <TestLayout ctx={consoleCtx}>
          <DocumentKubeExec doc={baseDoc} visible={true} />
        </TestLayout>
      </ContextProvider>
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

  test('renders data dialog when status is waiting-for-exec-data', () => {
    setup('waiting-for-exec-data');

    expect(screen.getByText('Exec into a pod')).toBeInTheDocument();
  });

  test('does not render data dialog when status is initialized', () => {
    setup('initialized');

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
  command: '/bin/bash',
};
