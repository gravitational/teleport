import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { requestPending, requestDenied, requestApproved } from '../fixtures';
import { RequestList } from './RequestList';

export default {
  title: 'TeleportE/Workflow/RequestList',
};

export const Processing = () => {
  return <RequestList {...sample} attempt={{ status: 'processing' }} />;
};

export const Loaded = () => {
  return (
    <MemoryRouter>
      <RequestList {...sample} />
    </MemoryRouter>
  );
};

export const Failed = () => {
  return (
    <RequestList
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  );
};

const requestRows = [
  {
    ...requestPending,
    canAssume: false,
    isAssumed: false,
  },
  {
    ...requestDenied,
    canAssume: false,
    isAssumed: false,
  },
  {
    ...requestApproved,
    canAssume: true,
    isAssumed: false,
  },
  {
    ...requestApproved,
    canAssume: true,
    isAssumed: true,
  },
];

const sample = {
  attempt: {
    status: 'success' as any,
  },
  requests: requestRows,
  assumeRole: () => null,
};
