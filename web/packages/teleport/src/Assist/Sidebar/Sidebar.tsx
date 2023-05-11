/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useCallback } from 'react';
import styled, { useTheme } from 'styled-components';

import { NavLink } from 'react-router-dom';

import { useHistory } from 'react-router';

import { ChatIcon, PlusIcon } from 'design/SVGIcon';

import { useConversations } from 'teleport/Assist/contexts/conversations';

import cfg from 'teleport/config';

const Container = styled.div`
  display: flex;
  flex-direction: column;
  margin-top: 10px;
  height: calc(100vh - 230px);
`;

const ChatHistoryTitle = styled.div`
  font-size: 13px;
  line-height: 14px;
  color: ${props => props.theme.colors.text.main};
  font-weight: bold;
  margin-left: 32px;
  margin-bottom: 13px;
`;

const ChatHistoryItem = styled(NavLink)`
  display: flex;
  color: ${props => props.theme.colors.text.main};
  padding: 7px 0px 7px 30px;
  line-height: 1.4;
  align-items: center;
  cursor: pointer;
  text-decoration: none;
  border-left: 4px solid transparent;
  opacity: 0.7;

  &:hover {
    background: ${props => props.theme.colors.spotBackground[0]};
  }

  &.active {
    opacity: 1;
    background: ${props => props.theme.colors.spotBackground[0]};
    border-left-color: ${props => props.theme.colors.brand};
  }
`;

const ChatHistoryItemTitle = styled.div`
  font-size: 15px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  padding-right: 20px;
`;

const NewChatButton = styled.div`
  padding: 10px 20px;
  border-radius: 7px;
  font-size: 15px;
  font-weight: bold;
  display: flex;
  cursor: pointer;
  margin: 0 15px;
  background: ${p => p.theme.colors.buttons.primary.default};
  color: ${p => p.theme.colors.buttons.primary.text};
  align-items: center;

  svg {
    position: relative;
    margin-right: 10px;
  }

  &:hover {
    background: ${p => p.theme.colors.buttons.primary.hover};
  }
`;

const ChatHistoryItemIcon = styled.div`
  flex: 0 0 14px;
  margin-right: 10px;
  padding-top: 4px;
`;

const ChatHistoryList = styled.div.attrs({ 'data-scrollbar': 'default' })`
  overflow-y: auto;
  flex: 1;
`;

const ErrorMessage = styled.div`
  color: ${p => p.theme.colors.error.main};
  font-weight: 700;
  margin-bottom: 5px;
  padding: 0 15px 15px;
`;

export function Sidebar() {
  const theme = useTheme();

  const history = useHistory();

  const { create, conversations, error } = useConversations();

  const handleNewChat = useCallback(() => {
    create().then(conversationId =>
      history.push(cfg.getAssistConversationUrl(conversationId))
    );
  }, []);

  const chatHistory = conversations.map(conversation => (
    <ChatHistoryItem
      key={conversation.id}
      to={`/web/assist/${conversation.id}`}
    >
      <ChatHistoryItemIcon>
        <ChatIcon size={14} />
      </ChatHistoryItemIcon>
      <ChatHistoryItemTitle>{conversation.title}</ChatHistoryItemTitle>
    </ChatHistoryItem>
  ));

  return (
    <Container>
      {error && <ErrorMessage>{error}</ErrorMessage>}

      <ChatHistoryTitle>Chat History</ChatHistoryTitle>
      <ChatHistoryList>{chatHistory}</ChatHistoryList>

      <NewChatButton onClick={() => handleNewChat()}>
        <PlusIcon size={16} fill={theme.colors.buttons.primary.text} />
        New Conversation
      </NewChatButton>
    </Container>
  );
}
