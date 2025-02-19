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
import { PropsWithChildren, ReactNode } from 'react';
import styled from 'styled-components';

import { ButtonIcon, Flex } from 'design';
import { Cross } from 'design/Icon';

export const SidePanel = ({
  onClose,
  header,
  footer,
  disabled = false,
  children,
}: PropsWithChildren & {
  onClose: () => void;
  header?: ReactNode;
  footer?: ReactNode;
  disabled?: boolean;
}) => {
  return (
    <Container
      width="500px"
      flexDirection="column"
      borderLeft={1}
      borderColor="levels.surface"
    >
      <Flex alignItems="center" justifyContent="space-between" my={3} px={4}>
        {header}
        <ButtonIcon onClick={onClose} disabled={disabled}>
          <Cross size="medium" />
        </ButtonIcon>
      </Flex>
      <Flex
        flexDirection="column"
        px={4}
        style={{
          flex: 1,
          overflow: 'auto',
          height: '100%',
        }}
      >
        {children}
      </Flex>
      <Flex borderTop={1} borderColor="levels.surface" py={3} px={4}>
        {footer}
      </Flex>
    </Container>
  );
};

const Container = styled(Flex)`
  display: flex;
  flex-direction: column;

  height: calc(100vh - ${props => props.theme.topBarHeight[1]}px);
`;
