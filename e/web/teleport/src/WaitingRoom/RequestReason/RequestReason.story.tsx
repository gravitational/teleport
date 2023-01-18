import React from 'react';

import { RequestReason } from './RequestReason';

export default {
  title: 'TeleportE/WaitingRoom/Reason',
};

export const Loaded = () => {
  return <RequestReason {...sample} />;
};

export const Processing = () => {
  const attempt = {
    isProcessing: true,
    isFailed: false,
    isSuccess: false,
    message: '',
  };
  return <RequestReason {...sample} attempt={attempt} />;
};

export const Failed = () => {
  const attempt = {
    isProcessing: false,
    isFailed: true,
    isSuccess: false,
    message: 'some error message',
  };

  return <RequestReason {...sample} attempt={attempt} />;
};

export const LoadedWithPrompt = () => {
  return (
    <RequestReason
      {...sample}
      prompt={'Some custom prompt set by administrator'}
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
  prompt: '',
  reason: 'some reason',
  setReason: () => null,
  createRequest: () => Promise.resolve(null),
};
