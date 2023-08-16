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

import styled from 'styled-components';

import { Box, Flex } from 'design';
import { TeleportLogoII } from 'design/assets/images/TeleportLogoII';
import cloudCity from 'design/assets/images/backgrounds/cloud-city.png';

import { OnboardFooter } from './OnboardFooter';

export const WelcomeWrapper = ({ children }) => {
  return (
    <OnboardWrapper>
      <Flex flexDirection="column" justifyContent="space-between" height="100%">
        <span>
          <WelcomeHeader>
            <TeleportLogoII />
          </WelcomeHeader>
          {children}
        </span>
        <OnboardFooter />
      </Flex>
    </OnboardWrapper>
  );
};

const OnboardWrapper = styled.div`
  position: absolute;
  width: 100vw;
  height: 100vh;
  top: 0;
  left: 0;
  overflow: hidden;
  // z-index -2 will place the image behind the black transparent/blur effect
  z-index: -2;

  background: url('${cloudCity}');
  -webkit-background-size: cover;
  -moz-background-size: cover;
  -o-background-size: cover;
  background-size: cover;

  // leveraging pseudo element for opacity/blur
  ::after {
    content: '';
    top: 0;
    left: 0;
    bottom: 0;
    right: 0;
    position: absolute;
    // z-index -1 will place the transparent/blur effect behind all other components on the page
    z-index: -1;

    background-color: black;
    opacity: 0.25;
    backdrop-filter: blur(17.5px);
  }
`;

const WelcomeHeader = styled(Box)`
  display: flex;
  flex-direction: column;
  align-items: center;
  margin: ${props => props.theme.space[4]}px 0;
`;
