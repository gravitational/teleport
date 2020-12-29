import React from 'react';
import { requests } from '../fixtures';
import { RequestList } from './RequestList';

export default {
  title: 'Teleport/Workflow/RequestList',
};

export const Processing = () => {
  return <RequestList {...sample} attempt={{ status: 'processing' }} />;
};

export const Loaded = () => {
  return <RequestList {...sample} />;
};

export const Failed = () => {
  return (
    <RequestList
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  );
};

const sample = {
  attempt: {
    status: 'success' as any,
  },
  requests: requests,
  assumeRole: () => null,
};
