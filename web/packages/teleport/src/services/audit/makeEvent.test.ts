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
