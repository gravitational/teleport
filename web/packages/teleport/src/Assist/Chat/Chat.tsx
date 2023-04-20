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

import { Dots } from 'teleport/Assist/Dots';

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

const LoadingContainer = styled.div`
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
`;

const RespondingContainer = styled.div`
  display: flex;
  justify-content: center;
  margin-top: 30px;
`;

export function Chat() {
  const ref = useRef<HTMLDivElement>(null);

  const { send, messages, loading, responding } = useMessages();

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

  const items = messages.map((message, index) => (
    <ChatItem
      scrollTextarea={scrollTextarea}
      key={index}
      message={message}
      isNew={message.isNew}
      isLast={index === messages.length - 1}
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
          <RespondingContainer>
            <Dots />
          </RespondingContainer>
        )}

        <div ref={ref} />
      </Padding>
    );
  }

  return (
    <Container>
      <Header>New Chat</Header>

      <Content>{content}</Content>

      <ChatBox onSubmit={handleSubmit} />
    </Container>
  );
}

export function NewChat() {
  return (
    <Container>
      <Header>New Chat</Header>

      <Content>
        <Padding>
          <ExampleChatItem />
        </Padding>
      </Content>
    </Container>
  );
}
