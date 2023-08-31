/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import { ButtonPrimary } from 'design/Button';
import { Unlock } from 'design/Icon';
import Flex from 'design/Flex';

import { getSalesURL } from 'teleport/services/sales';

import { CtaEvent, userEventService } from 'teleport/services/userEvent';
import useTeleport from 'teleport/useTeleport';

import cfg from 'teleport/config';

export type Props = {
  children: React.ReactNode;
  noIcon?: boolean;
  event?: CtaEvent;
  [index: string]: any;
};

export function ButtonLockedFeature({
  children,
  noIcon = false,
  event,
  ...rest
}: Props) {
  const ctx = useTeleport();
  const version = ctx.storeUser.state.cluster.authVersion;
  const isEnterprise = ctx.isEnterprise;

  const isUsageBased = cfg.isUsageBasedBilling;

  function handleClick() {
    if (isEnterprise) {
      userEventService.captureCtaEvent(event);
    }
  }

  return (
    <ButtonPrimary
      as="a"
      target="blank"
      href={getSalesURL(version, isEnterprise, isUsageBased, event)}
      onClick={handleClick}
      py="12px"
      width="100%"
      style={{ textTransform: 'none' }}
      rel="noreferrer"
      {...rest}
    >
      <Flex alignItems="center">
        {!noIcon && <UnlockIcon size="medium" data-testid="locked-icon" />}
        {children}
      </Flex>
    </ButtonPrimary>
  );
}

const UnlockIcon = styled(Unlock)`
  color: inherit;
  font-weight: 500;
  font-size: 15px;
  margin-right: 10px;
`;
