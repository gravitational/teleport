import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import {
  requestRolePending,
  requestRoleDenied,
  requestRoleApproved,
  requestSearchPending,
} from '../../fixtures';
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
    ...requestSearchPending,
    canAssume: false,
    isAssumed: false,
  },
  {
    ...requestRolePending,
    canAssume: false,
    isAssumed: false,
  },
  {
    ...requestRoleDenied,
    canAssume: false,
    isAssumed: false,
  },
  {
    ...requestRoleApproved,
    canAssume: true,
    isAssumed: false,
  },
  {
    ...requestRoleApproved,
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
