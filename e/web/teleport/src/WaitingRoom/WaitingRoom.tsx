import React, { PropsWithChildren } from 'react';
import { Indicator } from 'design';
import { useStore } from 'shared/libs/stores';
import { AppVerticalSplit } from 'teleport/components/Layout';
import AjaxPoller from 'teleport/components/AjaxPoller';

import useTeleportE from 'e-teleport/useTeleportE';

import RequestReason from './RequestReason';
import RequestPending from './RequestPending';
import RequestDenied from './RequestDenied';
import RequestError from './RequestError';
import useWaitingRoom, { State } from './useWaitingRoom';

export const Container: React.FC<PropsWithChildren<Props>> = props => {
  const ctx = useTeleportE();
  useStore(ctx.storeAccessRequests);

  const state = useWaitingRoom(ctx);
  return <WaitingRoomComponent {...props} {...state} />;
};

export const WaitingRoomComponent: React.FC<
  PropsWithChildren<State & Partial<Props>>
> = props => {
  const {
    children,
    attempt,
    strategy,
    accessRequest,
    createRequest,
    refresh,
    checkerInterval = 5000,
  } = props;

  if (attempt.isProcessing) {
    return (
      <AppVerticalSplit
        style={{ alignItems: 'center', justifyContent: 'center' }}
      >
        <Indicator />
      </AppVerticalSplit>
    );
  }

  if (attempt.isFailed) {
    return <RequestError err={attempt.message} />;
  }

  // render access request
  if (accessRequest.state === 'APPLIED') {
    return <>{children}</>;
  }

  if (accessRequest.state === 'PENDING' || accessRequest.state === 'APPROVED') {
    return (
      <>
        <AjaxPoller time={checkerInterval} onFetch={refresh} />
        <RequestPending />
      </>
    );
  }

  if (accessRequest.state === 'DENIED') {
    return <RequestDenied reason={accessRequest.resolveReason} />;
  }

  // render strategy
  if (!strategy || strategy.type == 'optional') {
    return <>{children}</>;
  }

  if (strategy.type === 'reason' || strategy.type === 'always') {
    return (
      <RequestReason onCreateRequest={createRequest} prompt={strategy.prompt} />
    );
  }

  return null;
};

type Props = {
  checkerInterval?: number;
};
