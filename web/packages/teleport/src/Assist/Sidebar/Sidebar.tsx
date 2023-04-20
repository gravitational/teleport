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
import styled from 'styled-components';

import { Link, NavLink } from 'react-router-dom';

import { CaretLeft } from 'design/Icon';

import { useHistory } from 'react-router';

import { useTeleport } from 'teleport';

import { useConversations } from 'teleport/Assist/contexts/conversations';

import { ChatIcon } from '../Icons/ChatIcon';
import { PlusIcon } from '../Icons/PlusIcon';

import logo from './logo.png';

const Container = styled.div`
  flex: 0 0 370px;
  padding-top: 30px;
  padding-left: 40px;
  display: flex;
  flex-direction: column;
  padding-bottom: 40px;
`;

const Logo = styled.div`
  background: url(${logo}) no-repeat;
  background-size: cover;
  width: 43px;
  height: 40px;
  flex: 0 0 40px;
  position: relative;
  margin: 10px 30px 40px 0;

  &:before {
    position: absolute;
    content: 'Teleport';
    top: 7px;
    right: -145px;
    font-size: 34px;
    font-weight: bold;
  }

  &:after {
    position: absolute;
    content: 'Assist';
    top: 7px;
    right: -252px;
    font-size: 34px;
    font-weight: bold;
    text-shadow: 0 0 5px rgba(255, 255, 255, 0.4),
      0 0 10px rgba(255, 255, 255, 0.4), 0 0 15px rgba(255, 255, 255, 0.4),
      0 0 20px rgba(255, 255, 255, 0.1);
  }
`;

const ChatHistoryTitle = styled.div`
  color: rgba(255, 255, 255, 0.6);
  font-weight: bold;
  font-size: 16px;
  margin-bottom: 20px;
  margin-top: 60px;
`;

const ChatHistoryItem = styled(NavLink)`
  color: white;
  display: flex;
  margin-bottom: 15px;
  border-radius: 10px;
  padding: 13px 20px 12px;
  line-height: 1.4;
  cursor: pointer;
  text-decoration: none;

  &:hover {
    background: rgba(255, 255, 255, 0.1);
  }

  &.active {
    background: rgba(255, 255, 255, 0.2);
  }
`;

const ChatHistoryItemTitle = styled.div`
  font-size: 18px;
`;

const NewChatButton = styled.div`
  padding: 15px 30px;
  border: 2px solid rgba(255, 255, 255, 0.6);
  border-radius: 10px;
  font-size: 18px;
  font-weight: bold;
  display: flex;
  justify-content: center;
  cursor: pointer;

  svg {
    position: relative;
    top: 2px;
    margin-right: 12px;
  }

  &:hover {
    background: rgba(255, 255, 255, 0.1);
  }
`;

const ChatHistoryItemIcon = styled.div`
  flex: 0 0 33px;
  padding-top: 4px;
`;

const UserInfoAvatar = styled.div`
  background: #5130c9;
  width: 32px;
  height: 32px;
  border-radius: 5px;
  overflow: hidden;
  font-size: 18px;
  color: white;
  font-weight: bold;
  display: flex;
  align-items: center;
  justify-content: center;
  background-size: cover;
`;

const UserInfo = styled.div`
  justify-self: flex-end;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 10px;
  padding: 15px 20px;
  display: flex;
  align-items: center;
`;

const UserInfoContent = styled.div`
  font-size: 20px;
  font-weight: bold;
  margin-left: 20px;
`;

const BackToTeleport = styled(Link)`
  color: white;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  border-radius: 5px;
  padding: 5px 10px;
  margin-left: -10px;
  margin-top: -5px;
  cursor: pointer;

  &:hover {
    background: rgba(255, 255, 255, 0.1);
  }

  span {
    margin-right: 10px;
  }
`;

const ChatHistoryList = styled.div`
  overflow-y: auto;
`;

export function Sidebar() {
  const ctx = useTeleport();
  const history = useHistory();

  const { create, conversations } = useConversations();

  const handleNewChat = useCallback(() => {
    create().then(id => history.push(`/web/assist/${id}`));
  }, []);

  const chatHistory = conversations.map(conversation => (
    <ChatHistoryItem
      key={conversation.id}
      to={`/web/assist/${conversation.id}`}
    >
      <ChatHistoryItemIcon>
        <ChatIcon size={18} />
      </ChatHistoryItemIcon>
      <ChatHistoryItemTitle>New Chat</ChatHistoryItemTitle>
    </ChatHistoryItem>
  ));

  return (
    <Container>
      <div>
        <BackToTeleport to={`/web`}>
          <CaretLeft /> Back to Teleport
        </BackToTeleport>
      </div>

      <Logo />

      <NewChatButton onClick={() => handleNewChat()}>
        <PlusIcon size={22} />
        New Chat
      </NewChatButton>

      <ChatHistoryTitle>Chat History</ChatHistoryTitle>
      <ChatHistoryList>{chatHistory}</ChatHistoryList>

      <UserInfo>
        <UserInfoAvatar>
          {ctx.storeUser.state.username.slice(0, 1).toUpperCase()}
        </UserInfoAvatar>

        <UserInfoContent>{ctx.storeUser.state.username}</UserInfoContent>
      </UserInfo>
    </Container>
  );
}
