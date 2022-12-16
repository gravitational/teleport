import React from 'react';
import { BroadcastChannel } from 'broadcast-channel';

import { SessionContextProvider } from 'teleport/WebSessionContext';
import { WebSession } from 'teleport/services/websession';

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
  return (
    <SessionWrapper>
      <WaitingRoom {...sample} attempt={attempt} />
    </SessionWrapper>
  );
};

export const Failed = () => {
  const attempt = {
    isProcessing: false,
    isFailed: true,
    isSuccess: false,
    message: 'some error',
  };
  return (
    <SessionWrapper>
      <WaitingRoom {...sample} attempt={attempt} />
    </SessionWrapper>
  );
};

export const Pending = () => {
  return (
    <SessionWrapper>
      <RequestPending />
    </SessionWrapper>
  );
};

export const PrivateKeyRequired = () => {
  return (
    <SessionWrapper>
      <WaitingRoom
        {...sample}
        privateKeyRequirement={{
          accessRequestId: 'request-id-1234',
          username: 'llama',
          clusterId: 'cluster-id-1234',
          authType: 'local',
        }}
      />
    </SessionWrapper>
  );
};

export const SessionWrapper = ({ children }: { children: JSX.Element }) => {
  const mockBcBroadcaster = new BroadcastChannel(
    'test'
  ) as unknown as globalThis.BroadcastChannel;
  const mockBcReceiver = new BroadcastChannel(
    'test'
  ) as unknown as globalThis.BroadcastChannel;
  const mockWebSession = new WebSession(mockBcBroadcaster, mockBcReceiver);
  mockWebSession.logout = () => null;
  return (
    <SessionContextProvider session={mockWebSession}>
      {children}
    </SessionContextProvider>
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
