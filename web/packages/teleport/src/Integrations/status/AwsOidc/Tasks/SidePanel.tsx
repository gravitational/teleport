/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';

import { Box, ButtonIcon, Flex } from 'design';
import { Cross } from 'design/Icon';

export const SidePanel = ({
  onClose,
  header,
  disabled = false,
  children,
}: PropsWithChildren & {
  onClose: () => void;
  header?: React.ReactNode;
  disabled?: boolean;
}) => {
  return (
    <Flex width="500px" flexDirection="column">
      <Flex
        alignItems="center"
        mb={3}
        justifyContent="space-between"
        maxWidth="500px"
        borderBottom={1}
        borderColor="levels.surface"
        py={1}
        px={4}
      >
        {header}
        <ButtonIcon onClick={onClose} disabled={disabled}>
          <Cross size="medium" />
        </ButtonIcon>
      </Flex>
      <ScrollContent px={4}>{children}</ScrollContent>
    </Flex>
  );
};

const ScrollContent = styled(Box)`
  max-height: calc(
    100vh - ${props => props.theme.space[9]}px -
      ${props => props.theme.topBarHeight[1]}px
  );
  overflow: auto;
`;
