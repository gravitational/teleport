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

import React, { ChangeEvent, useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import { useTerminalAssist } from 'teleport/Console/DocumentSsh/TerminalAssist/TerminalAssistContext';

interface MessageBoxProps {
  onUseCommand: (command: string) => void;
}

const Container = styled.div`
  padding: 0 ${p => p.theme.space[2]}px
    ${p => p.theme.space[2] + p.theme.space[1]}px ${p => p.theme.space[2]}px;
`;

const Input = styled.input`
  width: 100%;
  background: ${props => props.theme.colors.levels.popout};
  color: ${props => props.theme.colors.text.main};
  border: 2px solid ${props => props.theme.colors.spotBackground[1]};
  border-radius: 18px;
  resize: none;
  padding: ${p => p.theme.space[3]}px;
  font-size: 14px;
  line-height: 1;
  box-sizing: border-box;
  overflow-y: hidden;

  &:focus {
    outline: none;
    border-color: ${props => props.theme.colors.spotBackground[2]};
  }

  ::placeholder {
    color: ${props => props.theme.colors.text.muted};
  }
`;

export function MessageBox(props: MessageBoxProps) {
  const { close, getLastSuggestedCommand, loading, send, visible } =
    useTerminalAssist();

  const ref = useRef<HTMLTextAreaElement>(null);

  const [value, setValue] = useState('');

  function handleChange(event: ChangeEvent<HTMLTextAreaElement>) {
    setValue(event.target.value);
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (!visible || event.key !== 'Enter') {
      return;
    }

    if (event.metaKey) {
      const lastCommand = getLastSuggestedCommand();

      if (lastCommand) {
        close();

        props.onUseCommand(lastCommand);
      }

      return;
    }

    if (loading || !value) {
      return;
    }

    event.preventDefault();
    event.stopPropagation();

    send(value);
    setValue('');
  }

  useEffect(() => {
    if (visible) {
      ref.current.focus();
    }
  }, [visible]);

  return (
    <Container>
      <Input
        ref={ref}
        value={value}
        onKeyDown={handleKeyDown}
        onChange={handleChange}
        placeholder="Ask a question..."
      />
    </Container>
  );
}
