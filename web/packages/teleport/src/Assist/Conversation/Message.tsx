/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';

import { CheckIcon } from 'design/SVGIcon';

import {
  Author,
  ConversationMessage,
  ResolvedServerMessage,
  ServerMessageType,
} from 'teleport/Assist/types';

import {
  TeleportAvatar,
  UserAvatar,
} from 'teleport/Assist/Conversation/Avatar';
import {
  TypingContainer,
  TypingDot,
} from 'teleport/Assist/Conversation/Typing';
import { Timestamp } from 'teleport/Assist/Conversation/Timestamp';
import { EntryContainer } from 'teleport/Assist/Conversation/EntryContainer';
import { MessageEntry } from 'teleport/Assist/Conversation/MessageEntry';
import { useAssist } from 'teleport/Assist/context/AssistContext';
import { ExecuteRemoteCommandEntry } from 'teleport/Assist/Conversation/ExecuteRemoteCommandEntry';
import { CommandResultEntry } from 'teleport/Assist/Conversation/CommandResultEntry';
import { CommandResultSummaryEntry } from 'teleport/Assist/Conversation/CommandResultSummaryEntry';

interface MessageProps {
  message: ConversationMessage;
  lastMessage: boolean;
}

const Container = styled.li`
  padding: 0 20px;
  margin: 0 0 20px;
  flex: 1;
  display: flex;
  flex-direction: column;
`;

const Entries = styled.ul`
  list-style: none;
  padding: 20px 0 5px;
  margin: 0;
`;

const Footer = styled.footer`
  display: flex;
  align-items: center;
  color: ${props => props.theme.colors.text.muted};

  strong {
    display: block;
    margin-right: 10px;
    color: ${props => props.theme.colors.text.main};
  }
`;

const TimestampContainer = styled.span`
  font-size: 12px;
`;

const Thought = styled.div`
  display: flex;
  align-items: center;
  margin: 10px 0;
  font-size: 13px;

  ${TypingContainer} {
    padding: 0;
    margin-right: 10px;
  }

  ${TypingDot} {
    width: 5px;
    height: 5px;
    margin-right: 5px;
  }
`;

const ThoughtIcon = styled.div`
  margin-right: 10px;
  height: 16px;
`;

function createComponentForEntry(
  entry: ResolvedServerMessage,
  lastMessage: boolean
) {
  switch (entry.type) {
    case ServerMessageType.Assist:
    case ServerMessageType.User:
    case ServerMessageType.Error:
      return <MessageEntry content={entry.message} />;

    case ServerMessageType.Command:
      return (
        <ExecuteRemoteCommandEntry
          command={entry.command}
          query={entry.query}
          disabled={!lastMessage}
        />
      );

    case ServerMessageType.CommandResultStream:
      return (
        <CommandResultEntry
          nodeId={entry.nodeId}
          nodeName={entry.nodeName}
          output={entry.output}
          finished={entry.finished}
        />
      );

    case ServerMessageType.CommandResult:
      return (
        <CommandResultEntry
          nodeId={entry.nodeId}
          nodeName={entry.nodeName}
          output={entry.output}
          finished={true}
          errorMessage={entry.errorMessage}
        />
      );
    case ServerMessageType.CommandResultSummary:
      return (
        <CommandResultSummaryEntry
          command={entry.command}
          summary={entry.summary}
        />
      );
  }
}

export function Message(props: MessageProps) {
  const { messages } = useAssist();

  const entries = props.message.entries.map((entry, index) =>
    entry.type === ServerMessageType.AssistThought ? (
      <Thought key={index}>
        {index === props.message.entries.length - 1 ? (
          <TypingContainer>
            <TypingDot style={{ animationDelay: '0s' }} />
            <TypingDot style={{ animationDelay: '0.2s' }} />
            <TypingDot style={{ animationDelay: '0.4s' }} />
          </TypingContainer>
        ) : (
          <ThoughtIcon>
            <CheckIcon size={16} fill="#34a853" />
          </ThoughtIcon>
        )}

        {entry.message}
      </Thought>
    ) : (
      <EntryContainer
        author={props.message.author}
        key={index}
        index={index}
        length={props.message.entries.length}
        streaming={messages.streaming}
        lastMessage={props.lastMessage}
        hideOverflow={
          entry.type === ServerMessageType.CommandResultStream ||
          entry.type === ServerMessageType.CommandResult
        }
      >
        {createComponentForEntry(
          entry,
          props.lastMessage && index === props.message.entries.length - 1
        )}
      </EntryContainer>
    )
  );

  const authorIsTeleport = props.message.author === Author.Teleport;
  const typing = authorIsTeleport && props.lastMessage && messages.streaming;

  return (
    <Container>
      <Entries>{entries}</Entries>

      <Footer
        style={{
          justifyContent: authorIsTeleport ? 'flex-start' : 'flex-end',
        }}
      >
        {authorIsTeleport ? (
          <>
            <TeleportAvatar /> <strong>Teleport</strong>
          </>
        ) : (
          <>
            <UserAvatar /> <strong>You</strong>
          </>
        )}

        {typing ? (
          <TypingContainer>
            <TypingDot style={{ animationDelay: '0s' }} />
            <TypingDot style={{ animationDelay: '0.2s' }} />
            <TypingDot style={{ animationDelay: '0.4s' }} />
          </TypingContainer>
        ) : (
          <TimestampContainer>
            <Timestamp timestamp={props.message.timestamp} />
          </TimestampContainer>
        )}
      </Footer>
    </Container>
  );
}
