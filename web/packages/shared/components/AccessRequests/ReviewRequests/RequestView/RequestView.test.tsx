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

import { render, screen } from 'design/utils/testing';
import { makeEmptyAttempt, makeSuccessAttempt } from 'shared/hooks/useAsync';

import { requestRolePending } from '../../fixtures';
import { RequestView, RequestViewProps } from './RequestView';
import { RequestFlags } from './types';

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
