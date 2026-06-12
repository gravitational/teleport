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

import { fireEvent, render, screen, userEvent } from 'design/utils/testing';
import { RequestState } from 'shared/services/accessRequests';

import { dryRunResponse } from '../../fixtures';
import { useSpecifiableFields } from '../useSpecifiableFields';
import {
  RequestCheckoutWithSlider as RequestCheckoutComp,
  RequestCheckoutWithSliderProps,
} from './RequestCheckout';

test('adding a reviewer and then removing it afterwards when there are no suggested reviewers', async () => {
  render(<RequestCheckout />);

  // Test init renders no reviewers.
  let reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(0);

  // Add a reviewer
  await userEvent.click(screen.getByRole('button', { name: 'Add' }));
  await userEvent.type(
    screen.getByText(/type or select a name/i),
    'llama{enter}'
  );
  await userEvent.click(screen.getByRole('button', { name: 'Done' }));

  reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(1);
  expect(reviewers.childNodes[0]).toHaveTextContent('llama');

  // Remove by clicking on x button.
  fireEvent.click(reviewers.childNodes[0].lastChild);
  reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(0);
});

test('adding a reviewer and removing a suggested reviewer does not remove it from the suggested section', async () => {
  render(<RequestCheckout reviewers={['llama']} />);

  // Initially, all suggested reviewers are pre-selected.
  let reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(1);
  expect(reviewers.childNodes[0]).toHaveTextContent('llama');

  // Add another reviewer.
  await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
  await userEvent.type(
    screen.getByText(/type or select a name/i),
    'alpaca{enter}'
  );
  await userEvent.click(screen.getByRole('button', { name: 'Done' }));

  reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(2);
  expect(reviewers.childNodes[0]).toHaveTextContent('llama');
  expect(reviewers.childNodes[1]).toHaveTextContent('alpaca');

  // Remove a suggested reviewer by typing the name.
  await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
  await userEvent.type(
    screen.getByText(/type or select a name/i),
    'llama{enter}'
  );
  await userEvent.click(screen.getByRole('button', { name: 'Done' }));

  reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(1);
  expect(reviewers.childNodes[0]).toHaveTextContent('alpaca');

  // Suggested reviewer should still be rendered in the dropdown.
  await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
  await userEvent.click(screen.getByTitle(/llama/i));
  await userEvent.click(screen.getByRole('button', { name: 'Done' }));

  reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(2);
  expect(reviewers.childNodes[0]).toHaveTextContent('alpaca');
  expect(reviewers.childNodes[1]).toHaveTextContent('llama');
});

test('changing access duration, also changes pending TTL options and the request lifetime text', () => {
  jest.useFakeTimers().setSystemTime(dryRunResponse.created);
  render(<RequestCheckout />);

  const infoBtn = screen.getByTestId('additional-info-btn');

  // Init state.
  expect(screen.queryByText(/start time/i)).not.toBeInTheDocument();
  expect(screen.getByText(/access duration/i)).toBeInTheDocument();
  expect(screen.getAllByText(/2 days/i)).toHaveLength(1);

  // Expand the additional info box where the access lifetime
  // gets displayed.
  fireEvent.click(infoBtn);
  expect(screen.getByText(/Access Request Lifetime/i)).toBeInTheDocument();

  // Test that all three fields display the same time info (max duration, pending, and lifetime)
  expect(screen.getAllByText(/2 days/i)).toHaveLength(3);

  // Changing the "access duration" to a shorter time
  // should also update pending and lifetime text
  fireEvent.keyDown(screen.getAllByText(/2 days/i)[0], { key: 'ArrowDown' });
  fireEvent.click(screen.getByText(/1 day/i));
  expect(screen.getAllByText(/1 day/i)).toHaveLength(3);
});

const RequestCheckout = ({ reviewers = [] }: { reviewers?: string[] }) => {
  const specifiableProps = useSpecifiableFields();

  if (!specifiableProps.dryRunResponse) {
    const request = {
      ...dryRunResponse,
      reviewers: reviewers.map(r => ({ name: r, state: '' as RequestState })),
    };
    specifiableProps.onDryRunChange(request);
  }

  return (
    <div>
      <RequestCheckoutComp
        {...props}
        isResourceRequest={true}
        fetchResourceRequestRolesAttempt={{ status: 'success' }}
        {...specifiableProps}
      />
    </div>
  );
};

const props: RequestCheckoutWithSliderProps = {
  createAttempt: { status: '' },
  fetchResourceRequestRolesAttempt: { status: '' },
  isResourceRequest: false,
  requireReason: true,
  reasonPrompts: [],
  selectedReviewers: [],
  setSelectedReviewers: () => null,
  createRequest: () => null,
  pendingAccessRequests: [],
  clearAttempt: () => null,
  onClose: () => null,
  toggleResource: () => null,
  reset: () => null,
  transitionState: 'entered',
  numRequestedResources: 4,
  resourceRequestRoles: ['admin', 'access', 'developer'],
  selectedResourceRequestRoles: ['admin', 'access'],
  setSelectedResourceRequestRoles: () => null,
  fetchStatus: 'loaded',
  maxDuration: { value: 0, label: '12 hours' },
  onMaxDurationChange: () => null,
  maxDurationOptions: [],
  pendingRequestTtlOptions: [],
  pendingRequestTtl: { value: 0, label: '1 hour' },
  setPendingRequestTtl: () => null,
  dryRunResponse: null,
  startTime: null,
  onStartTimeChange: () => null,
  fetchKubeNamespaces: () => null,
  updateNamespacesForKubeCluster: () => null,
};
