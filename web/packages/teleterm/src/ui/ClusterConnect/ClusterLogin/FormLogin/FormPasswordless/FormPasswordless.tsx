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

import styled from 'styled-components';

import { Box, ButtonText, Flex, Text } from 'design';
import { ArrowForward, Key } from 'design/Icon';

import type { Props } from '../FormLogin';

export const FormPasswordless = ({
  loginAttempt,
  onLoginWithPasswordless,
  autoFocus = false,
}: Props) => (
  <Box data-testid="passwordless">
    <StyledPaswordlessBtn
      size="large"
      py={2}
      px={3}
      border={1}
      borderRadius={2}
      width="100%"
      onClick={onLoginWithPasswordless}
      disabled={loginAttempt.status === 'processing'}
      autoFocus={autoFocus}
    >
      <Flex alignItems="center" justifyContent="space-between">
        <Flex alignItems="center">
          <Key mr={3} size="medium" />
          <Box>
            <Text mb={1}>Passwordless</Text>
            <Text typography="subtitle3" color="text.slightlyMuted">
              Follow the prompts
            </Text>
          </Box>
        </Flex>
        <ArrowForward size="medium" />
      </Flex>
    </StyledPaswordlessBtn>
  </Box>
);

const StyledPaswordlessBtn = styled(ButtonText)`
  display: block;
  text-align: left;
  border: 1px solid ${({ theme }) => theme.colors.buttons.border.border};

  &:hover,
  &:focus {
    background: ${({ theme }) => theme.colors.buttons.border.hover};
    text-decoration: none;
  }

  &:active {
    background: ${({ theme }) => theme.colors.buttons.border.active};
  }

  &[disabled] {
    pointer-events: none;
    background: ${({ theme }) => theme.colors.buttons.bgDisabled};
  }
`;
