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

import React, { useEffect } from 'react';
import styled, { keyframes } from 'styled-components';

import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import { Header } from 'teleport/Assist/Header';
import { ConversationHistory } from 'teleport/Assist/ConversationHistory';
import { AssistContextProvider } from 'teleport/Assist/context/AssistContext';
import { ConversationList } from 'teleport/Assist/ConversationList';
import { KeysEnum } from 'teleport/services/localStorage';
import { useLayout } from 'teleport/Main/LayoutContext';

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
    case AssistViewMode.Collapsed:
    case AssistViewMode.CollapsedSidebarVisible:
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

    case AssistViewMode.Expanded:
    case AssistViewMode.ExpandedSidebarVisible:
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

    case AssistViewMode.Docked:
    case AssistViewMode.DockedSidebarVisible:
      return {
        '--assist-gutter': '0',
        '--assist-border-radius': '0',
        '--assist-left': 'auto',
        '--assist-right': '0',
        '--assist-width': '520px',
        '--assist-height': '100vh',
        '--assist-box-shadow': 'none',
        '--assist-left-border': '1px solid rgba(0, 0, 0, 0.1)',
        '--assist-header-height': '72px',
        '--assist-bottom-padding': '5px',
      };
  }
}

function sidebarVariables(props: { viewMode: AssistViewMode }) {
  switch (props.viewMode) {
    case AssistViewMode.Collapsed:
      return {
        '--conversation-width': '555px',
        '--conversation-list-width': '550px',
        '--conversation-list-margin':
          'calc((var(--conversation-list-width) * -1) - 4px)',
        '--command-input-width': '400px',
        '--conversation-list-display': 'none',
        '--conversation-list-position': 'absolute',
      };

    case AssistViewMode.CollapsedSidebarVisible:
      return {
        '--conversation-width': '550px',
        '--conversation-list-width': '550px',
        '--conversation-list-margin': '0',
        '--command-input-width': '400px',
        '--conversation-list-display': 'flex',
        '--conversation-list-position': 'absolute',
      };

    case AssistViewMode.Expanded:
      return {
        '--conversation-width': '1100px',
        '--conversation-list-width': '250px',
        '--conversation-list-margin':
          'calc((var(--conversation-list-width) * -1))',
        '--command-input-width': '700px',
        '--conversation-list-display': 'none',
        '--conversation-list-position': 'absolute',
      };

    case AssistViewMode.ExpandedSidebarVisible:
      return {
        '--conversation-list-margin': '0',
        '--conversation-width': '850px',
        '--conversation-list-width': '250px',
        '--command-input-width': '600px',
        '--conversation-list-display': 'flex',
        '--conversation-list-position': 'static',
      };

    case AssistViewMode.Docked:
      return {
        '--conversation-width': '525px',
        '--conversation-list-width': '520px',
        '--conversation-list-margin':
          'calc((var(--conversation-list-width) * -1) - 1px)',
        '--command-input-width': '380px',
        '--conversation-list-display': 'none',
        '--conversation-list-position': 'absolute',
      };

    case AssistViewMode.DockedSidebarVisible:
      return {
        '--conversation-width': '520px',
        '--conversation-list-width': '520px',
        '--conversation-list-margin': '0',
        '--command-input-width': '380px',
        '--conversation-list-display': 'flex',
        '--conversation-list-position': 'absolute',
      };
  }
}

const Container = styled.div<{ docked: boolean }>`
  position: fixed;
  top: 0;
  left: ${p => (p.docked ? 'auto' : '0')};
  right: 0;
  bottom: 0;
  opacity: 0;
  animation: forwards ${fadeIn} 0.3s ease-in-out;
  background: rgba(0, 0, 0, 0.5);
  z-index: 1000;
  display: flex;
  justify-content: flex-end;
`;

const AssistContainer = styled.div`
  ${variables};
  ${sidebarVariables};

  transform: translate3d(
    calc(var(--assist-width) + var(--assist-gutter)),
    0,
    0
  );
  animation: forwards ${slideIn} 0.5s cubic-bezier(0.33, 1, 0.68, 1);
  transition: width 0.5s cubic-bezier(0.33, 1, 0.68, 1),
    height 0.5s cubic-bezier(0.33, 1, 0.68, 1);
  background: ${p => p.theme.colors.levels.popout};
  border-radius: var(--assist-border-radius);
  box-shadow: var(--assist-box-shadow);
  position: absolute;
  width: var(--assist-width);
  max-height: calc(100vh - var(--assist-gutter) * 2);
  height: var(--assist-height);
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

const AssistContent = styled.div`
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  display: flex;
`;

export enum AssistViewMode {
  Collapsed = 'collapsed',
  CollapsedSidebarVisible = 'collapsed-sidebar-visible',
  Expanded = 'expanded',
  ExpandedSidebarVisible = 'expanded-sidebar-visible',
  Docked = 'docked',
  DockedSidebarVisible = 'docked-sidebar-visible',
}

function isDocked(viewMode: AssistViewMode) {
  return (
    viewMode === AssistViewMode.Docked ||
    viewMode === AssistViewMode.DockedSidebarVisible
  );
}

function isSidebarVisible(viewMode: AssistViewMode) {
  return (
    viewMode === AssistViewMode.CollapsedSidebarVisible ||
    viewMode === AssistViewMode.ExpandedSidebarVisible ||
    viewMode === AssistViewMode.DockedSidebarVisible
  );
}

export function Assist(props: AssistProps) {
  const [viewMode, setViewMode] = useLocalStorage(
    KeysEnum.ASSIST_VIEW_MODE,
    AssistViewMode.Collapsed
  );

  const { hasDockedElement, setHasDockedElement } = useLayout();

  useEffect(() => {
    if (!hasDockedElement && isDocked(viewMode)) {
      setHasDockedElement(true);
    }

    if (hasDockedElement && !isDocked(viewMode)) {
      setHasDockedElement(false);
    }
  }, [hasDockedElement, viewMode]);

  function handleClick(e: React.MouseEvent<HTMLElement>) {
    e.stopPropagation();
  }

  function handleConversationSelect() {
    if (viewMode === AssistViewMode.CollapsedSidebarVisible) {
      setViewMode(AssistViewMode.Collapsed);
    }

    if (viewMode === AssistViewMode.DockedSidebarVisible) {
      setViewMode(AssistViewMode.Docked);
    }
  }

  function handleExpand() {
    switch (viewMode) {
      case AssistViewMode.Collapsed:
        setViewMode(AssistViewMode.Expanded);

        break;

      case AssistViewMode.CollapsedSidebarVisible:
        setViewMode(AssistViewMode.ExpandedSidebarVisible);

        break;

      case AssistViewMode.Expanded:
      case AssistViewMode.ExpandedSidebarVisible:
        setViewMode(AssistViewMode.Collapsed);

        break;
    }
  }

  function handleDocking() {
    switch (viewMode) {
      case AssistViewMode.Collapsed:
      case AssistViewMode.CollapsedSidebarVisible:
      case AssistViewMode.Expanded:
      case AssistViewMode.ExpandedSidebarVisible:
        setViewMode(AssistViewMode.Docked);

        break;

      case AssistViewMode.Docked:
      case AssistViewMode.DockedSidebarVisible:
        setViewMode(AssistViewMode.Collapsed);

        break;
    }
  }

  function handleViewModeChange(viewMode: AssistViewMode) {
    setViewMode(viewMode);
  }

  function handleClose() {
    props.onClose();
    setHasDockedElement(false);

    if (viewMode === AssistViewMode.CollapsedSidebarVisible) {
      setViewMode(AssistViewMode.Collapsed);
    }

    if (viewMode === AssistViewMode.DockedSidebarVisible) {
      setViewMode(AssistViewMode.Docked);
    }
  }

  return (
    <AssistContextProvider>
      <Container onClick={handleClose} docked={isDocked(viewMode)}>
        <AssistContainer onClick={handleClick} viewMode={viewMode}>
          <Header
            onClose={handleClose}
            onExpand={handleExpand}
            onViewModeChange={handleViewModeChange}
            onDocking={handleDocking}
            viewMode={viewMode}
          />
          <AssistContent>
            {isSidebarVisible(viewMode) && (
              <ConversationHistory
                onConversationSelect={handleConversationSelect}
                viewMode={viewMode}
              />
            )}
            <AssistConversation>
              <ConversationList viewMode={viewMode} />
            </AssistConversation>
          </AssistContent>
        </AssistContainer>
      </Container>
    </AssistContextProvider>
  );
}
