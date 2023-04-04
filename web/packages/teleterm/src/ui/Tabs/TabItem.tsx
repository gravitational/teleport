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
import { ButtonIcon, Text } from 'design';

import LinearProgress from 'teleterm/ui/components/LinearProgress';

import { useTabDnD } from './useTabDnD';

type TabItemProps = {
  index?: number;
  name?: string;
  active?: boolean;
  closeTabTooltip?: string;
  isLoading?: boolean;
  onClick?(): void;
  onClose?(): void;
  onMoved?(oldIndex: number, newIndex: number): void;
  onContextMenu?(): void;
};

export function TabItem(props: TabItemProps) {
  const {
    name,
    active,
    onClick,
    onClose,
    index,
    onMoved,
    isLoading,
    onContextMenu,
    closeTabTooltip,
  } = props;
  const ref = useRef<HTMLDivElement>(null);
  const canDrag = !!onMoved;
  const { isDragging } = useTabDnD({
    index,
    onDrop: onMoved,
    ref,
    canDrag,
  });

  const handleClose = (event: MouseEvent) => {
    event.stopPropagation();
    onClose?.();
  };

  return (
    <StyledTabItem
      onClick={onClick}
      onContextMenu={onContextMenu}
      ref={ref}
      active={active}
      dragging={isDragging}
      title={name}
      canDrag={canDrag}
    >
      <Title color="inherit" fontWeight={700} fontSize="12px">
        {name}
      </Title>
      {isLoading && active && <LinearProgress transparentBackground={true} />}
      {onClose && (
        <ButtonIcon
          size={0}
          mr={1}
          title={closeTabTooltip}
          css={`
            transition: none;
          `}
          onClick={handleClose}
        >
          <CloseIcon fontSize="16px" />
        </ButtonIcon>
      )}
    </StyledTabItem>
  );
}

const StyledTabItem = styled.div(({ theme, active, dragging, canDrag }) => {
  const styles: any = {
    display: 'flex',
    flexBasis: '0',
    flexGrow: '1',
    opacity: '1',
    color: theme.colors.text.secondary,
    alignItems: 'center',
    minWidth: '0',
    height: '100%',
    border: 'none',
    borderRadius: '8px 8px 0 0',
    '&:hover, &:focus': {
      color: theme.colors.text.contrast,
      transition: 'color .3s',
    },
    position: 'relative',
  };

  if (active) {
    styles['backgroundColor'] = theme.colors.levels.sunken;
    styles['color'] = theme.colors.text.contrast;
    styles['transition'] = 'none';
  }

  if (dragging) {
    styles['opacity'] = 0;
  }

  if (canDrag) {
    styles['cursor'] = 'pointer';
  }

  return styles;
});

const Title = styled(Text)`
  display: block;
  cursor: pointer;
  outline: none;
  color: inherit;
  font-family: inherit;
  line-height: 32px;
  background-color: transparent;
  white-space: nowrap;
  padding-left: 12px;
  border: none;
  min-width: 0;
  width: 100%;
`;
