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

import React, { useEffect, useState } from 'react';
import styled, { keyframes } from 'styled-components';
import { sharedStyles } from 'design/theme/themes/sharedStyles';

import { AssistViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/assist_pb';

import { Header } from 'teleport/Assist/Header';
import { ConversationHistory } from 'teleport/Assist/ConversationHistory';
import {
  AssistContextProvider,
  useAssist,
} from 'teleport/Assist/context/AssistContext';
import { ConversationList } from 'teleport/Assist/ConversationList';
import { useLayout } from 'teleport/Main/LayoutContext';
import { Settings } from 'teleport/Assist/Settings';
import { ErrorBanner, ErrorList } from 'teleport/Assist/ErrorBanner';
import { useUser } from 'teleport/User/UserContext';
import { LandingPage } from 'teleport/Assist/LandingPage';

const { dockedAssistWidth } = sharedStyles;

interface AssistProps {
  onClose: () => void;
}

const fadeIn = keyframes`
  from {
    opacity: 0;
  }

  to {
    opacity: 1;
  }
`;

const slideIn = keyframes`
  from {
    transform: translate3d(calc(var(--assist-width) + var(--assist-gutter)), 0, 0);
  }

  to {
    transform: translate3d(0, 0, 0);
  }
`;

function variables(props: { viewMode: AssistViewMode }) {
  switch (props.viewMode) {
    case AssistViewMode.POPUP:
      return {
        '--assist-gutter': '20px',
        '--assist-border-radius': '15px',
        '--assist-left': 'auto',
        '--assist-right': 'var(--assist-gutter)',
        '--assist-width': '550px',
        '--assist-height': '780px',
        '--assist-box-shadow': '0 30px 60px 0 rgba(0, 0, 0, 0.4)',
        '--assist-left-border': 'none',
        '--assist-header-height': '59px',
        '--assist-bottom-padding': '5px',
      };

    case AssistViewMode.POPUP_EXPANDED:
    case AssistViewMode.POPUP_EXPANDED_SIDEBAR_VISIBLE:
      return {
        '--assist-gutter': '20px',
        '--assist-border-radius': '15px',
        '--assist-left': 'auto',
        '--assist-right': 'var(--assist-gutter)',
        '--assist-width': '1100px',
        '--assist-height': 'calc(100vh - calc(var(--assist-gutter) * 2))',
        '--assist-box-shadow': '0 30px 60px 0 rgba(0, 0, 0, 0.4)',
        '--assist-left-border': 'none',
        '--assist-header-height': '59px',
        '--assist-bottom-padding': '5px',
      };

    case AssistViewMode.DOCKED:
      return {
        '--assist-gutter': '0',
        '--assist-border-radius': '0',
        '--assist-left': 'auto',
        '--assist-right': '0',
        '--assist-width': `${dockedAssistWidth}px`,
        '--assist-height': '100vh',
        '--assist-box-shadow': 'none',
        '--assist-left-border': '1px solid rgba(0, 0, 0, 0.1)',
        '--assist-header-height': '72px',
        '--assist-bottom-padding': '5px',
      };
  }
}

function sidebarVariables(props: {
  viewMode: AssistViewMode;
  sidebarVisible: boolean;
}) {
  switch (props.viewMode) {
    case AssistViewMode.POPUP:
      if (props.sidebarVisible) {
        return {
          '--conversation-width': '550px',
          '--conversation-list-width': '550px',
          '--conversation-list-margin': '0',
          '--command-input-width': '400px',
          '--conversation-list-display': 'flex',
          '--conversation-list-position': 'absolute',
        };
      }

      return {
        '--conversation-width': '555px',
        '--conversation-list-width': '550px',
        '--conversation-list-margin':
          'calc((var(--conversation-list-width) * -1) - 4px)',
        '--command-input-width': '400px',
        '--conversation-list-display': 'none',
        '--conversation-list-position': 'absolute',
      };

    case AssistViewMode.POPUP_EXPANDED:
    case AssistViewMode.POPUP_EXPANDED_SIDEBAR_VISIBLE:
      if (props.sidebarVisible) {
        return {
          '--conversation-list-margin': '0',
          '--conversation-width': '900px',
          '--conversation-list-width': '250px',
          '--command-input-width': '600px',
          '--conversation-list-display': 'flex',
          '--conversation-list-position': 'static',
        };
      }

      return {
        '--conversation-width': '1100px',
        '--conversation-list-width': '250px',
        '--conversation-list-margin':
          'calc((var(--conversation-list-width) * -1))',
        '--command-input-width': '700px',
        '--conversation-list-display': 'none',
        '--conversation-list-position': 'absolute',
      };

    case AssistViewMode.DOCKED:
      if (props.sidebarVisible) {
        return {
          '--conversation-width': `${dockedAssistWidth}px`,
          '--conversation-list-width': `${dockedAssistWidth}px`,
          '--conversation-list-margin': '0',
          '--command-input-width': '380px',
          '--conversation-list-display': 'flex',
          '--conversation-list-position': 'absolute',
        };
      }

      return {
        '--conversation-width': '525px',
        '--conversation-list-width': `${dockedAssistWidth}px`,
        '--conversation-list-margin':
          'calc((var(--conversation-list-width) * -1) - 1px)',
        '--command-input-width': '380px',
        '--conversation-list-display': 'none',
        '--conversation-list-position': 'absolute',
      };
  }
}

const Container = styled.div<{ docked: boolean }>`
  position: fixed;
  ${p =>
    p.docked
      ? `top: ${p.theme.topBarHeight[0]}px;
  @media screen and (min-width: ${p.theme.breakpoints.small}px) {
    top: ${p.theme.topBarHeight[1]}px;
  }
  @media screen and (min-width: ${p.theme.breakpoints.large}px) {
    top: ${p.theme.topBarHeight[2]}px;
  }
  `
      : 'top: 0;'}
  left: ${p => (p.docked ? 'auto' : '0')};
  right: 0;
  bottom: 0;
  opacity: 0;
  animation: forwards ${fadeIn} 0.3s ease-in-out;
  background: rgba(0, 0, 0, 0.5);
  z-index: ${p => (p.docked ? 2 : 100)};
  display: flex;
  justify-content: flex-end;
`;

const AssistContainer = styled.div<{ docked: boolean }>`
  ${variables};
  ${sidebarVariables};

  transform: translate3d(
    calc(var(--assist-width) + var(--assist-gutter)),
    0,
    0
  );
  animation: forwards ${slideIn} 0.5s cubic-bezier(0.33, 1, 0.68, 1);
  transition:
    width 0.5s cubic-bezier(0.33, 1, 0.68, 1),
    height 0.5s cubic-bezier(0.33, 1, 0.68, 1);
  background: ${p => p.theme.colors.levels.popout};
  border-radius: var(--assist-border-radius);
  box-shadow: var(--assist-box-shadow);
  position: absolute;
  width: var(--assist-width);
  max-height: calc(100vh - var(--assist-gutter) * 2);
  ${p =>
    p.docked
      ? `height: calc(100vh - ${p.theme.topBarHeight[0]}px);
  @media screen and (min-width: ${p.theme.breakpoints.small}px) {
    height: calc(100vh - ${p.theme.topBarHeight[1]}px);
  }
  @media screen and (min-width: ${p.theme.breakpoints.large}px) {
    height: calc(100vh - ${p.theme.topBarHeight[2]}px);
  }
  `
      : 'height: var(--assist-height);'}
  top: var(--assist-gutter);
  right: var(--assist-right);
  left: var(--assist-left);
  bottom: var(--assist-gutter);
  display: flex;
  flex-direction: column;
  border-left: var(--assist-left-border);
  overflow: hidden;
`;

const AssistConversation = styled.div`
  display: flex;
  flex-direction: column;
  width: var(--conversation-width);
  overflow-y: auto;
  height: 100%;
`;

const Content = styled.div`
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  display: flex;
  position: relative;
`;

export function Assist(props: AssistProps) {
  return (
    <AssistContextProvider>
      <AssistContent onClose={props.onClose} />
    </AssistContextProvider>
  );
}

let errorIndex = 0;

interface AssistError {
  message: string;
  index: number;
}

function getInitialErrors(conversations: { error?: string }) {
  const errors: AssistError[] = [];

  if (conversations.error) {
    errors.push({
      message: conversations.error,
      index: errorIndex++,
    });
  }

  return errors;
}

function AssistContent(props: AssistProps) {
  const { preferences } = useUser();
  const { conversations, sidebarVisible, toggleSidebar } = useAssist();

  const [errors, setErrors] = useState<{ message: string; index: number }[]>(
    () => getInitialErrors(conversations)
  );
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [debugMenuEnabled, setDebugMenuEnabled] = useState(false);

  const { hasDockedElement, setHasDockedElement } = useLayout();

  useEffect(() => {
    if (
      !hasDockedElement &&
      preferences.assist.viewMode === AssistViewMode.DOCKED
    ) {
      setHasDockedElement(true);
    }

    if (
      hasDockedElement &&
      preferences.assist.viewMode !== AssistViewMode.DOCKED
    ) {
      setHasDockedElement(false);
    }
  }, [hasDockedElement, preferences.assist.viewMode]);

  function handleClick(e: React.MouseEvent<HTMLElement>) {
    e.stopPropagation();
  }

  function handleConversationSelect() {
    if (!sidebarVisible) {
      return;
    }

    if (
      preferences.assist.viewMode === AssistViewMode.POPUP ||
      preferences.assist.viewMode === AssistViewMode.DOCKED
    ) {
      toggleSidebar(false);
    }
  }

  function handleClose() {
    props.onClose();
    setHasDockedElement(false);

    if (
      sidebarVisible &&
      preferences.assist.viewMode !==
        AssistViewMode.POPUP_EXPANDED_SIDEBAR_VISIBLE
    ) {
      toggleSidebar(false);
    }
  }

  function handleToggleSidebar() {
    toggleSidebar(!sidebarVisible);
  }

  function handleDebugMenuToggle(enabled: boolean) {
    if (process.env.NODE_ENV !== 'development') {
      throw new Error('Debug menu is only available in development mode');
    }

    setDebugMenuEnabled(enabled);
  }

  function handleError(message: string) {
    setErrors([...errors, { message, index: errorIndex++ }]);
  }

  function removeError(index: number) {
    setErrors(errors.filter(error => error.index !== index));
  }

  const errorList = errors.map(error => (
    <ErrorBanner key={error.index} onDismiss={() => removeError(error.index)}>
      {error.message}
    </ErrorBanner>
  ));

  const docked = preferences.assist.viewMode === AssistViewMode.DOCKED;

  return (
    <Container onClick={handleClose} docked={docked}>
      {settingsOpen && (
        <Settings
          onClose={() => setSettingsOpen(false)}
          debugMenuEnabled={debugMenuEnabled}
          onDebugMenuToggle={handleDebugMenuToggle}
        />
      )}

      <AssistContainer
        onClick={handleClick}
        viewMode={preferences.assist.viewMode}
        sidebarVisible={sidebarVisible}
        docked={docked}
      >
        <Header
          onClose={handleClose}
          onSettingsOpen={() => setSettingsOpen(true)}
          onToggleSidebar={handleToggleSidebar}
          sidebarVisible={sidebarVisible}
          viewMode={preferences.assist.viewMode}
          onError={handleError}
        />

        <ErrorList>{errorList}</ErrorList>

        <Content>
          {sidebarVisible && (
            <ConversationHistory
              onConversationSelect={handleConversationSelect}
              viewMode={preferences.assist.viewMode}
              onError={handleError}
            />
          )}
          <AssistConversation>
            {conversations.selectedId ? (
              <ConversationList viewMode={preferences.assist.viewMode} />
            ) : (
              <LandingPage />
            )}
          </AssistConversation>
        </Content>
      </AssistContainer>
    </Container>
  );
}
