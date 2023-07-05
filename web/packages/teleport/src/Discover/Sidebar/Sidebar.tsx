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

import { Box, Flex, Text } from 'design';

import * as Icons from 'design/Icon';

import styled from 'styled-components';

import * as sideNav from 'teleport/SideNav';

import { Resource, View } from '../flow';

import { StepList } from './StepList';

interface SidebarProps {
  currentStep: number;
  selectedResource: Resource;
  views: View[];
}

const StyledNav = styled(sideNav.Nav)`
  min-width: var(--sidebar-width);
  width: var(--sidebar-width);
`;

const StyledNavContent = styled(sideNav.Content)`
  padding: 0 20px;
`;

export function Sidebar(props: SidebarProps) {
  let content;
  if (props.views) {
    content = <StepList views={props.views} currentStep={props.currentStep} />;
  }

  return (
    <StyledNav>
      <sideNav.Logo />
      <StyledNavContent>
        <Box
          border="1px solid rgba(255,255,255,0.1);"
          borderRadius="8px"
          css={{ backgroundColor: 'rgba(255,255,255,0.02);' }}
          p={3}
        >
          <Flex alignItems="center">
            <Flex
              borderRadius={5}
              alignItems="center"
              justifyContent="center"
              bg="brand.main"
              height="30px"
              width="30px"
              mr={2}
            >
              {props.selectedResource ? (
                props.selectedResource.icon
              ) : (
                <Icons.Server />
              )}
            </Flex>
            <Text bold>Add New Resource</Text>
          </Flex>

          <Box mt={3}>{content}</Box>
        </Box>
      </StyledNavContent>
    </StyledNav>
  );
}
