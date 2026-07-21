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

import makeEvent, { formatters } from './makeEvent';
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

describe('makeEvent', () => {
  test.each([
    [
      eventCodes.SPIFFE_FEDERATION_CREATE,
      // Must match SPIFFEFederationCreateEvent in lib/events/api.go, since
      // this is what the backend emits and matches against when filtering
      // the audit log by event type.
      'spiffe.federation.create',
      'SPIFFE Federation Created',
      'User [alice] created a SPIFFE federation [example.com]',
    ],
    [
      eventCodes.SPIFFE_FEDERATION_DELETE,
      // Must match SPIFFEFederationDeleteEvent in lib/events/api.go.
      'spiffe.federation.delete',
      'SPIFFE Federation Deleted',
      'User [alice] deleted a SPIFFE federation [example.com]',
    ],
  ])(
    'formats %s events instead of falling back to Unknown Event',
    (code, backendEventType, expectedDesc, expectedMessage) => {
      const event = makeEvent({
        code,
        event: backendEventType,
        user: 'alice',
        time: '2026-01-01T00:00:00.000Z',
        name: 'example.com',
      });

      expect(event.codeDesc).toBe(expectedDesc);
      expect(event.message).toBe(expectedMessage);
      // The formatter's `type` is used as the audit log's event-type filter
      // value, so it must match the backend's emitted event name or the
      // filter silently returns no results.
      expect(formatters[code].type).toBe(backendEventType);
    }
  );
});
