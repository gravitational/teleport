import React from 'react';

import { SessionWrapper } from '../WaitingRoom.story';

import RequestDenied from './RequestDenied';

export default {
  title: 'TeleportE/WaitingRoom/Denied',
};

export const WithReason = () => {
  return (
    <SessionWrapper>
      <RequestDenied {...sample} />
    </SessionWrapper>
  );
};

export const WithoutReason = () => {
  return (
    <SessionWrapper>
      <RequestDenied {...sample} reason={''} />
    </SessionWrapper>
  );
};

const sample = {
  reason: 'some reason for denying request',
};
