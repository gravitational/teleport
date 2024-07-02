/**
 * Copyright 2023 Gravitational, Inc.
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

import logoDark from 'design/assets/images/enterprise-dark.svg';
import logoLight from 'design/assets/images/enterprise-light.svg';

import { ViewMode } from 'teleport/Assist/types';
import { Description, Title } from 'teleport/Assist/Settings/shared';

interface DisplaySettingsProps {
  viewMode: ViewMode;
  onChange: (viewMode: ViewMode) => void;
}

const ViewModes = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
`;

const ViewModeExample = styled.div`
  width: 200px;
  height: 100px;
  box-shadow: 0 2px 5px 0 rgba(0, 0, 0, 0.2);
  border-radius: 5px;
  position: relative;
  overflow: hidden;
  display: flex;
`;

const DimmedBackground = styled.div`
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.3);
`;

const ViewModeContainer = styled.div`
  color: ${p =>
    p.active
      ? p.theme.colors.buttons.primary.default
      : p.theme.colors.text.main};
  font-weight: ${p => (p.active ? 'bold' : 'normal')};
  cursor: pointer;

  ${ViewModeExample} {
    border: 2px solid
      ${p =>
        p.active ? p.theme.colors.buttons.primary.default : 'transparent'};
  }

  &:hover ${ViewModeExample} {
    border: 2px solid ${p => p.theme.colors.buttons.primary.default};
  }
`;

const Assist = styled.div`
  background: ${p => p.theme.colors.levels.popout};
  box-shadow: 0 5px 10px 0 rgba(0, 0, 0, 0.3);
  position: absolute;
  overflow: hidden;
`;

const Chat = styled.div`
  display: flex;
  flex-direction: column;
  padding: 5px;
`;

const Message = styled.div`
  height: 6px;
  flex: 0 0 6px;
  margin-bottom: 5px;
  background: ${p =>
    p.author === 'teleport'
      ? p.theme.colors.levels.popout
      : p.theme.colors.buttons.primary.default};
  align-self: ${p => (p.author === 'teleport' ? 'flex-start' : 'flex-end')};
  border-radius: 5px;
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const PopupAssist = styled(Assist)`
  top: 10px;
  right: 10px;
  width: 60px;
  height: 60px;
  border-radius: 7px;
`;

const ExpandedAssist = styled(Assist)`
  top: 10px;
  right: 10px;
  width: 120px;
  height: 80px;
  border-radius: 7px;
  display: flex;

  ${Chat} {
    flex: 1;
  }
`;

const DockedAssist = styled(Assist)`
  position: static;
  box-shadow: none;
  border-left: 1px solid ${p => p.theme.colors.spotBackground[1]};
  flex: 0 0 60px;
`;

const Conversation = styled.div`
  height: 7px;
  margin-bottom: 5px;
  background: ${p => p.theme.colors.spotBackground[1]};
  border-radius: 3px;
`;

const Sidebar = styled.div`
  flex: 0 0 30px;
  padding: 5px;
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;

const Page = styled.div`
  display: flex;
  background: ${p => p.theme.colors.levels.sunken};
  height: inherit;
  flex: 1;
`;

const PageNavigation = styled.div`
  background: ${p => p.theme.colors.levels.surface};
  flex: 0 0 45px;
  height: inherit;
  box-shadow:
    0px 2px 1px -1px rgba(0, 0, 0, 0.2),
    0px 1px 1px rgba(0, 0, 0, 0.14),
    0px 1px 3px rgba(0, 0, 0, 0.12);
`;

const NavigationLogo = styled.div`
  background: url(${p => (p.theme.type === 'light' ? logoLight : logoDark)})
    no-repeat;
  background-size: contain;
  width: 36px;
  height: 32px;
  margin-top: 5px;
  margin-left: 5px;
`;

const PageContent = styled.div`
  flex: 1;
`;

const PageHeader = styled.div`
  height: 15px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

const PageTitle = styled.div`
  font-size: 8px;
  line-height: 10px;
  margin-bottom: 5px;
  color: ${p => p.theme.colors.text.main};
  font-weight: 400;
`;

const PagePadding = styled.div`
  padding: 5px;
`;

const PageTable = styled.div`
  background: ${p => p.theme.colors.levels.surface};
  width: 100%;
  height: 40px;
  border-radius: 3px;
  box-shadow:
    0px 2px 1px -1px rgba(0, 0, 0, 0.1),
    0px 1px 1px rgba(0, 0, 0, 0.07),
    0px 1px 3px rgba(0, 0, 0, 0.06);
`;

function MockPage() {
  return (
    <Page>
      <PageNavigation>
        <NavigationLogo />
      </PageNavigation>

      <PageContent>
        <PageHeader />

        <PagePadding>
          <PageTitle>Servers</PageTitle>

          <PageTable></PageTable>
        </PagePadding>
      </PageContent>
    </Page>
  );
}

function MockChat() {
  return (
    <Chat>
      <Message author="teleport" style={{ width: '25px' }} />
      <Message author="user" style={{ width: '20px' }} />
      <Message author="teleport" style={{ width: '35px' }} />
      <Message author="teleport" style={{ width: '15px' }} />
      <Message author="user" style={{ width: '30px' }} />
      <Message author="teleport" style={{ width: '25px' }} />
      <Message author="user" style={{ width: '27px' }} />
      <Message author="teleport" style={{ width: '32px' }} />
    </Chat>
  );
}

export function DisplaySettings(props: DisplaySettingsProps) {
  return (
    <div>
      <Title>View Mode</Title>

      <Description>
        Choose how you want Assist to be shown on your screen.
      </Description>

      <ViewModes>
        <ViewModeContainer
          active={props.viewMode === ViewMode.Popup}
          onClick={() => props.onChange(ViewMode.Popup)}
        >
          <ViewModeExample>
            <DimmedBackground />

            <MockPage />

            <PopupAssist>
              <MockChat />
            </PopupAssist>
          </ViewModeExample>
          Popup
        </ViewModeContainer>
        <ViewModeContainer
          active={props.viewMode === ViewMode.Docked}
          onClick={() => props.onChange(ViewMode.Docked)}
        >
          <ViewModeExample>
            <MockPage />

            <DockedAssist>
              <MockChat />
            </DockedAssist>
          </ViewModeExample>
          Docked
        </ViewModeContainer>
        <ViewModeContainer
          active={props.viewMode === ViewMode.PopupExpanded}
          onClick={() => props.onChange(ViewMode.PopupExpanded)}
        >
          <ViewModeExample>
            <DimmedBackground />

            <MockPage />

            <ExpandedAssist>
              <MockChat />
            </ExpandedAssist>
          </ViewModeExample>
          Expanded popup
        </ViewModeContainer>
        <ViewModeContainer
          active={props.viewMode === ViewMode.PopupExpandedSidebarVisible}
          onClick={() => props.onChange(ViewMode.PopupExpandedSidebarVisible)}
        >
          <ViewModeExample>
            <DimmedBackground />

            <MockPage />

            <ExpandedAssist>
              <Sidebar>
                <Conversation />
                <Conversation />
                <Conversation />
                <Conversation />
              </Sidebar>

              <MockChat />
            </ExpandedAssist>
          </ViewModeExample>
          Expanded popup with sidebar
        </ViewModeContainer>
      </ViewModes>
    </div>
  );
}
