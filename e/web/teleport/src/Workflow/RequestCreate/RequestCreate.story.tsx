import React from 'react';
import { RequestCreate } from './RequestCreate';

export default {
  title: 'TeleportE/Workflow/RequestCreate',
};

export const LoadedNoReviewers = () => {
  return <RequestCreate {...sample} reason="" reviewers={[]} />;
};

export const LoadedReasonRequired = () => {
  return <RequestCreate {...sample} reason="" />;
};

export const LoadedReasonUnrequired = () => {
  return <RequestCreate {...sample} reason="" requireReason={false} />;
};

export const Processing = () => {
  return <RequestCreate {...sample} attempt={{ status: 'processing' }} />;
};

export const Failed = () => {
  return (
    <RequestCreate
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  );
};

const sample = {
  attempt: {
    status: 'success' as any,
  },
  reason: 'Some reason for requesting access for specified role.',
  requireReason: true,
  setReason: () => null,
  roles: ['dev-a', 'dev-b', 'dev-c', 'dev-d'],
  reviewers: ['alice', 'bob', 'foo', 'bar'],
  createRequest: () => null,
  close: () => null,
};
