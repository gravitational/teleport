import React, { ChangeEvent, useEffect, useRef, useState } from 'react';
import styled, { keyframes } from 'styled-components';
import { BrainIcon } from 'design/SVGIcon';

import { Cross } from 'design/Icon';

import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';

import {
  Popover,
  PopoverHeader,
} from 'e-teleport/AccessMonitoring/shared/Popover';
import { useQueryAssist } from 'e-teleport/AccessMonitoring/QueryEditor/Assist/context';

const Container = styled.div`
  position: absolute;
  bottom: 16px;
  left: 16px;
  display: flex;
  align-items: center;
  gap: 8px;
`;

const AssistButton = styled.div`
  cursor: pointer;
  border-radius: 4px;
  display: flex;
  line-height: 1;
  align-items: center;
  justify-content: center;
  padding: 4px;
  position: relative;
  z-index: 2;

  &:hover {
    background: ${p =>
      p.isLoading ? 'none' : p.theme.colors.spotBackground[0]};
  }
`;

const StyledHeader = styled(PopoverHeader)`
  padding: 4px 4px 4px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
`;

const AssistPopover = styled(Popover)`
  top: -3px;
  width: 500px;
  left: 37px;
  transform: ${p => (p.visible ? 'translateY(0)' : 'translateX(-20px)')};
  transition: all 0.4s linear;
  opacity: ${p => (p.visible ? 1 : 0)};
  z-index: 1;

  &:before {
    left: -11px;
    top: 6px;
    border-width: 10px 10px 10px 0;
    border-color: transparent ${p => p.theme.colors.spotBackground[0]}
      transparent transparent;
  }

  &:after {
    left: -8px;
    top: 8px;
    border-width: 8px 8px 8px 0;
    border-color: transparent ${p => p.theme.colors.levels.popout} transparent
      transparent;
  }
`;

const Input = styled.input`
  width: 100%;
  background: ${props => props.theme.colors.levels.popout};
  color: ${props => props.theme.colors.text.main};
  border: none;
  resize: none;
  padding: 12px 16px;
  font-size: 14px;
  line-height: 1;
  box-sizing: border-box;
  overflow-y: hidden;
  border-radius: 0 0 8px 8px;

  &:focus {
    outline: none;
    border-color: ${props => props.theme.colors.spotBackground[2]};
  }

  ::placeholder {
    color: ${props => props.theme.colors.text.muted};
  }
`;

const spin = keyframes`
  0% {
    transform: rotate(0deg);
  }
  100% {
    transform: rotate(360deg);
  }
`;

const LoadingSpinner = styled.div`
  width: 18px;
  height: 18px;
  border-radius: 50%;
  box-sizing: border-box;
  border: 0.25rem solid ${p => p.theme.colors.spotBackground[2]};
  border-top-color: ${p => p.theme.colors.text.muted};
  animation: ${spin} 1s linear infinite;
`;

const CloseButton = styled.div`
  line-height: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4px;
  border-radius: 4px;
  cursor: pointer;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
  }
`;

const ErrorMessage = styled.div`
  color: ${p => p.theme.colors.error.main};
`;

const Hint = styled.div`
  font-size: 12px;
  color: ${p => p.theme.colors.text.muted};
  margin-left: 8px;
  line-height: 1.5;
  text-align: right;
  padding-right: 4px;
  padding-bottom: 2px;
`;

const Key = styled.span`
  color: ${p => p.theme.colors.text.slightlyMuted};
  font-weight: 600;
`;

export function QueryAssist() {
  const { close, loading, errorMessage, send, visible, open } =
    useQueryAssist();

  const ref = useRef<HTMLInputElement>(null);
  const popoverRef = useRefClickOutside<HTMLDivElement>({
    open: visible,
    setOpen: close,
  });

  const [value, setValue] = useState('');

  function handleChange(event: ChangeEvent<HTMLTextAreaElement>) {
    setValue(event.target.value);
  }

  function handleKeyDown(event: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === 'Escape') {
      close();
      return;
    }

    if (event.key !== 'Enter') {
      return;
    }

    event.preventDefault();
    event.stopPropagation();

    if (!value) {
      return;
    }

    send(value);
    setValue('');
  }

  function handleClick(event: React.MouseEvent) {
    event.preventDefault();
    event.stopPropagation();

    open();
  }

  useEffect(() => {
    if (visible) {
      ref.current?.focus();
    }
  }, [visible]);

  return (
    <Container>
      <AssistButton onClick={handleClick} isLoading={loading}>
        {loading ? <LoadingSpinner /> : <BrainIcon size={18} />}
      </AssistButton>

      {errorMessage && <ErrorMessage>{errorMessage}</ErrorMessage>}

      <AssistPopover visible={visible} ref={popoverRef}>
        <StyledHeader>
          AI Assisted Query
          <CloseButton onClick={close}>
            <Cross size="small" />
          </CloseButton>
        </StyledHeader>

        <Input
          ref={ref}
          value={value}
          onKeyDown={handleKeyDown}
          onChange={handleChange}
          placeholder="Show me all SSH sessions..."
        />

        <Hint>
          Press <Key>enter</Key> to generate
        </Hint>
      </AssistPopover>
    </Container>
  );
}
