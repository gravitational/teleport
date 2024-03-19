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

import { ChatCircleSparkle, Copy, Cross } from 'design/Icon';

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
        <ChatCircleSparkle size={14} />
        Ask Assist
      </Button>

      <CloseButton onClick={props.onClose}>
        <Cross size={14} />
      </CloseButton>
    </Container>
  );
}
