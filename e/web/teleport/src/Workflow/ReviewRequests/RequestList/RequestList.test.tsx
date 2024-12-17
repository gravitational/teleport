import { MemoryRouter } from 'react-router-dom';
import { fireEvent, render, screen } from 'design/utils/testing';

import { requestRoleApproved } from 'shared/components/AccessRequests/fixtures';

import { RequestList } from './RequestList';
import { AccessRequestWithFlags } from './useRequestList';
import { sample } from './fixtures';

test('disabled assume button with assume start date', async () => {
  // Set system time before the assume start date.
  jest.useFakeTimers().setSystemTime(new Date('2024-02-16T02:51:12.70087Z'));
  global.IntersectionObserver = jest.fn(callback => {
    callback(
      [
        {
          // This is the property that triggers the fetch. We need it to be true.
          isIntersecting: true,
          intersectionRatio: null,
          boundingClientRect: null,
          intersectionRect: null,
          rootBounds: null,
          target: null,
          time: null,
        },
      ],
      null
    );
    return {
      observe: jest.fn(),
      unobserve: jest.fn(),
      disconnect: jest.fn(),
      takeRecords: jest.fn(),
      root: null,
      rootMargin: null,
      thresholds: null,
    };
  });

  render(
    <MemoryRouter>
      <RequestList
        {...sample}
        attempt={{ status: 'success' }}
        assumeRole={() => null}
        resources={[request]}
      />
    </MemoryRouter>
  );

  const assumeBtn = screen.getByText('Assume Roles');
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
        {...sample}
        attempt={{ status: 'success' }}
        assumeRole={() => null}
        resources={[request]}
      />
    </MemoryRouter>
  );

  const assumeBtn = screen.getByText('Assume Roles');
  expect(assumeBtn).toBeEnabled();
});

test('enabled assume button with no assume start date', () => {
  render(
    <MemoryRouter>
      <RequestList
        {...sample}
        attempt={{ status: 'success' }}
        assumeRole={() => null}
        resources={[
          { ...request, assumeStartTime: null, assumeStartTimeDuration: '' },
        ]}
      />
    </MemoryRouter>
  );

  const assumeBtn = screen.getByText('Assume Roles');
  expect(assumeBtn).toBeEnabled();
});

const request: AccessRequestWithFlags = {
  ...requestRoleApproved,
  assumeStartTime: new Date('2024-02-17T02:51:12.70087Z'),
  assumeStartTimeDuration: '24 hours from now',
  canAssume: true,
  isAssumed: false,
  ownRequest: true,
  isPromoted: false,
};
