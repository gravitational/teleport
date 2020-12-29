import React from 'react';
import { RequestCreate } from './RequestCreate';

export default {
  title: 'Teleport/Workflow/RequestCreate',
};

export const LoadedReasonRequired = () => {
  return <RequestCreate {...sample} reason="" selectedRoles={[]} />;
};

export const LoadedReasonUnrequired = () => {
  return (
    <RequestCreate
      {...sample}
      reason=""
      selectedRoles={[]}
      requireReason={false}
    />
  );
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
  selectedRoles: [
    { value: 'dev-a', label: 'dev-a' },
    { value: 'dev-c', label: 'dev-c' },
  ],
  setSelectedRoles: () => null,
  roles: ['dev-a', 'dev-b', 'dev-c', 'dev-d'],
  createRequest: () => null,
  close: () => null,
};
