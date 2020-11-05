/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';
import { Indicator } from 'design';
import { AppVerticalSplit } from 'teleport/components/Layout';
import AjaxPoller from 'teleport/components/AjaxPoller';
import RequestReason from './RequestReason';
import RequestPending from './RequestPending';
import RequestDenied from './RequestDenied';
import RequestError from './RequestError';
import useAccessStrategy, { State } from './useAccessStrategy';

export default function Container(props: Props) {
  const state = useAccessStrategy();
  return <AccessStrategy {...props} {...state} />;
}

export function AccessStrategy(props: State & Props) {
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
      <StyledIndicator>
        <Indicator />
      </StyledIndicator>
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
    return <RequestDenied reason={accessRequest.reason} />;
  }

  // render strategy
  if (strategy.type == 'optional') {
    return <>{children}</>;
  }

  if (strategy.type === 'reason') {
    return (
      <RequestReason onCreateRequest={createRequest} prompt={strategy.prompt} />
    );
  }

  return null;
}

const StyledIndicator = styled(AppVerticalSplit)`
  align-items: center;
  justify-content: center;
`;

type Props = {
  children: React.ReactNode;
  checkerInterval?: number;
};
