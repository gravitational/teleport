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

import { Box, Text } from 'design';

export const Container = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
`;

interface CommandBoxProps {
  header?: React.ReactNode;
  // hasTtl when true means that the command has an expiry TTL, otherwise the command
  // is valid forever.
  hasTtl?: boolean;
}

export function CommandBox({
  header,
  children,
  hasTtl = true,
}: React.PropsWithChildren<CommandBoxProps>) {
  return (
    <Container p={3} borderRadius={3} mb={3}>
      {header || <Text bold>Command</Text>}
      <Box mt={3} mb={3}>
        {children}
      </Box>
      {hasTtl && `This script is valid for 4 hours.`}
    </Container>
  );
}
