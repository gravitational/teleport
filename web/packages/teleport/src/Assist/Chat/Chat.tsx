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

import React, { useCallback, useEffect, useRef } from 'react';
import styled from 'styled-components';

import { useMessages } from '../contexts/messages';

import { ChatBox } from './ChatBox';
import { ChatItem } from './ChatItem';
import { ExampleChatItem } from './ChatItem/ChatItem';

const Container = styled.div`
  flex: 1;
  position: relative;
  background: #222c5a;
  overflow: hidden;
  display: flex;
  margin: 40px;
  border-radius: 20px;
  flex-direction: column;
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
`;

const Header = styled.div`
  padding: 30px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.2);
  color: white;
  position: relative;
  z-index: 100;
  font-size: 22px;
  font-weight: bold;
`;

const Content = styled.div`
  flex: 1 1 auto;
  overflow-y: auto;
  min-height: 100px;
`;

const Padding = styled.div`
  padding: 30px;
  box-sizing: border-box;
`;

export function Chat() {
  const ref = useRef<HTMLDivElement>(null);

  const { send, messages } = useMessages();

  const scrollTextarea = useCallback(() => {
    ref.current?.scrollIntoView({ behavior: 'smooth' });
  }, [ref.current]);

  useEffect(() => {
    scrollTextarea();
  }, [messages, scrollTextarea]);

  const handleSubmit = useCallback(
    (message: string) => {
      send(message);
    },
    [messages]
  );

  const items = messages
    .filter(message => !message.hidden)
    .map((message, index) => (
      <ChatItem
        scrollTextarea={scrollTextarea}
        key={index}
        message={message}
        isLast={index === messages.length - 1}
      />
    ));

  return (
    <Container>
      <Header>New Chat</Header>

      <Content>
        <Padding>
          {!items.length && <ExampleChatItem />}

          {items}

          <div ref={ref} />
        </Padding>
      </Content>

      <ChatBox onSubmit={handleSubmit} />
    </Container>
  );
}
