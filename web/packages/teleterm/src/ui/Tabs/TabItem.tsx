/*
Copyright 2019 Gravitational, Inc.

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

import React, { useRef } from 'react';
import styled from 'styled-components';
import { Close as CloseIcon } from 'design/Icon';
import { space } from 'design/system';
import { Text } from 'design';
import { useTabDnD } from './useTabDnD';

export function TabItem(props: Props) {
  const {
    name,
    active,
    onClick,
    onClose,
    style,
    index,
    onMoved,
    onContextMenu,
  } = props;
  const ref = useRef<HTMLDivElement>(null);
  const { isDragging } = useTabDnD({ index, onDrop: onMoved, ref });

  const handleClose = (event: MouseEvent) => {
    event.stopPropagation();
    onClose();
  };

  return (
    <StyledTabItem
      onClick={onClick}
      onContextMenu={onContextMenu}
      ref={ref}
      active={active}
      dragging={isDragging}
      title={name}
      style={{ ...style }}
    >
      <StyledTabButton>
        <Text mx="auto">{name}</Text>
      </StyledTabButton>
      <StyledCloseButton title="Close" onClick={handleClose}>
        <CloseIcon />
      </StyledCloseButton>
    </StyledTabItem>
  );
}

type Props = {
  index: number;
  name: string;
  users: { user: string }[];
  active: boolean;
  onClick: () => void;
  onClose: () => void;
  onMoved: (oldIndex: number, newIndex: number) => void;
  onContextMenu: () => void;
  style: any;
};

const StyledTabItem = styled.div(({ theme, active, dragging }) => {
  const styles: any = {
    display: 'flex',
    opacity: '1',
    alignItems: 'center',
    minWidth: '0',
    height: '100%',
    cursor: 'pointer',
    border: 'none',
    borderRight: `1px solid ${theme.colors.bgTerminal}`,
    '&:hover, &:focus': {
      color: theme.colors.primary.contrastText,
      transition: 'color .3s',
    },
  };

  if (active) {
    styles['backgroundColor'] = theme.colors.bgTerminal;
    styles['color'] = theme.colors.primary.contrastText;
    styles['fontWeight'] = 'bold';
    styles['transition'] = 'none';
  }

  if (dragging) {
    styles['opacity'] = 0;
  }

  return styles;
});

const StyledTabButton = styled.button`
  display: flex;
  cursor: pointer;
  outline: none;
  color: inherit;
  font-family: inherit;
  line-height: 32px;
  background-color: transparent;
  white-space: nowrap;
  padding: 0 8px;
  border: none;
  min-width: 0;
  width: 100%;
`;

const StyledCloseButton = styled.button`
  background: transparent;
  border-radius: 2px;
  border: none;
  cursor: pointer;
  height: 16px;
  width: 16px;
  outline: none;
  padding: 0;
  margin: 0 8px 0 0;
  transition: all 0.3s;

  ${space}
`;
