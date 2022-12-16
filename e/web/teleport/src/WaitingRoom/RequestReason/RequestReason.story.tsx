import React from 'react';

import { SessionWrapper } from '../WaitingRoom.story';

import { RequestReason } from './RequestReason';

export default {
  title: 'TeleportE/WaitingRoom/Reason',
};

export const Loaded = () => {
  return (
    <SessionWrapper>
      <RequestReason {...sample} />
    </SessionWrapper>
  );
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
      <RequestReason {...sample} attempt={attempt} />
    </SessionWrapper>
  );
};

export const Failed = () => {
  const attempt = {
    isProcessing: false,
    isFailed: true,
    isSuccess: false,
    message: 'some error message',
  };

  return (
    <SessionWrapper>
      <RequestReason {...sample} attempt={attempt} />
    </SessionWrapper>
  );
};

export const LoadedWithPrompt = () => {
  return (
    <SessionWrapper>
      <RequestReason
        {...sample}
        prompt={'Some custom prompt set by administrator'}
      />
    </SessionWrapper>
  );
};

const sample = {
  attempt: {
    isProcessing: false,
    isFailed: false,
    isSuccess: false,
    message: '',
  },
  prompt: '',
  reason: 'some reason',
  setReason: () => null,
  createRequest: () => Promise.resolve(null),
};
