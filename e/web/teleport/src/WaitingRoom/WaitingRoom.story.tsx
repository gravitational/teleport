import React from 'react';

import { WaitingRoom } from './WaitingRoom';
import RequestPending from './RequestPending';

export default {
  title: 'TeleportE/WaitingRoom',
};

export const Processing = () => {
  const attempt = {
    isProcessing: true,
    isFailed: false,
    isSuccess: false,
    message: '',
  };
  return <WaitingRoom {...sample} attempt={attempt} />;
};

export const Failed = () => {
  const attempt = {
    isProcessing: false,
    isFailed: true,
    isSuccess: false,
    message: 'some error',
  };
  return <WaitingRoom {...sample} attempt={attempt} />;
};

export const Pending = () => {
  return <RequestPending />;
};

export const PrivateKeyRequired = () => {
  return (
    <WaitingRoom
      {...sample}
      privateKeyRequirement={{
        accessRequestId: 'request-id-1234',
        username: 'llama',
        clusterId: 'cluster-id-1234',
        authType: 'local',
      }}
    />
  );
};

const sample = {
  attempt: {
    isProcessing: false,
    isFailed: false,
    isSuccess: false,
    message: '',
  },
  strategy: {
    type: 'optional' as any,
    prompt: '',
  },
  accessRequest: null,
  createRequest: null,
  refresh: null,
  children: null,
  checkerInterval: 0,
  privateKeyRequirement: null,
  clearPrivateKeyRequirement: () => null,
};
