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

import { CtaEvent, userEventService } from 'teleport/services/userEvent';

export type Props = {
  children: React.ReactNode;
  noIcon?: boolean;
  event?: CtaEvent;
  [index: string]: any;
};

const salesUrl = 'https://goteleport.com/signup/enterprise/';

export function ButtonLockedFeature({
  children,
  noIcon = false,
  event,
  ...rest
}: Props) {
  function handleClick() {
    userEventService.captureCtaEvent(event);
    window.open(salesUrl, 'blank');
  }

  return (
    <StyledButton onClick={handleClick} {...rest}>
      <Flex alignItems="center">
        {!noIcon && <UnlockIcon />}
        {children}
      </Flex>
    </StyledButton>
  );
}

const StyledButton = styled(ButtonPrimary)`
  text-transform: none;
  width: 100%;
  padding-top: 12px;
  padding-bottom: 12px;
  font-size: 12px;
`;

const UnlockIcon = styled(Unlock)`
  color: inherit;
  font-weight: 500;
  font-size: 15px;
  margin-right: 4px;
`;
