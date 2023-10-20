/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import styled, { useTheme } from 'styled-components';

import { Flex, Link, Text } from 'design';

export const OnboardFooter = () => {
  const theme = useTheme();
  return (
    <StyledFooter>
      <StyledContent>
        <Text typography={theme.typography.paragraph2}>
          &copy; Gravitational, Inc. All Rights Reserved
        </Text>
        <StyledLink href={'https://goteleport.com/legal/tos/'} target="_blank">
          Terms of Service
        </StyledLink>
        <StyledLink
          href={'https://goteleport.com/legal/privacy/'}
          target="_blank"
        >
          Privacy Policy
        </StyledLink>
      </StyledContent>
    </StyledFooter>
  );
};

const StyledContent = styled(Flex)`
  justify-content: center;
  width: 100%;
  gap: 50px;

  @media screen and (max-width: 800px) {
    flex-direction: column-reverse;
    text-align: center;
    gap: 10px;
  }
`;

const StyledFooter = styled('footer')`
  padding-bottom: ${props => props.theme.space[4]}px;
  width: 100%;
  // we don't want to leverage theme.colors.text.main here
  // because the footer is always on a dark image background
  color: white;
`;

const StyledLink = styled(Link)`
  // we don't want to leverage theme.colors.text.main here
  // because the footer is always on a dark image background
  color: white;
  text-decoration: none;

  &:hover,
  &:active,
  &:focus {
    // we don't want to leverage theme.colors.text.muted here
    // because the footer is always on a dark image background
    color: rgba(255, 255, 255, 0.54);
  }
`;
