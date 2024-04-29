import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { fireEvent, render, screen } from 'design/utils/testing';

import { requestRoleApproved } from 'e-teleport/AccessRequests/fixtures';
import { AccessRequest } from 'e-teleport/services/accessRequests';
import { RequestFlags } from 'e-teleport/AccessRequests/ReviewRequests';

import { RequestList } from './RequestList';

test('disabled assume button with assume start date', async () => {
  // Set system time before the assume start date.
  jest.useFakeTimers().setSystemTime(new Date('2024-02-16T02:51:12.70087Z'));

  render(
    <MemoryRouter>
      <RequestList
        attempt={{ status: 'success' }}
        assumeRole={() => null}
        assumeRoleAttempt={{ status: '', data: null, statusText: '' }}
        getRequests={() => null}
        viewRequest={() => null}
        assumeAccessList={() => null}
        getFlags={() => flags}
        requests={[request]}
      />
    </MemoryRouter>
  );

  const assumeBtn = screen.getByText(/assume roles/i);
  expect(assumeBtn).toBeDisabled();

  // Mouse over the disabled button, and expect a popup message.
  fireEvent.mouseEnter(assumeBtn);
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
        attempt={{ status: 'success' }}
        assumeRole={() => null}
        assumeRoleAttempt={{ status: '', data: null, statusText: '' }}
        getRequests={() => null}
        viewRequest={() => null}
        assumeAccessList={() => null}
        getFlags={() => flags}
        requests={[request]}
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
        attempt={{ status: 'success' }}
        assumeRole={() => null}
        assumeRoleAttempt={{ status: '', data: null, statusText: '' }}
        getRequests={() => null}
        viewRequest={() => null}
        assumeAccessList={() => null}
        getFlags={() => flags}
        requests={[
          { ...request, assumeStartTime: null, assumeStartTimeDuration: '' },
        ]}
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
