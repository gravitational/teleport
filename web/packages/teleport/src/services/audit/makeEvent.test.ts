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

import makeEvent from './makeEvent';
import { eventCodes } from './types';

describe('formatRawEventForUI', () => {
  test.each([
    [eventCodes.CREATE_MFA_AUTH_CHALLENGE, 0, 'UNSPECIFIED'],
    [eventCodes.CREATE_MFA_AUTH_CHALLENGE, 1, 'PER_SESSION_CERTIFICATE'],
    [eventCodes.CREATE_MFA_AUTH_CHALLENGE, 2, 'IN_BAND'],
    [eventCodes.VALIDATE_MFA_AUTH_RESPONSE, 0, 'UNSPECIFIED'],
    [eventCodes.VALIDATE_MFA_AUTH_RESPONSE, 1, 'PER_SESSION_CERTIFICATE'],
    [eventCodes.VALIDATE_MFA_AUTH_RESPONSE, 2, 'IN_BAND'],
  ])(
    'formats raw event for MFA event code %s with flow_type %s to %s',
    (code, flowType, expectedFlowType) => {
      const input = {
        code,
        event: 'mfa.auth',
        user: 'alice',
        time: '2026-01-01T00:00:00.000Z',
        flow_type: flowType,
      };

      const event = makeEvent(input);

      expect(event.raw).toEqual({
        code,
        event: 'mfa.auth',
        user: 'alice',
        time: '2026-01-01T00:00:00.000Z',
        flow_type: expectedFlowType,
      });
      expect(input.flow_type).toBe(flowType);
    }
  );

  test('leaves non-MFA events unchanged', () => {
    const input = {
      code: eventCodes.USER_LOCAL_LOGIN,
      event: 'user.login',
      user: 'alice',
      time: '2026-01-01T00:00:00.000Z',
    };

    const event = makeEvent(input);

    expect(event.raw).toBe(input);
  });
});

describe('Access Graph access path changed formatter', () => {
  test('renders all affected resources with source, kind, and name', () => {
    const event = makeEvent({
      affected_resources: [
        {
          name: 'node-a',
          source: 'TELEPORT',
          type: 'server',
        },
        {
          name: 'admins',
          source: 'Okta',
          kind: 'group',
        },
      ],
      change_id: 'change-id',
      code: eventCodes.ACCESS_GRAPH_PATH_CHANGED,
      event: 'access_graph.access_path_changed',
      time: '2026-01-01T00:00:00.000Z',
      uid: 'event-id',
    });

    expect(event.message).toBe(
      'Access path changed for resources [server/node-a@TELEPORT; group/admins@Okta]'
    );
  });

  test('renders legacy flat affected resource fields', () => {
    const event = makeEvent({
      affected_resource_name: 'node-a',
      affected_resource_source: 'TELEPORT',
      affected_resource_type: 'server',
      change_id: 'change-id',
      code: eventCodes.ACCESS_GRAPH_PATH_CHANGED,
      event: 'access_graph.access_path_changed',
      time: '2026-01-01T00:00:00.000Z',
      uid: 'event-id',
    });

    expect(event.message).toBe(
      'Access path changed for resources [server/node-a@TELEPORT]'
    );
  });

  test('trims long affected resource fields', () => {
    const longSource = 'source-'.repeat(20);
    const longKind = 'kind-'.repeat(20);
    const longName = 'name-'.repeat(20);

    const event = makeEvent({
      affected_resources: [
        {
          name: longName,
          source: longSource,
          type: longKind,
        },
      ],
      change_id: 'change-id',
      code: eventCodes.ACCESS_GRAPH_PATH_CHANGED,
      event: 'access_graph.access_path_changed',
      time: '2026-01-01T00:00:00.000Z',
      uid: 'event-id',
    });

    expect(event.message).toBe(
      `Access path changed for resources [${longKind.substring(0, 77)}.../${longName.substring(0, 77)}...@${longSource.substring(0, 77)}...]`
    );
  });
});
