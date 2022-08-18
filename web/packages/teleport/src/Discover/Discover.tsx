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

import React, { useState } from 'react';
import styled from 'styled-components';
import { Flex, Text, Box, Indicator } from 'design';
import { Danger } from 'design/Alert';
import * as Icons from 'design/Icon';

import { Prompt } from 'react-router-dom';

import useTeleport from 'teleport/useTeleport';
import getFeatures from 'teleport/features';
import { UserMenuNav } from 'teleport/components/UserMenuNav';
import * as main from 'teleport/Main';
import * as sideNav from 'teleport/SideNav';
import { TopBarContainer } from 'teleport/TopBar';
import { FeatureBox } from 'teleport/components/Layout';

import { useDiscover, State } from './useDiscover';

import { SelectResource } from './SelectResource';
import { DownloadScript } from './DownloadScript';
import { LoginTrait } from './LoginTrait';
import { TestConnection } from './TestConnection';
import { Finished } from './Finished';

import type { AgentKind } from './useDiscover';
import type { AgentStepComponent } from './types';

export const agentViews: Record<AgentKind, AgentStepComponent[]> = {
  app: [],
  db: [],
  desktop: [],
  kube: [],
  node: [SelectResource, DownloadScript, LoginTrait, TestConnection, Finished],
};

export default function Container() {
  const [features] = useState(() => getFeatures());
  const ctx = useTeleport();
  const state = useDiscover(ctx, features);

  return <Discover {...state} />;
}

export function Discover({
  initAttempt,
  userMenuItems,
  username,
  currentStep,
  selectedAgentKind = 'node',
  logout,
  onSelectResource,
  ...agentProps
}: State) {
  let AgentComponent;
  if (selectedAgentKind) {
    AgentComponent = agentViews[selectedAgentKind][currentStep];
  }

  return (
    <MainContainer>
      <Prompt
        message="Are you sure you want to exit the “Add New Resource” workflow? You’ll have to start from the beginning next time."
        when={currentStep > 0}
      />
      {initAttempt.status === 'processing' && (
        <main.StyledIndicator>
          <Indicator />
        </main.StyledIndicator>
      )}
      {initAttempt.status === 'failed' && (
        <Danger>{initAttempt.statusText}</Danger>
      )}
      {initAttempt.status === 'success' && (
        <>
          <SideNavAgentConnect currentStep={currentStep} />
          <main.HorizontalSplit>
            <TopBarContainer>
              <Text typography="h4" bold>
                Access Manager
              </Text>
              <UserMenuNav
                navItems={userMenuItems}
                logout={logout}
                username={username}
              />
            </TopBarContainer>
            <FeatureBox pt={4}>
              {AgentComponent && <AgentComponent {...agentProps} />}
            </FeatureBox>
          </main.HorizontalSplit>
        </>
      )}
    </MainContainer>
  );
}

function SideNavAgentConnect({ currentStep }) {
  const agentStepTitles: string[] = [
    'Select Resource Type',
    'Configure Resource',
    'Configure Role',
    'Test Connection',
    '',
  ];

  return (
    <StyledNav>
      <sideNav.Logo />
      <StyledNavContent>
        <Box
          border="1px solid rgba(255,255,255,0.1);"
          borderRadius="8px"
          css={{ backgroundColor: 'rgba(255,255,255,0.02);' }}
          p={4}
        >
          <Flex alignItems="center">
            <Flex
              borderRadius={5}
              alignItems="center"
              justifyContent="center"
              bg="secondary.main"
              height="30px"
              width="30px"
              mr={2}
            >
              <Icons.Database />
            </Flex>
            <Text bold>Resource Connection</Text>
          </Flex>
          <Box ml={4} mt={4}>
            {agentStepTitles.map((stepTitle, index) => {
              let className = '';
              if (currentStep > index) {
                className = 'checked';
              } else if (currentStep === index) {
                className = 'active';
              }

              // All flows will have a finished step that
              // does not have a title.
              if (!stepTitle) {
                return null;
              }

              return (
                <StepsContainer className={className} key={stepTitle}>
                  <Bullet />
                  {stepTitle}
                </StepsContainer>
              );
            })}
          </Box>
        </Box>
      </StyledNavContent>
    </StyledNav>
  );
}

const Bullet = styled.span`
  height: 14px;
  width: 14px;
  border: 1px solid #9b9b9b;
  border-radius: 50%;
  margin-right: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
`;

const StepsContainer = styled(Text)`
  display: flex;
  align-items: center;
  color: ${props => props.theme.colors.text.secondary};
  margin-bottom: 8px;

  &.active,
  &.checked {
    color: inherit;
  }

  &.active ${Bullet}, &.checked ${Bullet} {
    border-color: ${props => props.theme.colors.secondary.main};
    background: ${props => props.theme.colors.secondary.main};
  }

  &.active ${Bullet} {
    :before {
      content: '';
      height: 8px;
      width: 8px;
      border-radius: 50%;
      border: 2px solid ${props => props.theme.colors.primary.main};
    }
  }

  &.checked ${Bullet} {
    :before {
      content: '✓';
    }
  }
`;

const StyledNav = styled(sideNav.Nav)`
  min-width: 350px;
  width: 350px;
`;

const StyledNavContent = styled(sideNav.Content)`
  padding: 20px 32px 32px 32px;
`;

// TODO (lisa) we should look into reducing this width.
// Any smaller than this will produce a double stacked horizontal scrollbar
// making navigation harder.
//
// Our SelectResource component is the widest and can use some space
// tightening. Also look into shrinking the side nav if possible.
const MainContainer = styled(main.MainContainer)`
  min-width: 1460px;
`;
