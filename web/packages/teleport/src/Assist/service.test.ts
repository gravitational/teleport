/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
