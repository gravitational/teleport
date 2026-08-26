/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { userEventService } from 'teleport/services/userEvent';

beforeAll(() => {
  jest.spyOn(userEventService, 'captureIntegrationEnrollEvent');
});

afterEach(() => {
  jest.mocked(userEventService.captureIntegrationEnrollEvent).mockClear();
});

export function trackingTester() {
  let nextIndex = 0;

  const assertTracking = (data: unknown) => {
    const { calls } = jest.mocked(
      userEventService.captureIntegrationEnrollEvent
    ).mock;

    const call = calls[nextIndex];
    if (!call) {
      throw new Error('expected a tracking call but received none');
    }

    expect(call[0]).toStrictEqual(data);

    nextIndex += 1;
  };

  return {
    skip: (count: number = 1) => {
      nextIndex += count;
    },
    assertStart(eventId: string) {
      assertTracking({
        event: 'tp.ui.integrationEnroll.start',
        eventData: {
          id: eventId,
          kind: 'INTEGRATION_ENROLL_KIND_MACHINE_ID_GITHUB_ACTIONS_KUBERNETES',
        },
      });
    },
    assertComplete(eventId: string) {
      assertTracking({
        event: 'tp.ui.integrationEnroll.complete',
        eventData: {
          id: eventId,
          kind: 'INTEGRATION_ENROLL_KIND_MACHINE_ID_GITHUB_ACTIONS_KUBERNETES',
        },
      });
    },
    assertStep(eventId: string, step: string, status: string) {
      assertTracking({
        event: 'tp.ui.integrationEnroll.step',
        eventData: {
          id: eventId,
          kind: 'INTEGRATION_ENROLL_KIND_MACHINE_ID_GITHUB_ACTIONS_KUBERNETES',
          step,
          status: {
            code: status,
          },
        },
      });
    },
    assertError(eventId: string, step: string, error: string) {
      assertTracking({
        event: 'tp.ui.integrationEnroll.step',
        eventData: {
          id: eventId,
          kind: 'INTEGRATION_ENROLL_KIND_MACHINE_ID_GITHUB_ACTIONS_KUBERNETES',
          step,
          status: {
            code: 'INTEGRATION_ENROLL_STATUS_CODE_ERROR',
            error,
          },
        },
      });
    },
    assertField(eventId: string, step: string, field: string) {
      assertTracking({
        event: 'tp.ui.integrationEnroll.fieldComplete',
        eventData: {
          id: eventId,
          kind: 'INTEGRATION_ENROLL_KIND_MACHINE_ID_GITHUB_ACTIONS_KUBERNETES',
          step,
          field,
        },
      });
    },
    assertSection(eventId: string, step: string, section: string) {
      assertTracking({
        event: 'tp.ui.integrationEnroll.sectionOpen',
        eventData: {
          id: eventId,
          kind: 'INTEGRATION_ENROLL_KIND_MACHINE_ID_GITHUB_ACTIONS_KUBERNETES',
          step,
          section,
        },
      });
    },
  };
}
