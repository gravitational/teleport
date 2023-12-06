/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { rest } from 'msw';
import { setupServer } from 'msw/node';

import cfg from 'teleport/config';
import { EventType } from 'teleport/lib/term/enums';
import {
  getLoginsForQuery,
  getSessionEvents,
  resolveServerCommandResultMessage,
} from 'teleport/Assist/service';
import { ExecEvent, ServerMessageType } from 'teleport/Assist/types';

const sessionUrl = cfg.getSshPlaybackPrefixUrl({
  clusterId: 'test-cluster',
  sid: 'some-session-id',
});

const sessionUrlEmptyStream = cfg.getSshPlaybackPrefixUrl({
  clusterId: 'test-cluster',
  sid: 'some-empty-stream-session-id',
});

const nodesUrl = cfg
  .getClusterNodesUrl('cluster-id-nodes', { query: 'name == "node"' })
  .split('?')[0]; // remove the query params to make MSW happy;
const noNodesUrl = cfg
  .getClusterNodesUrl('cluster-id-no-nodes', { query: 'name == "node"' })
  .split('?')[0]; // remove the query params to make MSW happy;

const server = setupServer(
  rest.get(sessionUrl + '/events', (req, res, ctx) => {
    return res(
      ctx.json({
        events: [
          {
            event: EventType.EXEC,
            exitError: 'some error',
          },
        ],
      })
    );
  }),
  rest.get(sessionUrl + '/stream', (req, res, ctx) => {
    return res(ctx.text('some text'));
  }),
  rest.get(sessionUrlEmptyStream + '/stream', (req, res, ctx) => {
    return res(ctx.status(404));
  }),
  rest.get(sessionUrlEmptyStream + '/events', (req, res, ctx) => {
    return res(
      ctx.json({
        events: [
          {
            event: EventType.EXEC,
            exitError: 'some error',
          },
        ],
      })
    );
  }),
  rest.get(nodesUrl, (req, res, ctx) => {
    return res(
      ctx.json({
        items: [
          {
            id: 'node-id',
            sshLogins: [],
          },
        ],
      })
    );
  }),
  rest.get(noNodesUrl, (req, res, ctx) => {
    return res(
      ctx.json({
        items: [],
      })
    );
  })
);

beforeAll(() => server.listen());

afterEach(() => server.resetHandlers());

afterAll(() => server.close());

describe('assist service', () => {
  it('should return session events', async () => {
    const events = await getSessionEvents(sessionUrl);

    expect(events.events).toHaveLength(1);
    expect((events.events[0] as ExecEvent).exitError).toBe('some error');
  });

  it('should resolve a command result output to the stream text', async () => {
    const resolvedMessage = await resolveServerCommandResultMessage(
      {
        type: ServerMessageType.CommandResult,
        payload: JSON.stringify({
          session_id: 'some-session-id',
        }),
        created_time: '2021-01-01T00:00:00Z',
        conversation_id: 'some-conversation-id',
      },
      'test-cluster'
    );

    expect(resolvedMessage.output).toBe('some text');
  });

  it('should resolve a command result output to exec events exit error if no stream', async () => {
    const resolvedMessage = await resolveServerCommandResultMessage(
      {
        type: ServerMessageType.CommandResult,
        payload: JSON.stringify({
          session_id: 'some-empty-stream-session-id',
        }),
        created_time: '2021-01-01T00:00:00Z',
        conversation_id: 'some-conversation-id',
      },
      'test-cluster'
    );

    expect(resolvedMessage.output).toBe('some error');
  });

  it('should throw an error if there are no nodes', async () => {
    await expect(
      getLoginsForQuery('name == "node"', 'cluster-id-no-nodes')
    ).rejects.toThrow('No nodes match the query');
  });

  it('should throw an error if there are no logins for the nodes', async () => {
    await expect(
      getLoginsForQuery('name == "node"', 'cluster-id-nodes')
    ).rejects.toThrow('No available logins found');
  });
});
