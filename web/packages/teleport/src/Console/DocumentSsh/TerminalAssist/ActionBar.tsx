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

import { Copy, Cross } from 'design/Icon';
import { BrainIcon } from 'design/SVGIcon';

interface Position {
  top: number;
  left: number;
}

interface ActionBarProps {
  position: Position;
  visible: boolean;
  onAskAssist: () => void;
  onClose: () => void;
  onCopy: () => void;
}

const Container = styled.div`
  position: absolute;
  background: ${p => p.theme.colors.levels.popout};
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  box-shadow: 0 0 10px ${p => p.theme.colors.spotBackground[1]};
  border-radius: 10px;
  line-height: 1;
  z-index: 100;
  transform: translate(-50%, 0);
  display: flex;
  overflow: hidden;
  padding: ${p => p.theme.space[1] / 2}px;
  transition: opacity 0.2s ease-in-out;
  opacity: ${p => (p.visible ? 1 : 0)};
  pointer-events: ${p => (p.visible ? 'auto' : 'none')};
`;

const Button = styled.div`
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
  padding: ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px;
  border-radius: 5px;
  cursor: pointer;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
  }
`;

const CloseButton = styled(Button)`
  padding: ${p => p.theme.space[2]}px;
`;

export function ActionBar(props: ActionBarProps) {
  const left = Math.max(props.position.left, 110); // make sure the bar is not off the edge of the screen

  function handleAskAssist() {
    props.onClose();
    props.onAskAssist();
  }

  function handleCopy() {
    props.onClose();
    props.onCopy();
  }

  return (
    <Container
      visible={props.visible}
      style={{ left, top: props.position.top }}
    >
      <Button onClick={handleCopy}>
        <Copy size={16} />
        Copy
      </Button>

      <Button onClick={handleAskAssist}>
        <BrainIcon size={14} />
        Ask Assist
      </Button>

      <CloseButton onClick={props.onClose}>
        <Cross size={14} />
      </CloseButton>
    </Container>
  );
}
