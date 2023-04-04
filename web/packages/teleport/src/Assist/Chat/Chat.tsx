import React, { useCallback, useEffect, useRef } from 'react';
import styled, { keyframes } from 'styled-components';

import { Dots } from 'teleport/Assist/Dots';

import { useMessages } from '../contexts/messages';

import { Author, Type } from '../services/messages';

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

const appear = keyframes`
  0% {
    opacity: 0;
  }

  100% {
    opacity: 1;
  }
`;

const LoadingContainer = styled.div`
  display: flex;
  justify-content: center;
  opacity: 0;
  margin-top: 50px;
  animation: ${appear} 0.6s linear forwards;
  animation-delay: 1s;
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
          <ExampleChatItem />

          {items}

          <div ref={ref} />
        </Padding>
      </Content>

      <ChatBox onSubmit={handleSubmit} />
    </Container>
  );
}
