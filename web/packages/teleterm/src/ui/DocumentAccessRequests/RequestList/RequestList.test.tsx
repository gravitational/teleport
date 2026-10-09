/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { MemoryRouter } from 'react-router';

import { render, screen, userEvent, within } from 'design/utils/testing';
import { requestRoleApproved } from 'shared/components/AccessRequests/fixtures';
import { RequestFlags } from 'shared/components/AccessRequests/ReviewRequests';
import { makeSuccessAttempt } from 'shared/hooks/useAsync';
import { AccessRequest } from 'shared/services/accessRequests';

import { RequestList } from './RequestList';

test('searches each request by its own stored display values and username', async () => {
  // Setup.
  const requests = [
    makeRequest('garcia', {
      user: '123456',
      userDisplay: {
        primary: 'Jane Garcia',
        secondary: 'jane.long+alerts@example.com',
      },
    }),
    makeRequest('smith', {
      user: '123456',
      userDisplay: { primary: 'Jane Smith' },
    }),
    makeRequest('older', { user: '123456' }),
    makeRequest('email', {
      user: 'email-only',
      userDisplay: { secondary: 'only.email+qa@example.com' },
    }),
  ];
  renderRequests(requests);
  const user = userEvent.setup();

  // Execute.
  await searchFor(user, 'jane garcia');
  // Assert.
  expect(visibleRequestIds()).toEqual(['search-request-garcia']);

  // Execute.
  await searchFor(user, 'long+alerts@example.com');
  // Assert.
  expect(visibleRequestIds()).toEqual(['search-request-garcia']);

  // Execute.
  await searchFor(user, 'Jane Smith');
  // Assert.
  expect(visibleRequestIds()).toEqual(['search-request-smith']);

  // Execute.
  await searchFor(user, 'only.email+qa@example.com');
  // Assert.
  expect(visibleRequestIds()).toEqual(['search-request-email']);

  // Execute.
  await searchFor(user, '123456');
  // Assert.
  expect(visibleRequestIds().sort()).toEqual([
    'search-request-garcia',
    'search-request-older',
    'search-request-smith',
  ]);

  // Execute.
  await searchFor(user, 'Garcia Jane');
  // Assert.
  expect(screen.getByText('No Requests Found')).toBeInTheDocument();
});

function makeRequest(
  id: string,
  overrides: Partial<AccessRequest> = {}
): AccessRequest {
  return {
    ...requestRoleApproved,
    id: `search-request-${id}`,
    user: `user-${id}`,
    ...overrides,
  };
}

function renderRequests(requests: AccessRequest[]) {
  const props = {
    assumeRole: jest.fn(),
    assumeRoleAttempt: makeSuccessAttempt<void>(undefined),
    getRequests: jest.fn(),
    viewRequest: jest.fn(),
    assumeAccessList: jest.fn(),
    getFlags: () => flags,
  };
  render(
    <MemoryRouter>
      <RequestList {...props} attempt={makeSuccessAttempt(requests)} />
    </MemoryRouter>
  );
}

async function searchFor(
  user: ReturnType<typeof userEvent.setup>,
  query: string
) {
  const input = screen.getByPlaceholderText('Search...');
  await user.clear(input);
  await user.type(input, `${query}{enter}`);
}

function visibleRequestIds(): string[] {
  return screen
    .getAllByRole('row')
    .slice(1)
    .flatMap(row => {
      const id = within(row).getAllByRole('cell')[0].getAttribute('title');
      return id ? [id] : [];
    });
}

test('disabled assume button with assume start date', async () => {
  // Set system time before the assume start date.
  jest.useFakeTimers().setSystemTime(new Date('2024-02-16T02:51:12.70087Z'));

  const user = userEvent.setup({ advanceTimers: jest.advanceTimersByTime });

  render(
    <MemoryRouter>
      <RequestList
        attempt={makeSuccessAttempt([request])}
        assumeRole={() => null}
        assumeRoleAttempt={{ status: '', data: null, statusText: '' }}
        getRequests={() => null}
        viewRequest={() => null}
        assumeAccessList={() => null}
        getFlags={() => flags}
      />
    </MemoryRouter>
  );

  const assumeBtn = screen.getByText(/assume roles/i);
  expect(assumeBtn).toBeDisabled();

  // Mouse over the disabled button, and expect a popup message.
  await user.hover(assumeBtn);
  expect(
    screen.getByText(/access is not available until the approved time/i)
  ).toBeInTheDocument();
});

test('enabled assume button with assume start date', () => {
  // Set system time as same as assume start time
  jest.useFakeTimers().setSystemTime(request.assumeStartTime);

  render(
    <MemoryRouter>
      <RequestList
        attempt={makeSuccessAttempt([request])}
        assumeRole={() => null}
        assumeRoleAttempt={{ status: '', data: null, statusText: '' }}
        getRequests={() => null}
        viewRequest={() => null}
        assumeAccessList={() => null}
        getFlags={() => flags}
      />
    </MemoryRouter>
  );

  const assumeBtn = screen.getByText(/assume roles/i);
  expect(assumeBtn).toBeEnabled();
});

test('enabled assume button with no assume start date', () => {
  render(
    <MemoryRouter>
      <RequestList
        attempt={makeSuccessAttempt([
          { ...request, assumeStartTime: null, assumeStartTimeDuration: '' },
        ])}
        assumeRole={() => null}
        assumeRoleAttempt={{ status: '', data: null, statusText: '' }}
        getRequests={() => null}
        viewRequest={() => null}
        assumeAccessList={() => null}
        getFlags={() => flags}
      />
    </MemoryRouter>
  );

  const assumeBtn = screen.getByText(/assume roles/i);
  expect(assumeBtn).toBeEnabled();
});

const request: AccessRequest = {
  ...requestRoleApproved,
  assumeStartTime: new Date('2024-02-17T02:51:12.70087Z'),
  assumeStartTimeDuration: '24 hours from now',
};

const flags: RequestFlags = {
  canAssume: true,
  isAssumed: false,
  ownRequest: true,
  isPromoted: false,
  canReview: true,
  canDelete: true,
};
