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
import { NavLink } from 'react-router-dom';
import styled from 'styled-components';
import { Flex, ButtonPrimary, Text, Box, Indicator } from 'design';
import TopNavUserMenu from 'design/TopNav/TopNavUserMenu';
import { Danger } from 'design/Alert';
import * as Icons from 'design/Icon';
import { MenuItem, MenuItemIcon } from 'shared/components/MenuAction';

import * as main from 'teleport/Main';
import * as sideNav from 'teleport/SideNav';
import { FeatureBox } from 'teleport/components/Layout';

import cfg from 'teleport/config';

import { useDiscoverContext } from './discoverContextProvider';
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
  const ctx = useDiscoverContext();
  const state = useDiscover(ctx);

  return <Discover {...state} />;
}

export function Discover({
  initAttempt,
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
            <TopBar onLogout={logout} username={username} />
            <FeatureBox pt={4}>
              {AgentComponent && <AgentComponent {...agentProps} />}
            </FeatureBox>
          </main.HorizontalSplit>
        </>
      )}
    </MainContainer>
  );
}

function TopBar(props: { onLogout: VoidFunction; username: string }) {
  const [open, setOpen] = React.useState(false);

  function showMenu() {
    setOpen(true);
  }

  function closeMenu() {
    setOpen(false);
  }

  function logout() {
    closeMenu();
    props.onLogout();
  }

  return (
    <Flex
      alignItems="center"
      justifyContent="space-between"
      height="56px"
      pl={5}
      borderColor="primary.main"
      css={{ borderBottomWidth: '1px', borderBottomStyle: 'solid' }}
    >
      <Text typography="h4" bold>
        Access Manager
      </Text>
      <TopNavUserMenu
        menuListCss={() => `
          width: 250px;
        `}
        open={open}
        onShow={showMenu}
        onClose={closeMenu}
        user={props.username}
      >
        <MenuItem as={NavLink} to={cfg.routes.root}>
          <MenuItemIcon as={Icons.Home} mr="2" />
          Dashboard
        </MenuItem>
        <MenuItem>
          <ButtonPrimary my={3} block onClick={logout}>
            Sign Out
          </ButtonPrimary>
        </MenuItem>
      </TopNavUserMenu>
    </Flex>
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
