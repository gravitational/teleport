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

import { Box, Flex } from 'design';

import { LogoHero } from 'teleport/components/LogoHero';

import { OnboardFooter } from './OnboardFooter';

export const WelcomeWrapper = ({ children }) => {
  return (
    <OnboardWrapper>
      <Flex flexDirection="column" justifyContent="space-between" height="100%">
        {/* Flexing column here to prevent margin collapse
        between WelcomeHeader and chidlren */}
        <Flex flexDirection="column">
          <WelcomeHeader>
            <LogoHero my="12px" />
          </WelcomeHeader>
          {children}
        </Flex>
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
  background: ${props => props.theme.colors.levels.sunken};
`;

const WelcomeHeader = styled(Box)`
  display: flex;
  flex-direction: column;
  align-items: center;
  margin: ${props => props.theme.space[4]}px 0;
`;
