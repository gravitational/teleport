/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React from 'react';
import styled from 'styled-components';

import { CheckIcon } from 'design/SVGIcon';

import { Author, ServerMessageType } from 'teleport/Assist/types';

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
import { AccessRequest } from 'teleport/Assist/Conversation/AccessRequest/AccessRequest';
import { AccessRequestCreated } from 'teleport/Assist/Conversation/AccessRequest/AccessRequestCreated';

import type {
  ConversationMessage,
  ResolvedServerMessage,
} from 'teleport/Assist/types';

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
      return (
        <MessageEntry
          content={entry.message}
          markdown={entry.type === ServerMessageType.Assist}
        />
      );

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

    case ServerMessageType.AccessRequest:
      return (
        <AccessRequest
          resources={entry.resources}
          reason={entry.reason}
          roles={entry.roles}
        />
      );

    case ServerMessageType.AccessRequestCreated:
      return <AccessRequestCreated accessRequestId={entry.accessRequestId} />;
  }
}

function createEntryWrapper(
  entry: ResolvedServerMessage,
  author: Author,
  streaming: boolean,
  lastMessage: boolean,
  index: number,
  length: number
) {
  switch (entry.type) {
    case ServerMessageType.AssistThought:
      const processing = index === length - 1;

      return (
        <Thought key={index}>
          {processing ? (
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
      );

    default:
      return (
        <EntryContainer
          author={author}
          key={index}
          index={index}
          length={length}
          streaming={streaming && index === length - 1}
          lastMessage={lastMessage}
          hideOverflow={
            entry.type === ServerMessageType.CommandResultStream ||
            entry.type === ServerMessageType.CommandResult
          }
        >
          {createComponentForEntry(entry, lastMessage && index === length - 1)}
        </EntryContainer>
      );
  }
}

export function Message(props: MessageProps) {
  const { messages } = useAssist();

  const entries = props.message.entries.map((entry, index) =>
    createEntryWrapper(
      entry,
      props.message.author,
      messages.streaming,
      props.lastMessage,
      index,
      props.message.entries.length
    )
  );

  const authorIsTeleport = props.message.author === Author.Teleport;
  const showFooter = !(
    authorIsTeleport &&
    props.lastMessage &&
    messages.streaming
  );

  return (
    <Container>
      <Entries>{entries}</Entries>

      {showFooter && (
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

          <TimestampContainer>
            <Timestamp timestamp={props.message.timestamp} />
          </TimestampContainer>
        </Footer>
      )}
    </Container>
  );
}
