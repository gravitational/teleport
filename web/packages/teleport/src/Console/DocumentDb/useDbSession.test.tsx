/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { act, renderHook, waitFor } from '@testing-library/react';

import { ContextProvider } from 'teleport';
import ConsoleCtx from 'teleport/Console/consoleContext';
import ConsoleContextProvider from 'teleport/Console/consoleContextProvider';
import { TermEvent } from 'teleport/lib/term/enums';
import { createTeleportContext } from 'teleport/mocks/contexts';
import type { SessionMetadata } from 'teleport/services/session';

import { useDbSession } from './useDbSession';

describe('useDbSession', () => {
  let consoleCtx: ConsoleCtx;
  let teleportCtx: any;
  let mockTty: any;

  beforeEach(() => {
    teleportCtx = createTeleportContext();
    consoleCtx = new ConsoleCtx();
    consoleCtx.storeUser = teleportCtx.storeUser;

    mockTty = {
      on: jest.fn(),
      removeAllListeners: jest.fn(),
      sendDbConnectData: jest.fn(),
    };

    consoleCtx.createTty = jest.fn(() => mockTty);
  });

  describe('URL handling after connection', () => {
    it('should keep the document at the connect URL after session is established', async () => {
      const connectUrl = '/web/cluster/test-cluster/console/db/connect/test-db';
      const docData = {
        kind: 'db' as const,
        clusterId: 'test-cluster',
        sid: undefined,
        name: 'test-db',
        url: connectUrl,
        created: new Date(),
      };

      const doc = consoleCtx.storeDocs.add(docData) as any;

      const updateDbDocumentSpy = jest.spyOn(consoleCtx, 'updateDbDocument');

      const wrapper = ({ children }) => (
        <ContextProvider ctx={teleportCtx}>
          <ConsoleContextProvider value={consoleCtx}>
            {children}
          </ConsoleContextProvider>
        </ContextProvider>
      );

      renderHook(() => useDbSession(doc), {
        wrapper,
      });

      const sessionMetadata: SessionMetadata = {
        kind: 'db',
        id: 'test-session-123',
        cluster_name: 'test-cluster',
        created: new Date().toISOString(),
        namespace: '',
        parties: [],
        terminal_params: { w: 80, h: 24 },
        login: '',
        last_active: '',
        server_id: '',
        server_hostname: '',
        server_hostport: 0,
        server_addr: '',
        kubernetes_cluster_name: '',
        desktop_name: '',
        database_name: 'test-db',
        app_name: '',
        resourceName: 'test-db',
      };

      const sessionCallback = mockTty.on.mock.calls.find(
        call => call[0] === TermEvent.SESSION
      )?.[1];

      await act(async () => {
        sessionCallback(JSON.stringify({ session: sessionMetadata }));
      });

      await waitFor(() => {
        expect(updateDbDocumentSpy).toHaveBeenCalled();
      });

      const updateCall = updateDbDocumentSpy.mock.calls[0];
      expect(updateCall[0]).toBe(doc.id);
      expect(updateCall[1]).toMatchObject({
        sid: 'test-session-123',
        clusterId: 'test-cluster',
        created: expect.any(Date),
      });
    });
  });
});
