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

import {
  CloseIcon,
  ConversationListIcon,
  PlusIcon,
  SettingsIcon,
} from 'design/SVGIcon';

import { AssistViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/assist_pb';

import { useAssist } from 'teleport/Assist/context/AssistContext';
import { Tooltip } from 'teleport/Assist/shared/Tooltip';
import { HeaderIcon } from 'teleport/Assist/shared';

interface HeaderProps {
  viewMode: AssistViewMode;
  sidebarVisible: boolean;
  onClose: () => void;
  onToggleSidebar: () => void;
  onSettingsOpen: () => void;
  onError: (message: string) => void;
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
  position: relative;
  z-index: 9999;
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

const ConversationListIconWrapper = styled.div`
  position: relative;
  top: 5.5px; // align with the plus icon
`;

export function Header(props: HeaderProps) {
  const {
    conversations: { selectedId, data },
    createConversation,
  } = useAssist();

  const title = selectedId
    ? data.find(conversation => conversation.id === selectedId)?.title
    : 'Teleport Assist';

  async function handleCreateNewConversation() {
    try {
      await createConversation();
    } catch (err) {
      props.onError('There was an error creating a new conversation.');
    }
  }

  return (
    <Container>
      <Icons>
        <HeaderIcon onClick={props.onToggleSidebar}>
          <ConversationListIconWrapper>
            <ConversationListIcon size={22} />
          </ConversationListIconWrapper>

          <Tooltip>
            {props.sidebarVisible ? 'Close' : 'Open'} conversation history
          </Tooltip>
        </HeaderIcon>
        <HeaderIcon onClick={handleCreateNewConversation}>
          <PlusIcon size={22} />

          <Tooltip>Start a new conversation</Tooltip>
        </HeaderIcon>
      </Icons>

      <Title>{title}</Title>

      <Icons style={{ justifyContent: 'flex-end' }}>
        <HeaderIcon onClick={props.onSettingsOpen}>
          <SettingsIcon size={18} />

          <Tooltip position="middle">Open settings</Tooltip>
        </HeaderIcon>
        <HeaderIcon onClick={() => props.onClose()}>
          <CloseIcon size={24} />

          <Tooltip position="right">Hide Assist</Tooltip>
        </HeaderIcon>
      </Icons>
    </Container>
  );
}
