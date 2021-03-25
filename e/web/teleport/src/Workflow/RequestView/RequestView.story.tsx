import React from 'react';
import { RequestView } from './RequestView';
import {
  requestApproved,
  requestDenied,
  requestPending,
  requestEmpty,
} from '../fixtures';

export default {
  title: 'TeleportE/Workflow/RequestView',
};

export const LoadedPending = () => {
  const flags = {
    ...sample.flags,
    canReview: true,
    canDelete: true,
  };
  return <RequestView {...sample} flags={flags} />;
};

export const LoadedDenied = () => {
  const flags = {
    ...sample.flags,
    canDelete: true,
  };
  return <RequestView {...sample} request={requestDenied} flags={flags} />;
};

export const LoadedApproved = () => {
  const flags = {
    ...sample.flags,
    canDelete: true,
    canAssume: true,
  };
  return <RequestView {...sample} request={requestApproved} flags={flags} />;
};

export const LoadedEmpty = () => {
  const flags = {
    ...sample.flags,
    canAssume: true,
    isAssumed: true,
  };
  return <RequestView {...sample} request={requestEmpty} flags={flags} />;
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
  reviewAttempt: { status: '' as any},
  request: requestPending,
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
