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

import React, { useEffect } from 'react';
import styled from 'styled-components';

import { CloseIcon } from 'design/SVGIcon';
import { ChatCircleSparkle } from 'design/Icon';

import { useTerminalAssist } from 'teleport/Console/DocumentSsh/TerminalAssist/TerminalAssistContext';
import {
  Key,
  KeyShortcut,
} from 'teleport/Console/DocumentSsh/TerminalAssist/Shared';
import { MessageItem } from 'teleport/Console/DocumentSsh/TerminalAssist/MessageItem';
import { getMetaKeySymbol } from 'teleport/Console/DocumentSsh/TerminalAssist/utils';
import { MessageBox } from 'teleport/Console/DocumentSsh/TerminalAssist/MessageBox';
import {
  TypingContainer,
  TypingDot,
} from 'teleport/Console/DocumentSsh/TerminalAssist/Typing';

interface TerminalAssistProps {
  onUseCommand: (command: string) => void;
  onClose: () => void;
}

const Container = styled.div`
  position: fixed;
  bottom: 10px;
  right: 20px;
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[2]}px;
  z-index: 1000;
`;

const Button = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  transition: opacity 0.2s ease-in-out;
  cursor: pointer;
  width: 50px;
  height: 50px;
  position: relative;
  z-index: 2;

  svg path {
    fill: white;
  }
`;

const Background = styled.div`
  position: absolute;
  bottom: 32px;
  right: 2px;
  border-radius: ${p => (p.visible ? '25px' : '25px')};
  background: ${p => (p.visible ? p.theme.colors.levels.popout : '#311c79')};
  width: ${p => (p.visible ? '500px' : '50px')};
  height: ${p => (p.visible ? '600px' : '50px')};
  transition: all
    ${p =>
      p.visible
        ? '0.5s cubic-bezier(0.33, 1.2, 0.68, 1)'
        : '0.3s cubic-bezier(0.33, 1, 0.68, 1)'};
  transform-origin: bottom right;
  box-shadow: 0 5px 10px rgba(0, 0, 0, 0.4);
`;

const ChatContainer = styled.div`
  position: absolute;
  bottom: 30px;
  right: 2px;
  width: 500px;
  height: 600px;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  opacity: ${p => (p.visible ? 1 : 0)};
  transition: ${p => (p.visible ? 'opacity 0.5s ease-in-out' : 'none')};
  transition-delay: 0.2s;
  box-sizing: border-box;
  visibility: ${p => (p.visible ? 'visible' : 'hidden')};
`;

const Header = styled.header`
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px
    ${p => p.theme.space[2]}px ${p => p.theme.space[2] + p.theme.space[3]}px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[0]};
  user-select: none;
  box-sizing: border-box;
`;

const ScrollArea = styled.div.attrs({
  'data-scrollbar': 'default',
})`
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column-reverse;
  padding: 0 ${p => p.theme.space[2]}px ${p => p.theme.space[2]}px
    ${p => p.theme.space[2]}px;
  gap: ${p => p.theme.space[2]}px;
`;

const Title = styled.h2`
  margin: 0;
  font-size: 16px;
`;

export const Tooltip = styled.div`
  position: absolute;
  right: 2px;
  white-space: nowrap;
  pointer-events: none;
  top: 40px;
  z-index: 999;
  background: rgba(0, 0, 0, 0.8);
  color: white;
  border-radius: 7px;
  padding: 5px 8px;

  &:after {
    content: '';
    position: absolute;
    width: 0;
    height: 0;
    border-style: solid;
    border-width: 0 7px 7px 7px;
    border-color: transparent transparent rgba(0, 0, 0, 0.8) transparent;
    top: -7px;
    right: 10px;
  }
`;

const HeaderIcon = styled.div`
  border-radius: 7px;
  width: 38px;
  height: 38px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: 0.2s ease-in-out opacity;
  position: relative;

  ${Tooltip} {
    display: none;
  }

  svg {
    transform: ${p => (p.rotated ? 'rotate(180deg)' : 'none')};
  }

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};

    ${Tooltip} {
      display: block;
    }
  }
`;

export function TerminalAssist(props: TerminalAssistProps) {
  const { close, loading, messages, open, visible } = useTerminalAssist();

  useEffect(() => {
    function keyDownHandler(e: KeyboardEvent) {
      if (e.metaKey && e.key === '/') {
        e.preventDefault();
        e.stopPropagation();

        visible ? close() : open();
      }
    }

    window.addEventListener('keydown', keyDownHandler);

    return () => {
      window.removeEventListener('keydown', keyDownHandler);
    };
  }, [visible]);

  function handleUseCommand(command: string) {
    props.onUseCommand(command);
    close();
  }

  return (
    <Container>
      <Background visible={visible} />

      <ChatContainer visible={visible}>
        <Header>
          <Title>Assist</Title>

          <HeaderIcon onClick={close}>
            <CloseIcon size={24} />

            <Tooltip>Hide Assist</Tooltip>
          </HeaderIcon>
        </Header>

        <ScrollArea>
          {loading && (
            <TypingContainer>
              <TypingDot style={{ animationDelay: '0s' }} />
              <TypingDot style={{ animationDelay: '0.2s' }} />
              <TypingDot style={{ animationDelay: '0.4s' }} />
            </TypingContainer>
          )}

          {messages.map((m, i) => (
            <MessageItem
              key={i}
              message={m}
              lastMessage={i === 0}
              onUseCommand={handleUseCommand}
            />
          ))}
        </ScrollArea>

        <MessageBox onUseCommand={handleUseCommand} />
      </ChatContainer>

      <Button
        style={{
          opacity: visible ? 0 : 1,
          pointerEvents: visible ? 'none' : 'auto',
        }}
        onClick={() => open()}
      >
        <ChatCircleSparkle size={22} />
      </Button>

      <KeyShortcut style={{ opacity: visible ? 0 : 0.5 }}>
        <Key>{getMetaKeySymbol()}</Key>+
        <Key style={{ padding: '2px 6px' }}>/</Key>
      </KeyShortcut>
    </Container>
  );
}
