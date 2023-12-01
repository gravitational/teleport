/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
