/**
 * Copyright 2022 Gravitational, Inc.
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
import { Text, Flex, ButtonText, Box } from 'design';
import { Key, ArrowForward } from 'design/Icon';

import type { Props } from '../FormLogin';

export const FormPasswordless = ({
  loginAttempt,
  onLoginWithPasswordless,
  autoFocus = false,
}: Props) => (
  <Box data-testid="passwordless">
    <StyledPaswordlessBtn
      py={2}
      px={3}
      border={1}
      borderRadius={2}
      borderColor="text.muted"
      width="100%"
      onClick={onLoginWithPasswordless}
      disabled={loginAttempt.status === 'processing'}
      autoFocus={autoFocus}
    >
      <Flex alignItems="center" justifyContent="space-between">
        <Flex alignItems="center">
          <Key mr={3} fontSize={16} />
          <Box>
            <Text typography="h6">Passwordless</Text>
            <Text fontSize={1} color="text.slightlyMuted">
              Follow the prompts
            </Text>
          </Box>
        </Flex>
        <ArrowForward fontSize={16} />
      </Flex>
    </StyledPaswordlessBtn>
  </Box>
);

const StyledPaswordlessBtn = styled(ButtonText)`
  display: block;
  text-align: left;
  border: 1px solid ${({ theme }) => theme.colors.text.muted};

  &:hover,
  &:active,
  &:focus {
    border-color: ${({ theme }) => theme.colors.action.active};
    text-decoration: none;
  }

  &[disabled] {
    pointer-events: none;
    opacity: 0.7;
  }
`;
