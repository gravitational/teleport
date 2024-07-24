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
    info: <Info mr={3} size="medium" role="icon" />,
    warning: <Info mr={3} size="medium" role="icon" />,
    danger: <Warning mr={3} size="medium" role="icon" />,
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
          <Cross size="medium" />
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

  &:hover {
    background-color: rgb(255, 255, 255, 0.1);
  }
  &:focus {
    border: 1px solid rgb(255, 255, 255, 0.1);
  }
`;
