import React from 'react';
import styled from 'styled-components';

import { ChangeEvent, KeyboardEvent, useEffect, useRef, useState } from 'react';

interface ChatBoxProps {
  disabled?: boolean;
  onSubmit: (value: string) => void;
}

const Container = styled.div`
  padding: 30px;
`;

const TextArea = styled.textarea`
  background: #222c5a;
  width: 100%;
  border: 2px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  resize: none;
  padding: 20px 20px 5px 30px;
  font-size: 16px;
  color: white;
  line-height: 24px;
  box-sizing: border-box;

  &:focus {
    outline: none;
    border-color: rgba(255, 255, 255, 0.3);
  }

  ::placeholder {
    color: rgba(255, 255, 255, 0.5);
  }
`;

export function ChatBox(props: ChatBoxProps) {
  const [value, setValue] = useState('');
  const ref = useRef<HTMLTextAreaElement>(null);

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

      props.onSubmit(value);
      setValue('');
    }
  }

  return (
    <Container>
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
