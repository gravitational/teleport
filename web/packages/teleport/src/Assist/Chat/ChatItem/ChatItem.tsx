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

import { useTeleport } from 'teleport';

import { ExampleList } from '../Examples/ExampleList';

import { Author, Message, Type } from '../../services/messages';

import teleport from './teleport-icon.png';

import { codeCSS } from './styles/code';
import { markdownCSS } from './styles/markdown';

import { Actions } from './Action';
import { ActionState } from 'teleport/Assist/Chat/ChatItem/Action/types';

interface ChatItemProps {
  message: Message;
  isLast: boolean;
  isNew: boolean;
  scrollTextarea: () => void;
}

const appear = keyframes`
  to {
    transform: translate3d(0, 0, 0);
    opacity: 1;
  }
`;

const Container = styled.div<{
  teleport?: boolean;
  isLast: boolean;
  isNew: boolean;
}>`
  padding: 20px 30px;
  background: ${p => (p.teleport ? '#0c143d' : 'rgba(255, 255, 255, 0.1)')};
  display: flex;
  border-radius: 10px;
  margin-bottom: ${p => (p.isLast ? 0 : '70px')};
  position: relative;
  animation: ${appear} 0.6s linear forwards;
  transform: ${p => (p.isNew ? 'translate3d(0, 30px, 0)' : 'none')};
  opacity: ${p => (p.isNew ? 0 : 1)};
`;

const ChatItemAvatar = styled.div`
  position: absolute;
  bottom: -30px;
  right: 30px;
`;

const ChatItemAvatarTeleport = styled(ChatItemAvatar)`
  background: #1b254d;
  padding: 10px;
  left: 30px;
  border-radius: 10px;
  right: auto;
`;

const ChatItemContent = styled.div`
  font-size: 18px;
  padding-top: 8px;
  width: 100%;
  position: relative;

  ${markdownCSS}
  ${codeCSS}
`;

const ChatItemAvatarUser = styled.div`
  background: #5130c9;
  width: 62px;
  height: 62px;
  border-radius: 5px;
  overflow: hidden;
  font-size: 24px;
  color: white;
  font-weight: bold;
  display: flex;
  align-items: center;
  justify-content: center;
  background-size: cover;
`;

const ChatItemAvatarImage = styled.div<{ backgroundImage: string }>`
  background: url(${p => p.backgroundImage}) no-repeat;
  width: 42px;
  height: 42px;
  border-radius: 5px;
  overflow: hidden;
  background-size: cover;
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
            __html: DOMPurify.sanitize(marked.parse(props.message.content.value)),
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
        />
      );

      break;
  }

  let avatar = (
    <ChatItemAvatarTeleport>
      <ChatItemAvatarImage backgroundImage={teleport} />
    </ChatItemAvatarTeleport>
  );

  if (props.message.author === Author.User) {
    avatar = (
      <ChatItemAvatar>
        <ChatItemAvatarUser>
          {ctx.storeUser.state.username.slice(0, 1).toUpperCase()}
        </ChatItemAvatarUser>
      </ChatItemAvatar>
    );
  }

  return (
    <Container
      teleport={props.message.author === Author.Teleport}
      isLast={props.isLast}
      isNew={props.isNew}
    >
      {avatar}

      <div
        style={{
          width: '100%',
        }}
      >
        {content}
      </div>
    </Container>
  );
}

export function ExampleChatItem() {
  const ctx = useTeleport();

  return (
    <Container teleport={true} isLast={false} isNew={false}>
      <ChatItemAvatarTeleport>
        <ChatItemAvatarImage backgroundImage={teleport} />
      </ChatItemAvatarTeleport>
      <ChatItemContent>
        Hey {ctx.storeUser.state.username}, I'm Teleport - a powerful tool that
        can assist you in managing your Teleport cluster via ChatGPT. <br />
        <br />
        Start a new chat with me on the left to get started! Here's some of the
        things I can do:
        <ExampleList />
      </ChatItemContent>
    </Container>
  );
}
