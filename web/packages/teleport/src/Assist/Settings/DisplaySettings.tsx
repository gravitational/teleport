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

import React from 'react';
import styled from 'styled-components';

import { AssistViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/assist_pb';

import logoLight from 'teleport/Navigation/logoLight.svg';
import logoDark from 'teleport/Navigation/logoDark.svg';
import { Description, Title } from 'teleport/Assist/Settings/shared';

interface DisplaySettingsProps {
  viewMode: AssistViewMode;
  onChange: (viewMode: AssistViewMode) => void;
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
          active={props.viewMode === AssistViewMode.POPUP}
          onClick={() => props.onChange(AssistViewMode.POPUP)}
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
          active={props.viewMode === AssistViewMode.DOCKED}
          onClick={() => props.onChange(AssistViewMode.DOCKED)}
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
          active={props.viewMode === AssistViewMode.POPUP_EXPANDED}
          onClick={() => props.onChange(AssistViewMode.POPUP_EXPANDED)}
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
          active={
            props.viewMode === AssistViewMode.POPUP_EXPANDED_SIDEBAR_VISIBLE
          }
          onClick={() =>
            props.onChange(AssistViewMode.POPUP_EXPANDED_SIDEBAR_VISIBLE)
          }
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
