/*
Copyright 2022 Gravitational, Inc.

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

import { Box, Flex, Link, Text } from 'design';
import { Cross, Info, Warning } from 'design/Icon';

import { CaptureEvent } from 'teleport/services/userEvent/types';
import { userEventService } from 'teleport/services/userEvent';

export type Severity = 'info' | 'warning' | 'danger';

export type Props = {
  message: string;
  severity: Severity;
  id: string;
  link?: string;
  onClose: (id: string) => void;
};

export function Banner({
  id,
  message = '',
  severity = 'info',
  link = '',
  onClose,
}: Props) {
  const icon = {
    info: <Info mr={3} fontSize="3" role="icon" />,
    warning: <Info mr={3} fontSize="3" role="icon" />,
    danger: <Warning mr={3} fontSize="3" role="icon" />,
  }[severity];

  const isValidTeleportLink = (link: string) => {
    try {
      const url = new URL(link);
      return url.hostname === 'goteleport.com';
    } catch {
      return false;
    }
  };

  let backgroundColor;
  if (severity === 'danger') {
    backgroundColor = 'error.main';
  } else if (severity === 'warning') {
    backgroundColor = 'warning.main';
  } else {
    backgroundColor = 'info';
  }

  return (
    <Box bg={backgroundColor} p={1} pl={2}>
      <Flex alignItems="center">
        {icon}
        {isValidTeleportLink(link) ? (
          <Link
            href={link}
            target="_blank"
            color="text.primaryInverse"
            style={{ fontWeight: 'bold' }}
            onClick={() =>
              userEventService.captureUserEvent({
                event: CaptureEvent.BannerClickEvent,
                alert: id,
              })
            }
          >
            {message}
          </Link>
        ) : (
          <Text bold>{message}</Text>
        )}
        <CloseButton
          onClick={() => {
            onClose(id);
          }}
        >
          <Cross />
        </CloseButton>
      </Flex>
    </Box>
  );
}

const CloseButton = styled.button`
  background: none;
  border: 1px solid transparent;
  box-sizing: border-box;
  cursor: pointer;
  display: flex;
  margin-left: auto;
  padding: 0.5rem;

  :hover {
    background-color: rgb(255, 255, 255, 0.1);
  }
  :focus {
    border: 1px solid rgb(255, 255, 255, 0.1);
  }
`;
