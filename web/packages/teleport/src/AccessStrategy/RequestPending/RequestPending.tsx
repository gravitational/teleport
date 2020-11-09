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
import session from 'teleport/services/session';
import { Text, ButtonLink, Flex } from 'design';
import { ShieldCheck } from 'design/Icon';
import Dialog, { DialogContent, DialogFooter } from 'design/Dialog';

export default function RequestPending() {
  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '500px',
        width: '100%',
        textAlign: 'center',
      })}
      open={true}
    >
      <DialogContent alignItems="center">
        <ShieldCheck fontSize={40} mb={3} />
        <Text mb={1} bold caps>
          Your access is being authorized
        </Text>
        <Text mb={4}>
          Please wait while an administrator authorizes your access.
        </Text>
        <StyledProgressBar height="16px" value={100} mb={4}>
          <Bar />
        </StyledProgressBar>
        <Text>One moment...</Text>
      </DialogContent>
      <DialogFooter>
        <ButtonLink onClick={() => session.logout()}>
          Logout of Account
        </ButtonLink>
      </DialogFooter>
    </Dialog>
  );
}

const StyledProgressBar = styled(Flex)`
  align-items: center;
  flex-shrink: 0;
  width: 80%;
  background-color: black;
  border-radius: 12px;
  > span {
    border-radius: 12px;
    ${({ value }) => `
      height: 100%;
      width: ${value}%;
      transition: 1s width;
    `}
  }
`;

const Bar = styled.span`
  text-align: center;
  margin: 0;
  padding: 0;
  display: block;
  height: 100%;
  border-top-right-radius: 8px;
  border-bottom-right-radius: 8px;
  border-top-left-radius: 20px;
  border-bottom-left-radius: 20px;
  background-color: #b097ff;

  box-shadow: inset 0 2px 9px rgba(255, 255, 255, 0.3),
    inset 0 -2px 6px rgba(0, 0, 0, 0.4);
  position: relative;
  overflow: hidden;
  width: 118px;

  ::after {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    bottom: 0;
    right: 0;
    background-image: linear-gradient(
      -45deg,
      rgba(255, 255, 255, 0.2) 25%,
      transparent 25%,
      transparent 50%,
      rgba(255, 255, 255, 0.2) 50%,
      rgba(255, 255, 255, 0.2) 75%,
      transparent 75%,
      transparent
    );
    z-index: 1;
    background-size: 50px 50px;
    animation: move 2s linear infinite;
    border-top-right-radius: 8px;
    border-bottom-right-radius: 8px;
    border-top-left-radius: 20px;
    border-bottom-left-radius: 20px;
    overflow: hidden;
    animation: animate 3s linear infinite;
  }

  @keyframes animate {
    0% {
      background-position: 0 0;
    }
    100% {
      background-position: 50px 50px;
    }
  }
`;
