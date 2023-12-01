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

import {
  Message,
  MessageType,
} from 'teleport/Console/DocumentSsh/TerminalAssist/types';
import {
  Key,
  KeyShortcut,
} from 'teleport/Console/DocumentSsh/TerminalAssist/Shared';
import { getMetaKeySymbol } from 'teleport/Console/DocumentSsh/TerminalAssist/utils';

interface MessageItemProps {
  message: Message;
  onUseCommand: (command: string) => void;
  lastMessage?: boolean;
}

const UserMessage = styled.div`
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  border: 1px solid ${p => p.theme.colors.spotBackground[0]};
  box-shadow: 0 0 3px ${p => p.theme.colors.spotBackground[0]};
  border-radius: 7px;
`;

const MessageContainer = styled.div`
  display: flex;
  width: 100%;
`;

const UserMessageContainer = styled(MessageContainer)`
  justify-content: flex-end;
`;

const Explanation = styled.div`
  background: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 15px;
  padding: ${p => p.theme.space[3]}px;
  overflow: hidden;
`;

const Reasoning = styled.div`
  font-size: 14px;
  border-left: 2px solid ${p => p.theme.colors.spotBackground[1]};
  padding: ${p => p.theme.space[1]}px 0 ${p => p.theme.space[1]}px
    ${p => p.theme.space[2]}px;
  margin: ${p => p.theme.space[1]}px 0;
`;

const SuggestedCommand = styled.div`
  background: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 15px;
  padding: ${p => p.theme.space[3]}px;
  overflow: hidden;
`;

const SuggestedCommandTitle = styled.div`
  font-weight: 700;
  color: ${props => props.theme.colors.text.slightlyMuted};
`;

const Command = styled.pre.attrs({
  'data-scrollbar': 'default',
})`
  margin: 0 -${p => p.theme.space[2]}px -${p => p.theme.space[2]}px;
  padding: ${p => p.theme.space[2]}px;
  font-family: ${p => p.theme.fonts.mono};
  overflow-x: auto;
  font-size: 13px;
`;

const SuggestedCommandButtons = styled.div`
  display: flex;
  gap: ${p => p.theme.space[2]}px;
  margin-top: ${p => p.theme.space[3]}px;
`;

const SuggestedCommandButton = styled.div`
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
  font-size: 14px;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: 7px;
  line-height: 1;
  font-weight: bold;
  cursor: pointer;

  ${KeyShortcut} {
    background: ${p => p.theme.colors.spotBackground[0]};
    border-color: ${p => p.theme.colors.spotBackground[0]};
    color: ${p => p.theme.colors.text.main};
    opacity: 0.7;

    span {
      opacity: 0.5;
    }
  }
`;

const UseCommandButton = styled(SuggestedCommandButton)`
  background: ${p => p.theme.colors.buttons.primary.default};
  color: ${p => p.theme.colors.buttons.primary.text};

  ${KeyShortcut} {
    background: ${p => p.theme.colors.buttons.primary.default};
    border-color: ${p => p.theme.colors.buttons.primary.default};
  }

  ${Key} {
    color: ${p => p.theme.colors.buttons.primary.text};
  }
`;

export function MessageItem(props: MessageItemProps) {
  function handleUseCommand() {
    if (props.message.type !== MessageType.SuggestedCommand) {
      return;
    }

    props.onUseCommand(props.message.command);
  }

  if (props.message.type === MessageType.User) {
    return (
      <UserMessageContainer>
        <UserMessage>{props.message.value}</UserMessage>
      </UserMessageContainer>
    );
  }

  if (props.message.type === MessageType.SuggestedCommand) {
    return (
      <MessageContainer>
        <SuggestedCommand>
          <SuggestedCommandTitle>Suggested command</SuggestedCommandTitle>

          <Reasoning>{props.message.reasoning}</Reasoning>

          <Command>{props.message.command}</Command>

          <SuggestedCommandButtons>
            <UseCommandButton onClick={handleUseCommand}>
              Use
              {props.lastMessage && (
                <KeyShortcut>
                  <Key>
                    {getMetaKeySymbol()}
                    <span>+</span>⏎
                  </Key>
                </KeyShortcut>
              )}
            </UseCommandButton>
          </SuggestedCommandButtons>
        </SuggestedCommand>
      </MessageContainer>
    );
  }

  if (props.message.type === MessageType.Explanation) {
    return (
      <MessageContainer>
        <Explanation>{props.message.value}</Explanation>
      </MessageContainer>
    );
  }

  return null;
}
