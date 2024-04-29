import React from 'react';

import {
  makeSuccessAttempt,
  makeEmptyAttempt,
  makeProcessingAttempt,
  makeErrorAttempt,
} from 'shared/hooks/useAsync';

import { requestRolePending } from '../../../fixtures';

import RequestReview, { RequestReviewProps } from './RequestReview';

export default {
  title: 'TeleportE/AccessRequests/RequestReview',
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
