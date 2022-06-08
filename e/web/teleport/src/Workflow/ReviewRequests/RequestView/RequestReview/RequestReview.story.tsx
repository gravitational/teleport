import React from 'react';
import RequestReview from './RequestReview';

export default {
  title: 'TeleportE/Workflow/RequestReview',
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
  return <RequestReview {...props} attempt={{ status: 'processing' }} />;
};

export const Failed = () => {
  return (
    <RequestReview
      {...props}
      attempt={{ status: 'failed', statusText: 'server error' }}
    />
  );
};

const props = {
  user: 'loggedInUsername',
  attempt: { status: '' as any },
  submitReview: () => null,
};
