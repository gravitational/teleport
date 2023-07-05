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

import React from 'react';
import styled, { keyframes } from 'styled-components';

import DOMPurify from 'dompurify';
import highlight from 'highlight.js';

import { marked } from 'marked';

import teleport from 'design/assets/images/icons/teleport.png';

import { useTeleport } from 'teleport';

import { getBorderRadius } from 'teleport/Assist/Chat/ChatItem/utils';

import { Author, Message, Type } from '../../services/messages';

import { Timestamp } from '../Timestamp';

import { codeCSS } from './styles/code';
import { markdownCSS } from './styles/markdown';

import { Actions } from './Action';

interface ChatItemProps {
  message: Message;
  isNew: boolean;
  scrollTextarea: () => void;
  hideAvatar: boolean;
  isFirstFromUser: boolean;
  isLastFromUser: boolean;
}

const appear = keyframes`
  to {
    transform: translate3d(0, 0, 0);
    opacity: 1;
  }
`;

const Content = styled.div`
  padding: 20px 25px 4px;
  box-shadow: 0 6px 12px -2px rgba(50, 50, 93, 0.05),
    0 3px 7px -3px rgba(0, 0, 0, 0.1);
  max-width: 90%;
`;

const Container = styled.div<{
  teleport?: boolean;
  isLast: boolean;
  isNew: boolean;
}>`
  display: flex;
  flex-direction: column;
  align-items: ${p => (p.teleport ? 'flex-start' : 'flex-end')};
  justify-content: ${p => (p.teleport ? '' : 'flex-end')};
  margin: 0 30px ${p => (p.hasSpacing ? '25px' : '15px')} 30px;
  position: relative;
  animation: ${appear} 0.6s linear forwards;
  transform: ${p => (p.isNew ? 'translate3d(0, 30px, 0)' : 'none')};
  opacity: ${p => (p.isNew ? 0 : 1)};
  font-size: 14px;

  ${Content} {
    background: ${p => (p.teleport ? '#4A5688' : '#9F85FF')};
    color: ${p => (p.teleport ? 'white' : 'black')};
    border-radius: ${p =>
      getBorderRadius(p.teleport, p.isFirstFromUser, p.isLastFromUser)};
  }
`;

const ChatItemAvatarUser = styled.div`
  width: 30px;
  height: 30px;
  border-radius: 10px;
  overflow: hidden;
  font-size: 14px;
  font-weight: bold;
  display: flex;
  align-items: center;
  justify-content: center;
  background-size: cover;
  margin-right: 10px;
  background: #9f85ff;
  color: black;
`;

const ChatItemAvatarImage = styled.div<{ backgroundImage: string }>`
  background: url(${p => p.backgroundImage}) no-repeat;
  width: 22px;
  height: 22px;
  overflow: hidden;
  background-size: cover;
`;

const AvatarContainer = styled.div`
  display: flex;
  align-items: center;
  color: rgba(255, 255, 255, 0.72);
  margin-top: 20px;

  strong {
    display: block;
    margin-right: 10px;
    color: white;
  }
`;

const UserAvatarContainer = styled(AvatarContainer)`
  right: 0;
`;

const TeleportAvatarContainer = styled(AvatarContainer)`
  left: 0;
`;

const ChatItemAvatarTeleport = styled.div`
  background: #9f85ff;
  color: black;
  padding: 4px;
  border-radius: 10px;
  left: 0;
  right: auto;
  margin-right: 10px;
`;

const ChatItemContent = styled.div`
  font-size: 15px;
  width: 100%;
  position: relative;

  ${markdownCSS}
  ${codeCSS}
`;

// TODO(jakule || ryan): Remove duplicated styles.
const CommandOutput = styled.div`
  margin-bottom: 15px;
  min-width: 100%;
`;

const MachineName = styled.div`
  margin-bottom: 5px;
  font-size: 14px;
`;

const Output = styled.div`
  white-space: pre-wrap;
  background: #020308;
  color: white;
  border-radius: 5px;
  min-width: 500px;
  padding: 5px 10px;
  font-family: SFMono-Regular, Consolas, Liberation Mono, Menlo, Courier,
    monospace;
`;

const ErrorMessage = styled.div`
  color: #ff6257;
  font-size: 15px;
  font-weight: 500;
`;

marked.setOptions({
  renderer: new marked.Renderer(),
  highlight: function (code, lang) {
    const language = highlight.getLanguage(lang) ? lang : 'plaintext';

    return highlight.highlight(code, { language }).value;
  },
  langPrefix: 'hljs language-',
  pedantic: false,
  gfm: true,
  breaks: true,
  sanitize: false,
  smartLists: true,
  smartypants: false,
  xhtml: false,
});

export function ChatItem(props: ChatItemProps) {
  const ctx = useTeleport();

  let content;

  switch (props.message.content.type) {
    case Type.Message:
      content = (
        <ChatItemContent
          dangerouslySetInnerHTML={{
            __html: DOMPurify.sanitize(
              marked.parse(props.message.content.value)
            ),
          }}
        />
      );

      break;

    case Type.ExecuteRemoteCommand:
      content = (
        <Actions
          scrollTextarea={props.scrollTextarea}
          actions={props.message.content}
          key="commands"
          showRunButton={props.isNew}
        />
      );

      break;
    case Type.ExecuteCommandOutput:
      return (
        <Container
          teleport={props.message.author === Author.Teleport}
          isNew={props.isNew}
          isFirstFromUser={props.isFirstFromUser}
          isLastFromUser={props.isLastFromUser}
          hasSpacing={!props.hideAvatar}
        >
          <Content>
            <CommandOutput
              key={
                props.message.content.nodeId?.toString() +
                props.message.content.executionId
              }
            >
              <MachineName>
                Command ran on node{' '}
                <strong>{props.message.content.nodeId}</strong>
              </MachineName>
              {props.message.content.errorMsg ? (
                <ErrorMessage>{props.message.content.errorMsg}</ErrorMessage>
              ) : (
                <Output>{props.message.content.payload}</Output>
              )}
            </CommandOutput>
          </Content>
        </Container>
      );
  }

  let avatar = (
    <TeleportAvatarContainer>
      <ChatItemAvatarTeleport>
        <ChatItemAvatarImage backgroundImage={teleport} />
      </ChatItemAvatarTeleport>

      <strong>Teleport</strong>

      <Timestamp isoTimestamp={props.message.timestamp} />
    </TeleportAvatarContainer>
  );

  if (props.message.author === Author.User) {
    avatar = (
      <UserAvatarContainer>
        <ChatItemAvatarUser>
          {ctx.storeUser.state.username.slice(0, 1).toUpperCase()}
        </ChatItemAvatarUser>

        <strong>You</strong>

        <Timestamp isoTimestamp={props.message.timestamp} />
      </UserAvatarContainer>
    );
  }

  return (
    <Container
      teleport={props.message.author === Author.Teleport}
      isNew={props.isNew}
      isFirstFromUser={props.isFirstFromUser}
      isLastFromUser={props.isLastFromUser}
      hasSpacing={!props.hideAvatar}
    >
      <Content>{content}</Content>

      {!props.hideAvatar && avatar}
    </Container>
  );
}
