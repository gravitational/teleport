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

import React, {
  ChangeEvent,
  KeyboardEvent,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import styled, { keyframes } from 'styled-components';

import { useAssist } from 'teleport/Assist/context/AssistContext';

const spin = keyframes`
  to {
    transform: rotate(360deg);
  }
`;

interface MessageBoxProps {
  disabled?: boolean;
  errorMessage: string | null;
}

const Container = styled.div`
  padding: 0 15px var(--assist-bottom-padding) 15px;
  position: relative;
`;

const Spinner = styled.div`
  width: 20px;
  height: 20px;

  &:after {
    content: ' ';
    display: block;
    width: 12px;
    height: 12px;
    margin: 8px;
    border-radius: 50%;
    border: 3px solid ${p => p.theme.colors.text.main};
    border-color: ${p => p.theme.colors.text.main} transparent
      ${p => p.theme.colors.text.main} transparent;
    animation: ${spin} 1.2s linear infinite;
  }
`;

const SpinnerContainer = styled.div`
  position: absolute;
  top: 12px;
  right: 40px;
`;

const TextArea = styled.textarea`
  width: 100%;
  background: ${props => props.theme.colors.levels.popout};
  color: ${props => props.theme.colors.text.main};
  border: 2px solid ${props => props.theme.colors.spotBackground[1]};
  border-radius: 10px;
  resize: none;
  padding: 17px 20px 1px 20px;
  font-size: 14px;
  line-height: 18px;
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

const ErrorMessage = styled.div`
  color: ${p => p.theme.colors.error.main};
  font-weight: 700;
  margin-bottom: 5px;
`;

export function MessageBox(props: MessageBoxProps) {
  const [value, setValue] = useState('');
  const ref = useRef<HTMLTextAreaElement>(null);

  const { conversations, messages, sendMessage } = useAssist();

  useEffect(() => {
    if (ref.current) {
      ref.current.style.height = '0px';
      const scrollHeight = ref.current.scrollHeight;

      ref.current.style.height = `${scrollHeight + 20}px`;
    }
  }, [ref.current, value]);

  useEffect(() => {
    if (ref.current) {
      ref.current.focus();
    }
  }, [props.disabled, ref.current]);

  useLayoutEffect(() => {
    if (ref.current) {
      ref.current.focus();
    }
  }, [conversations.selectedId, ref.current]);

  function handleChange(event: ChangeEvent<HTMLTextAreaElement>) {
    setValue(event.target.value);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      event.stopPropagation();

      if (!messages.streaming && value) {
        sendMessage(value);
        setValue('');
      }
    }
  }

  return (
    <Container>
      {props.errorMessage && <ErrorMessage>{props.errorMessage}</ErrorMessage>}

      {messages.streaming && (
        <SpinnerContainer>
          <Spinner />
        </SpinnerContainer>
      )}

      <TextArea
        disabled={props.disabled}
        ref={ref}
        rows={1}
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        placeholder="Reply to Teleport"
        autoFocus
      />
    </Container>
  );
}
