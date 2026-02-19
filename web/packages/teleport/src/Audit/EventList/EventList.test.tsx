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

import { screen, within } from '@testing-library/react';

import { render } from 'design/utils/testing';

import { RawEvents } from '../../services/audit';
import makeEvent from '../../services/audit/makeEvent';
import EventList from './EventList';

describe('EventList', () => {
  it('should sort events with same timestamp by event index', () => {
    const sameTimestamp = '2025-09-08T21:25:49.265Z';

    const mcpSessionStart = {
      code: 'TMCP001I',
      event: 'mcp.session.start',
      time: sameTimestamp,
      uid: '5dcf76ab-3f31-40a7-8550-bb29ecea1e42',
      user: 'admin',
      ei: 269750, // Lower event index - should be first
      sid: '5dcf76ab-3f31-40a7-8550-bb29ecea1e42',
      app_name: 'teleport-mcp-demo',
    } as RawEvents[typeof import('teleport/services/audit').eventCodes.MCP_SESSION_START];

    const mcpSessionRequest = {
      code: 'TMCP003I',
      event: 'mcp.session.request',
      time: sameTimestamp,
      uid: 'int64:0',
      user: 'admin',
      ei: 667167, // Higher event index - should be second
      app_name: 'teleport-mcp-demo',
      message: {
        id: 'init64:0',
        jsonrpc: '2.0',
        method: 'initialize',
        params: {
          capabilities: {},
          clientInfo: {
            name: 'claude-ai',
            version: '0.1.0',
          },
          protocolVersion: '2025-06-18',
        },
      },
    } as RawEvents[typeof import('teleport/services/audit').eventCodes.MCP_SESSION_REQUEST];

    const events = [makeEvent(mcpSessionStart), makeEvent(mcpSessionRequest)];

    render(
      <EventList
        events={events}
        fetchMore={() => null}
        fetchStatus=""
        pageSize={50}
      />
    );

    const table = screen.getByRole('table');
    const rows = within(table).getAllByRole('row');

    const firstDataRow = rows[1];
    const secondDataRow = rows[2];
    expect(
      within(firstDataRow).getByText(/MCP Session Started/i)
    ).toBeInTheDocument();
    expect(
      within(secondDataRow).getByText(/MCP Session Request/i)
    ).toBeInTheDocument();
  });

  it('should handle events with different timestamps correctly', () => {
    const olderEvent = {
      code: 'TMCP001I',
      event: 'mcp.session.start',
      time: '2025-09-08T21:25:48.000Z',
      uid: 'uid-1',
      user: 'admin',
      ei: 999999,
      sid: 'sid-1',
      app_name: 'teleport-mcp-demo',
    } as RawEvents[typeof import('teleport/services/audit').eventCodes.MCP_SESSION_START];

    const newerEvent = {
      code: 'TMCP003I',
      event: 'mcp.session.request',
      time: '2025-09-08T21:25:49.000Z',
      uid: 'uid-2',
      user: 'admin',
      ei: 1,
      app_name: 'teleport-mcp-demo',
      message: {
        id: 'int64:0',
        jsonrpc: '2.0',
        method: 'initialize',
        params: {
          capabilities: {},
          clientInfo: {
            name: 'claude-ai',
            version: '0.1.0',
          },
          protocolVersion: '2025-06-18',
        },
      },
    } as RawEvents[typeof import('teleport/services/audit').eventCodes.MCP_SESSION_REQUEST];

    const events = [makeEvent(olderEvent), makeEvent(newerEvent)];

    render(
      <EventList
        events={events}
        fetchMore={() => null}
        fetchStatus=""
        pageSize={50}
      />
    );

    const table = screen.getByRole('table');
    const rows = within(table).getAllByRole('row');

    const firstDataRow = rows[1];
    const secondDataRow = rows[2];

    expect(
      within(firstDataRow).getByText(/MCP Session Request/i)
    ).toBeInTheDocument();
    expect(
      within(secondDataRow).getByText(/MCP Session Started/i)
    ).toBeInTheDocument();
  });
});
