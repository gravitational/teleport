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

import { DocumentDb } from './DocumentDb';
import { Status, useDbSession } from './useDbSession';

jest.mock('./useDbSession');

const mockUseDbSession = useDbSession as jest.MockedFunction<
  typeof useDbSession
>;

const setup = (status: Status) => {
  mockUseDbSession.mockReturnValue({
    tty: {
      sendDbConnectData: jest.fn(),
      on: jest.fn(),
      removeListener: jest.fn(),
      connect: jest.fn(),
      disconnect: jest.fn(),
      removeAllListeners: jest.fn(),
    } as unknown as Tty,
    status,
    closeDocument: jest.fn(),
    sendDbConnectData: jest.fn(),
    session: baseSession,
  });

  const { ctx, consoleCtx } = getContexts();

  render(
    <ContextProvider ctx={ctx}>
      <TestLayout ctx={consoleCtx}>
        <DocumentDb doc={baseDoc} visible={true} />
      </TestLayout>
    </ContextProvider>
  );
};

test('renders loading indicator when status is loading', () => {
  jest.useFakeTimers();
  setup('loading');

  act(() => jest.runAllTimers());
  expect(screen.getByTestId('indicator')).toBeInTheDocument();
});

test('renders terminal window when status is initialized', () => {
  setup('initialized');

  expect(screen.getByTestId('terminal')).toBeInTheDocument();
});

test('renders data dialog when status is waiting', () => {
  setup('waiting');

  expect(screen.getByText('Connect To Database')).toBeInTheDocument();
});

test('does not render data dialog when status is initialized', () => {
  setup('initialized');

  expect(screen.queryByText('Connect to Database')).not.toBeInTheDocument();
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
  kind: 'db',
  sid: 'sid-value',
  clusterId: 'clusterId-value',
  serverId: 'serverId-value',
  login: 'login-value',
  url: 'fd',
  created: new Date(),
  name: 'mydb',
} as const;

const baseSession: Session = {
  kind: 'db',
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
