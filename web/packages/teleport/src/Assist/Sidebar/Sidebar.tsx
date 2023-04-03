import React from 'react';
import styled from 'styled-components';

import { useTeleport } from 'teleport';

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
  position: relative;
  margin: 35px 30px 60px 0;

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

const ChatHistory = styled.div`
  margin-top: 60px;
  flex: 1;
`;

const ChatHistoryTitle = styled.div`
  color: rgba(255, 255, 255, 0.6);
  font-weight: bold;
  font-size: 16px;
  margin-bottom: 20px;
`;

const ChatHistoryItem = styled.div`
  color: white;
  display: flex;
  margin-bottom: 15px;
  border-radius: 10px;
  padding: 13px 20px 12px;
  line-height: 1.4;
  margin-left: -12px;
  cursor: pointer;

  &:hover {
    background: rgba(255, 255, 255, 0.1);
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

  svg {
    position: relative;
    top: 2px;
    margin-right: 12px;
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

export function Sidebar() {
  const ctx = useTeleport();

  return (
    <Container>
      <Logo />

      <NewChatButton>
        <PlusIcon size={22} />
        New Chat
      </NewChatButton>

      <ChatHistory>
        <ChatHistoryTitle>Chat History</ChatHistoryTitle>

        <ChatHistoryItem>
          <ChatHistoryItemIcon>
            <ChatIcon size={18} />
          </ChatHistoryItemIcon>
          <ChatHistoryItemTitle>
            Update all production servers
          </ChatHistoryItemTitle>
        </ChatHistoryItem>
        <ChatHistoryItem>
          <ChatHistoryItemIcon>
            <ChatIcon size={18} />
          </ChatHistoryItemIcon>
          <ChatHistoryItemTitle>Check for audit anomalies</ChatHistoryItemTitle>
        </ChatHistoryItem>
      </ChatHistory>

      <UserInfo>
        <UserInfoAvatar>
          {ctx.storeUser.state.username.slice(0, 1).toUpperCase()}
        </UserInfoAvatar>

        <UserInfoContent>{ctx.storeUser.state.username}</UserInfoContent>
      </UserInfo>
    </Container>
  );
}
