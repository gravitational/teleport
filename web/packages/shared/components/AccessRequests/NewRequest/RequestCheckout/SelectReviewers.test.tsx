/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { render, screen, userEvent, within } from 'design/utils/testing';
import type { AccessRequestReviewer } from 'shared/services/accessRequests';

import { SelectReviewers } from './SelectReviewers';
import type { ReviewerOption } from './types';

test('renders duplicate display names with distinguishable usernames', async () => {
  const user = userEvent.setup();
  render(
    <SelectReviewersHarness
      reviewers={reviewersWithDuplicateDisplays}
      initialSelectedReviewers={[selectedReviewer('reviewer-one')]}
    />
  );

  const selectedReviewers = screen.getByTestId('reviewers');
  expect(within(selectedReviewers).getByText('Shared Reviewer')).toBeVisible();
  expect(within(selectedReviewers).getByText('reviewer-one')).toBeVisible();
  expect(
    within(selectedReviewers).getByText('reviewer-one@example.com')
  ).toBeVisible();

  await user.click(screen.getByRole('button', { name: 'Edit' }));

  const firstReviewer = screen.getByRole('option', {
    name: /Shared Reviewer.*reviewer-one/,
  });
  expect(firstReviewer).toBeVisible();
  expect(within(firstReviewer).getByText('reviewer-one')).toHaveStyle({
    color: 'inherit',
  });
  expect(
    within(firstReviewer).getByText('reviewer-one@example.com')
  ).toHaveStyle({ color: 'inherit' });
  expect(
    screen.getByRole('option', {
      name: /Shared Reviewer.*reviewer-two/,
    })
  ).toBeVisible();

  await user.click(firstReviewer);
  expect(
    within(selectedReviewers).queryByText('reviewer-one')
  ).not.toBeInTheDocument();

  const deselectedReviewer = screen.getByRole('option', {
    name: /Shared Reviewer.*reviewer-one/,
  });
  await user.click(deselectedReviewer);

  expect(within(selectedReviewers).getByText('Shared Reviewer')).toBeVisible();
  expect(within(selectedReviewers).getByText('reviewer-one')).toBeVisible();
  expect(
    within(selectedReviewers).getByText('reviewer-one@example.com')
  ).toBeVisible();
});

test('filters suggested reviewers by primary display, secondary display, and username', async () => {
  const user = userEvent.setup();
  render(<SelectReviewersHarness reviewers={reviewersWithDistinctDisplays} />);

  await user.click(screen.getByRole('button', { name: 'Add' }));
  const input = screen.getByRole('combobox');

  await user.type(input, 'Alice Jones');
  expect(
    screen.getByRole('option', { name: /Alice Jones.*reviewer-one/ })
  ).toBeVisible();
  expect(
    screen.queryByRole('option', { name: /Bob Smith.*reviewer-two/ })
  ).not.toBeInTheDocument();

  await user.clear(input);
  await user.type(input, 'alice@example.com');
  expect(
    screen.getByRole('option', { name: /Alice Jones.*reviewer-one/ })
  ).toBeVisible();
  expect(
    screen.queryByRole('option', { name: /Bob Smith.*reviewer-two/ })
  ).not.toBeInTheDocument();

  await user.clear(input);
  await user.type(input, 'reviewer-two');
  expect(
    screen.getByRole('option', { name: /Bob Smith.*reviewer-two/ })
  ).toBeVisible();
  expect(
    screen.queryByRole('option', { name: /Alice Jones.*reviewer-one/ })
  ).not.toBeInTheDocument();
});

test('shows the create label for a manually entered reviewer', async () => {
  const user = userEvent.setup();
  render(<SelectReviewersHarness reviewers={[]} />);

  await user.click(screen.getByRole('button', { name: 'Add' }));
  await user.type(screen.getByRole('combobox'), 'manual-reviewer');

  expect(
    screen.getByRole('option', { name: 'Create "manual-reviewer"' })
  ).toBeVisible();
});

test('updates selected and suggested displays when reviewer data changes', async () => {
  const user = userEvent.setup();
  const { rerender } = render(
    <SelectReviewersHarness
      reviewers={[reviewer('reviewer-one')]}
      initialSelectedReviewers={[selectedReviewer('reviewer-one')]}
    />
  );

  const selectedReviewers = screen.getByTestId('reviewers');
  expect(within(selectedReviewers).getByText('reviewer-one')).toBeVisible();
  expect(
    within(selectedReviewers).queryByText('Alice Jones')
  ).not.toBeInTheDocument();

  rerender(
    <SelectReviewersHarness
      reviewers={[reviewer('reviewer-one', 'Alice Jones')]}
      initialSelectedReviewers={[selectedReviewer('reviewer-one')]}
    />
  );

  expect(within(selectedReviewers).getByText('Alice Jones')).toBeVisible();
  expect(within(selectedReviewers).getByText('reviewer-one')).toBeVisible();

  await user.click(screen.getByRole('button', { name: 'Edit' }));
  expect(
    screen.getByRole('option', { name: /Alice Jones.*reviewer-one/ })
  ).toBeVisible();
});

function SelectReviewersHarness({
  reviewers,
  initialSelectedReviewers = [],
}: {
  reviewers: AccessRequestReviewer[];
  initialSelectedReviewers?: ReviewerOption[];
}) {
  const [selectedReviewers, setSelectedReviewers] = useState(
    initialSelectedReviewers
  );

  return (
    <SelectReviewers
      reviewers={reviewers}
      selectedReviewers={selectedReviewers}
      setSelectedReviewers={setSelectedReviewers}
    />
  );
}

function reviewer(
  name: string,
  primary?: string,
  secondary?: string
): AccessRequestReviewer {
  return {
    name,
    display: primary || secondary ? { primary, secondary } : undefined,
    state: 'PENDING',
  };
}

function selectedReviewer(value: string): ReviewerOption {
  return { value, label: value, isSelected: true };
}

const reviewersWithDuplicateDisplays = [
  reviewer('reviewer-one', 'Shared Reviewer', 'reviewer-one@example.com'),
  reviewer('reviewer-two', 'Shared Reviewer', 'reviewer-two@example.com'),
];

const reviewersWithDistinctDisplays = [
  reviewer('reviewer-one', 'Alice Jones', 'alice@example.com'),
  reviewer('reviewer-two', 'Bob Smith'),
];
