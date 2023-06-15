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

import {
  ChevronRightIcon,
  CloseIcon,
  ExpandIcon,
  PopupIcon,
  SidebarIcon,
} from 'design/SVGIcon';

import { AssistViewMode } from 'teleport/Assist/Assist';
import { useAssist } from 'teleport/Assist/context/AssistContext';

interface HeaderProps {
  viewMode: AssistViewMode;
  onClose: () => void;
  onExpand: () => void;
  onDocking: () => void;
  onViewModeChange: (viewMode: AssistViewMode) => void;
}

const Container = styled.header`
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 15px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[0]};
  user-select: none;
  flex: 0 0 var(--assist-header-height);
  box-sizing: border-box;
`;

const Icon = styled.div`
  border-radius: 7px;
  width: 38px;
  height: 38px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: 0.2s ease-in-out opacity;

  svg {
    transform: ${p => (p.rotated ? 'rotate(180deg)' : 'none')};
  }

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
  }
`;

const Icons = styled.section`
  display: flex;
  align-items: center;
  flex: 0 0 100px;
`;

const Title = styled.h2`
  margin: 0;
  font-size: 16px;
  white-space: nowrap;
  text-overflow: ellipsis;
  overflow: hidden;
`;

function isExpanded(viewMode: AssistViewMode) {
  return (
    viewMode === AssistViewMode.Expanded ||
    viewMode === AssistViewMode.ExpandedSidebarVisible
  );
}

function toggleSidebarVisible(viewMode: AssistViewMode) {
  switch (viewMode) {
    case AssistViewMode.Expanded:
      return AssistViewMode.ExpandedSidebarVisible;

    case AssistViewMode.ExpandedSidebarVisible:
      return AssistViewMode.Expanded;

    case AssistViewMode.Docked:
      return AssistViewMode.DockedSidebarVisible;

    case AssistViewMode.DockedSidebarVisible:
      return AssistViewMode.Docked;

    case AssistViewMode.Collapsed:
      return AssistViewMode.CollapsedSidebarVisible;

    case AssistViewMode.CollapsedSidebarVisible:
      return AssistViewMode.Collapsed;
  }
}

function isDocked(viewMode: AssistViewMode) {
  return (
    viewMode === AssistViewMode.Docked ||
    viewMode === AssistViewMode.DockedSidebarVisible
  );
}

export function Header(props: HeaderProps) {
  const {
    conversations: { selectedId, data },
  } = useAssist();

  const title = selectedId
    ? data.find(conversation => conversation.id === selectedId)?.title
    : 'Teleport Assist';

  return (
    <Container>
      <Icons>
        {(props.viewMode === AssistViewMode.Collapsed ||
          props.viewMode === AssistViewMode.Docked) && (
          <Icon
            rotated
            onClick={() =>
              props.onViewModeChange(toggleSidebarVisible(props.viewMode))
            }
          >
            <ChevronRightIcon size={16} />
          </Icon>
        )}
        {isExpanded(props.viewMode) && (
          <Icon
            onClick={() =>
              props.onViewModeChange(toggleSidebarVisible(props.viewMode))
            }
          >
            <SidebarIcon size={20} />
          </Icon>
        )}
      </Icons>

      <Title>{title}</Title>

      <Icons style={{ justifyContent: 'flex-end' }}>
        {!isDocked(props.viewMode) && (
          <Icon onClick={props.onExpand}>
            <ExpandIcon size={14} />
          </Icon>
        )}
        <Icon rotated onClick={props.onDocking}>
          {!isDocked(props.viewMode) ? (
            <SidebarIcon size={20} />
          ) : (
            <PopupIcon size={20} />
          )}
        </Icon>
        <Icon onClick={() => props.onClose()}>
          <CloseIcon size={24} />
        </Icon>
      </Icons>
    </Container>
  );
}
