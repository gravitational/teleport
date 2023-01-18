import React from 'react';

import {
  requestRoleApproved,
  requestRoleDenied,
  requestRolePending,
  requestSearchPending,
  requestRoleEmpty,
} from '../../fixtures';

import { RequestView } from './RequestView';

export default {
  title: 'TeleportE/Workflow/RequestView',
};

export const LoadedSearchPending = () => {
  const flags = {
    ...sample.flags,
    canReview: true,
    canDelete: true,
  };
  return (
    <RequestView {...sample} request={requestSearchPending} flags={flags} />
  );
};

export const LoadedRolePending = () => {
  const flags = {
    ...sample.flags,
    canReview: true,
    canDelete: true,
  };
  return <RequestView {...sample} flags={flags} />;
};

export const LoadedRoleDenied = () => {
  const flags = {
    ...sample.flags,
    canDelete: true,
  };
  return <RequestView {...sample} request={requestRoleDenied} flags={flags} />;
};

export const LoadedRoleApproved = () => {
  const flags = {
    ...sample.flags,
    canDelete: true,
    canAssume: true,
  };
  return (
    <RequestView {...sample} request={requestRoleApproved} flags={flags} />
  );
};

export const LoadedRoleApprovedWithPrivateKeyRequired = () => {
  const flags = {
    ...sample.flags,
    canDelete: true,
    canAssume: true,
  };
  return (
    <RequestView
      {...sample}
      request={requestRoleApproved}
      flags={flags}
      privateKeyRequirement={{
        accessRequestId: 'request-id-1234',
        username: 'llama',
        clusterId: 'cluster-id-1234',
        authType: 'local',
      }}
    />
  );
};

export const LoadedEmpty = () => {
  const flags = {
    ...sample.flags,
    canAssume: true,
    isAssumed: true,
  };
  return <RequestView {...sample} request={requestRoleEmpty} flags={flags} />;
};

export const Processing = () => {
  return <RequestView {...sample} attempt={{ status: 'processing' }} />;
};

export const Failed = () => {
  return (
    <RequestView
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  );
};

const sample = {
  user: 'loggedInUsername',
  attempt: { status: 'success' as any },
  reviewAttempt: { status: '' as any },
  request: requestRolePending,
  flags: {
    canAssume: false,
    isAssumed: false,
    canDelete: false,
    canReview: false,
  },
  confirmDelete: false,
  toggleConfirmDelete: () => null,
  submitReview: () => null,
  deleteRequest: () => null,
  assumeRole: () => null,
};
