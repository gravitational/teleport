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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import teleport from 'design/assets/images/icons/teleport.png';

import logger from 'shared/libs/logger';

import { Dots } from 'teleport/Assist/Dots';

import {
  Typing,
  TypingContainer,
  TypingDot,
} from 'teleport/Assist/Chat/Typing';

import {
  AvatarContainer,
  ChatItemAvatarImage,
  ChatItemAvatarTeleport,
} from 'teleport/Assist/Chat/Avatar';

import { useConversations } from 'teleport/Assist/contexts/conversations';

import {
  generateTitle,
  setConversationTitle,
  useMessages,
} from '../contexts/messages';

import { ChatBox } from './ChatBox';
import { ChatItem } from './ChatItem';

const Container = styled.div`
  flex: 1;
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
`;

const Content = styled.div.attrs({ 'data-scrollbar': 'default' })`
  flex: 1 1 auto;
  overflow-y: auto;
  padding-top: 30px;
  display: flex;
  justify-content: center;
`;

const Padding = styled.div`
  padding: 30px;
  box-sizing: border-box;
`;

const LoadingContainer = styled.div`
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
`;

const Width = styled.div`
  max-width: 1200px;
  width: 100%;
`;

class ChatProps {
  conversationId: string;
}

export function Chat(props: ChatProps) {
  const ref = useRef<HTMLDivElement>(null);

  const [error, setError] = useState<string>(null);
  const {
    send,
    messages,
    loading,
    responding,
    error: messagesError,
  } = useMessages();
  const { conversations, setConversations } = useConversations();

  const scrollTextarea = useCallback(() => {
    ref.current?.scrollIntoView({ behavior: 'smooth' });
  }, [ref.current]);

  useEffect(() => {
    scrollTextarea();
  }, [messages, scrollTextarea]);

  const handleSubmit = useCallback(
    (message: string) => {
      send(message).then(() => {
        if (messages.length == 1) {
          // Use the second message/first message from a user to generate the title.
          (async () => {
            try {
              // Generate title using the last message and OpenAI API.
              const title = await generateTitle(message);
              // Set the title in the backend.
              await setConversationTitle(props.conversationId, title);
              // Update the title in the frontend.
              setConversations(conversations =>
                conversations.map(c => {
                  if (c.id === props.conversationId) {
                    c.title = title;
                  }
                  return c;
                })
              );
            } catch (err) {
              setError('An error occurred when setting the conversation title');

              logger.error(err);
            }
          })();
        }
      });
    },
    [messages, conversations, setConversations]
  );

  const items = messages.map((message, index) => (
    <ChatItem
      scrollTextarea={scrollTextarea}
      key={index}
      message={message}
      isNew={message.isNew}
      hideAvatar={
        messages[index + 1] && messages[index + 1].author === message.author
      }
      isLastFromUser={
        messages[index + 1]
          ? messages[index + 1].author !== message.author
          : true
      }
      isFirstFromUser={
        messages[index - 1]
          ? messages[index - 1].author !== message.author
          : true
      }
    />
  ));

  let content;
  if (loading) {
    content = (
      <LoadingContainer>
        <Dots />
      </LoadingContainer>
    );
  } else {
    content = (
      <Padding>
        {items}

        {responding && (
          <Typing>
            <AvatarContainer>
              <ChatItemAvatarTeleport>
                <ChatItemAvatarImage backgroundImage={teleport} />
              </ChatItemAvatarTeleport>

              <TypingContainer>
                <TypingDot style={{ animationDelay: '0s' }} />
                <TypingDot style={{ animationDelay: '0.2s' }} />
                <TypingDot style={{ animationDelay: '0.4s' }} />
              </TypingContainer>
            </AvatarContainer>
          </Typing>
        )}

        <div ref={ref} />
      </Padding>
    );
  }

  return (
    <Container>
      <Content>
        <Width>{content}</Width>
      </Content>

      <div style={{ display: 'flex', justifyContent: 'center' }}>
        <Width>
          <ChatBox
            onSubmit={handleSubmit}
            errorMessage={error || messagesError}
          />
        </Width>
      </div>
    </Container>
  );
}
