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
import { MemoryRouter, useLocation } from 'react-router';

import '@testing-library/jest-dom';
import 'jest-canvas-mock';

import { act, render } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import ConsoleCtx from 'teleport/Console/consoleContext';
import ConsoleContextProvider from 'teleport/Console/consoleContextProvider';
import type { DocumentDb as DocumentDbType } from 'teleport/Console/stores';
import { TermEvent } from 'teleport/lib/term/enums';
import { createTeleportContext } from 'teleport/mocks/contexts';
import ResourceService from 'teleport/services/resources';
import type { Session } from 'teleport/services/session';

import { DocumentDb } from './DocumentDb';

// Mock Terminal component to avoid WebGL errors in jsdom
jest.mock('teleport/Console/DocumentSsh/Terminal', () => ({
  Terminal: jest.fn(() => <div data-testid="terminal">Terminal Mock</div>),
}));

const mockDatabase = {
  kind: 'db' as const,
  name: 'mydb',
  protocol: 'postgres' as const,
  names: ['test-db'],
  users: ['test-user'],
  roles: [],
  description: '',
  type: 'self-hosted',
  labels: [],
  hostname: 'localhost',
};

beforeEach(() => {
  jest
    .spyOn(ResourceService.prototype, 'fetchUnifiedResources')
    .mockResolvedValue({
      agents: [mockDatabase],
      startKey: '',
      totalCount: 1,
    });
});

afterEach(() => {
  jest.restoreAllMocks();
});

test('renders terminal window when session is established', async () => {
  const { ctx, consoleCtx, tty } = getContexts();

  render(
    <ContextProvider ctx={ctx}>
      <ConsoleContextProvider value={consoleCtx}>
        <DocumentDb doc={baseDoc} visible={true} />
      </ConsoleContextProvider>
    </ContextProvider>
  );

  await act(() =>
    tty.emit(TermEvent.SESSION, JSON.stringify({ session: { kind: 'db' } }))
  );

  expect(screen.getByTestId('terminal')).toBeInTheDocument();
});

test('renders data dialog when status is waiting', async () => {
  const { ctx, consoleCtx } = getContexts();

  render(
    <ContextProvider ctx={ctx}>
      <ConsoleContextProvider value={consoleCtx}>
        <DocumentDb doc={baseDoc} visible={true} />
      </ConsoleContextProvider>
    </ContextProvider>
  );

  expect(await screen.findByText('Connect To Database')).toBeInTheDocument();
});

test('does not render data dialog when status is initialized', async () => {
  const { ctx, consoleCtx, tty } = getContexts();

  tty.socket = { send: jest.fn() } satisfies Pick<WebSocket, 'send'>;

  render(
    <ContextProvider ctx={ctx}>
      <ConsoleContextProvider value={consoleCtx}>
        <DocumentDb doc={baseDoc} visible={true} />
      </ConsoleContextProvider>
    </ContextProvider>
  );

  const connectDialog = await screen.findByText('Connect To Database');
  expect(connectDialog).toBeInTheDocument();

  const connectButton = await screen.findByRole('button', { name: 'Connect' });
  await act(async () => {
    connectButton.click();
  });

  expect(screen.queryByText('Connect To Database')).not.toBeInTheDocument();
});

test('should keep the document at the connect URL after connecting', async () => {
  const connectUrl = '/web/cluster/test-cluster/console/db/connect/test-db';
  const connectDoc: DocumentDbType = {
    kind: 'db' as const,
    sid: 'test-session-id',
    clusterId: 'test-cluster',
    url: connectUrl,
    created: new Date(),
    name: 'test-db',
  };

  const { ctx, consoleCtx, tty } = getContexts();

  render(
    <MemoryRouter initialEntries={[connectUrl]}>
      <ContextProvider ctx={ctx}>
        <ConsoleContextProvider value={consoleCtx}>
          <DocumentDb doc={connectDoc} visible={true} />
        </ConsoleContextProvider>
      </ContextProvider>
      <LocationDisplay />
    </MemoryRouter>
  );

  expect(screen.getByTestId('location-display')).toHaveTextContent(connectUrl);

  await act(() =>
    tty.emit(TermEvent.SESSION, JSON.stringify({ session: { kind: 'db' } }))
  );

  expect(screen.getByTestId('location-display')).toHaveTextContent(connectUrl);
});

const LocationDisplay = () => {
  const location = useLocation();

  return <div data-testid="location-display">{location.pathname}</div>;
};

function getContexts() {
  const ctx = createTeleportContext();

  const consoleCtx = new ConsoleCtx();
  const tty = consoleCtx.createTty(baseSession);
  tty.connect = () => null;
  consoleCtx.createTty = () => tty;
  consoleCtx.storeUser = ctx.storeUser;

  return { ctx, consoleCtx, tty };
}

const baseDoc: DocumentDbType = {
  kind: 'db',
  sid: 'sid-value',
  clusterId: 'clusterId-value',
  url: 'fd',
  created: new Date(),
  name: 'mydb',
};

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
