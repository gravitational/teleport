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

import {
  makeEmptyAttempt,
  makeErrorAttempt,
  makeProcessingAttempt,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';

import { requestRolePending } from '../../../fixtures';
import RequestReview, { RequestReviewProps } from './RequestReview';

export default {
  title: 'Shared/AccessRequests/RequestReview',
  decorators: [
    Story => (
      <div style={{ backgroundColor: '#222C59', padding: '40px' }}>
        <Story />
      </div>
    ),
  ],
};

export const Loaded = () => {
  return <RequestReview {...props} />;
};

export const Processing = () => {
  return (
    <RequestReview {...props} submitReviewAttempt={makeProcessingAttempt()} />
  );
};

export const Failed = () => {
  return (
    <RequestReview
      {...props}
      submitReviewAttempt={makeErrorAttempt(new Error('server error'))}
    />
  );
};

const props: RequestReviewProps = {
  user: 'loggedInUsername',
  submitReviewAttempt: makeEmptyAttempt(),
  submitReview: () => null,
  shortTermDuration: '12 hours',
  request: requestRolePending,
  fetchSuggestedAccessListsAttempt: makeSuccessAttempt([]),
};
