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
  useRef,
  useState,
} from 'react';
import styled from 'styled-components';

import { useMessages } from 'teleport/Assist/contexts/messages';

interface ChatBoxProps {
  disabled?: boolean;
  onSubmit: (value: string) => void;
  errorMessage: string | null;
}

const Container = styled.div`
  padding: 0 30px 30px;
`;

const TextArea = styled.textarea`
  width: 100%;
  background: #4a5688;
  color: white;
  border: 2px solid rgba(255, 255, 255, 0.13);
  border-radius: 10px;
  resize: none;
  padding: 20px 20px 5px 30px;
  font-size: 16px;
  line-height: 24px;
  box-sizing: border-box;

  &:focus {
    outline: none;
    border-color: rgba(255, 255, 255, 0.18);
  }

  ::placeholder {
    color: rgba(255, 255, 255, 0.54);
  }
`;

const ErrorMessage = styled.div`
  color: #ff6257;
  font-weight: 700;
  margin-bottom: 5px;
`;

export function ChatBox(props: ChatBoxProps) {
  const [value, setValue] = useState('');
  const ref = useRef<HTMLTextAreaElement>(null);

  const { responding } = useMessages();

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
  }, [props.disabled]);

  function handleChange(event: ChangeEvent<HTMLTextAreaElement>) {
    setValue(event.target.value);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      event.stopPropagation();

      if (!responding && value) {
        props.onSubmit(value);
        setValue('');
      }
    }
  }

  return (
    <Container>
      {props.errorMessage && <ErrorMessage>{props.errorMessage}</ErrorMessage>}

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
