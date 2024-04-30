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

import { useState } from 'react';
import { render, screen, userEvent, fireEvent } from 'design/utils/testing';

import { Option } from 'shared/components/Select';

import { dryRunResponse } from '../../fixtures';

import { ReviewerOption } from './types';

import {
  RequestCheckout as RequestCheckoutComp,
  RequestCheckoutProps,
} from './RequestCheckout';

test('start with no suggested reviewers', async () => {
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

test('start with suggested reviewers', async () => {
  render(<RequestCheckout reviewers={['llama']} />);

  // Test init renders reviewers.
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

test('assume start time + additional info access request lifetime', () => {
  jest.useFakeTimers().setSystemTime(dryRunResponse.created);
  render(<RequestCheckout />);

  const infoBtn = screen.getByTestId('additional-info-btn');

  // Init state.
  expect(screen.queryByText(/start time/i)).not.toBeInTheDocument();
  expect(screen.getByText(/access duration/i)).toBeInTheDocument();
  expect(screen.getAllByText(/2 days/i)).toHaveLength(1);
  const calendarBtn = screen.getByText(/immediately/i);
  fireEvent.click(calendarBtn);

  // Expand the additional info box where the access lifetime
  // gets displayed.
  fireEvent.click(infoBtn);
  expect(screen.getByText(/Access Request Lifetime/i)).toBeInTheDocument();
  expect(screen.getAllByText(/2 days/i)).toHaveLength(2);

  // Changing the "access duration" to a shorter time
  // should reduce the "access lifetime".
  fireEvent.keyDown(screen.getAllByText(/2 days/i)[0], { key: 'ArrowDown' });
  fireEvent.click(screen.getByText(/1 day/i));
  expect(screen.getAllByText(/1 day/i)).toHaveLength(2);
});

const RequestCheckout = ({ reviewers = [] }: { reviewers?: string[] }) => {
  const [selectedReviewers, setSelectedReviewers] = useState<ReviewerOption[]>(
    () => reviewers.map(r => ({ label: r, value: r, isSelected: true }))
  );
  const [maxDuration, setMaxDuration] = useState<Option<number>>();

  return (
    <div>
      <RequestCheckoutComp
        {...props}
        reviewers={reviewers}
        selectedReviewers={selectedReviewers}
        setSelectedReviewers={setSelectedReviewers}
        isResourceRequest={true}
        fetchResourceRequestRolesAttempt={{ status: 'success' }}
        maxDuration={maxDuration}
        setMaxDuration={setMaxDuration}
      />
    </div>
  );
};

const props: RequestCheckoutProps = {
  createAttempt: { status: '' },
  fetchResourceRequestRolesAttempt: { status: '' },
  isResourceRequest: false,
  requireReason: true,
  reviewers: [],
  selectedReviewers: [],
  setSelectedReviewers: () => null,
  createRequest: () => null,
  data: [],
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
  setMaxDuration: () => null,
  requestTTL: { value: 0, label: '1 hour' },
  setRequestTTL: () => null,
  dryRunResponse,
};
