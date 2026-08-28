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

import { render, screen, userEvent, within } from 'design/utils/testing';
import {
  makeEmptyAttempt,
  makeErrorAttempt,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';
import { RequestKind } from 'shared/services/accessRequests';

import {
  requestResourcePendingWithConstraints,
  requestRolePending,
} from '../../fixtures';
import { RequestView, RequestViewProps } from './RequestView';
import { RequestFlags } from './types';

const longTermResourceRequest = {
  ...requestResourcePendingWithConstraints,
  requestKind: RequestKind.LongTerm,
};

const sampleFlags: RequestFlags = {
  canAssume: false,
  isAssumed: false,
  canDelete: false,
  canReview: true,
  ownRequest: false,
  isPromoted: false,
};

const props: RequestViewProps = {
  user: 'loggedInUsername',
  fetchRequestAttempt: makeSuccessAttempt(requestRolePending),
  submitReviewAttempt: makeEmptyAttempt(),
  getFlags: () => sampleFlags,
  confirmDelete: false,
  toggleConfirmDelete: () => null,
  submitReview: () => null,
  assumeRole: () => null,
  fetchSuggestedAccessListsAttempt: makeSuccessAttempt([]),
  assumeRoleAttempt: makeEmptyAttempt(),
  assumeAccessList: () => null,
  deleteRequestAttempt: makeEmptyAttempt(),
  deleteRequest: () => null,
};

const reviewBoxText = `${props.user} - add a review`;

test('renders review box if user can review', async () => {
  render(<RequestView {...props} />);
  expect(screen.getByText(reviewBoxText)).toBeInTheDocument();
});

test('does not render review box if user cannot review', async () => {
  render(
    <RequestView
      {...props}
      getFlags={() => ({
        ...sampleFlags,
        canReview: false,
      })}
    />
  );
  expect(screen.queryByText(reviewBoxText)).not.toBeInTheDocument();
});

// When no Access List can be promoted to (e.g., reviewer doesn't own one that
// grants every requested resource, including implicitly-added ones), long-term
// approval is disabled, leaving only Reject. The disabled option explains why.
test('disables long-term approval and explains why when no Access List is suggested', async () => {
  const user = userEvent.setup();
  render(
    <RequestView
      {...props}
      fetchRequestAttempt={makeSuccessAttempt(longTermResourceRequest)}
      fetchSuggestedAccessListsAttempt={makeSuccessAttempt([])}
    />
  );

  expect(screen.getAllByText('aws-console-prod').length).toBeGreaterThan(0);

  expect(screen.getByRole('radio', { name: 'Reject request' })).toBeEnabled();

  const promote = screen.getByRole('radio', {
    name: /Approve long-term access via Access List/,
  });
  expect(promote).toBeDisabled();

  await user.hover(
    screen.getByText(
      'Approve long-term access via Access List with the requested resources'
    )
  );
  const tooltip = await screen.findByRole('tooltip');
  expect(
    within(tooltip).getByText(
      /you must own an Access List that grants every requested resource/i
    )
  ).toBeInTheDocument();
});

// A permission failure (the reviewer can't read the eligible Access Lists) is a
// distinct state from there being none, and gets its own message.
test('shows a permission-specific message when the reviewer cannot view eligible Access Lists', async () => {
  const user = userEvent.setup();
  const permissionError = Object.assign(
    new Error('access denied to perform action "read" on access list'),
    { response: { status: 403 } }
  );
  render(
    <RequestView
      {...props}
      fetchRequestAttempt={makeSuccessAttempt(longTermResourceRequest)}
      fetchSuggestedAccessListsAttempt={makeErrorAttempt(permissionError)}
    />
  );

  expect(
    screen.getByRole('radio', {
      name: /Approve long-term access via Access List/,
    })
  ).toBeDisabled();

  await user.hover(
    screen.getByText(
      'Approve long-term access via Access List with the requested resources'
    )
  );
  const tooltip = await screen.findByRole('tooltip');
  expect(
    within(tooltip).getByText(/you don't have permission to view/i)
  ).toBeInTheDocument();
});

// Non-permission errors stay visible so they aren't hidden behind a generic message.
test('surfaces non-permission fetch errors as-is', async () => {
  const user = userEvent.setup();
  render(
    <RequestView
      {...props}
      fetchRequestAttempt={makeSuccessAttempt(longTermResourceRequest)}
      fetchSuggestedAccessListsAttempt={makeErrorAttempt(
        new Error('backend exploded')
      )}
    />
  );

  await user.hover(
    screen.getByText(
      'Approve long-term access via Access List with the requested resources'
    )
  );
  const tooltip = await screen.findByRole('tooltip');
  expect(within(tooltip).getByText('backend exploded')).toBeInTheDocument();
});
