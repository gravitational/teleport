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

import React, { useRef } from 'react';
import styled from 'styled-components';
import * as Icons from 'design/Icon';
import { ButtonIcon, Text } from 'design';

import LinearProgress from 'teleterm/ui/components/LinearProgress';

import { useTabDnD } from './useTabDnD';

type TabItemProps = {
  index?: number;
  name?: string;
  active?: boolean;
  nextActive?: boolean;
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
    nextActive,
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
    <RelativeContainer
      onClick={onClick}
      onContextMenu={onContextMenu}
      css={`
        flex-grow: 1;
        min-width: 0;
      `}
    >
      <TabContent
        ref={ref}
        active={active}
        dragging={isDragging}
        canDrag={canDrag}
        title={name}
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
            <Icons.Cross size="small" />
          </ButtonIcon>
        )}
      </TabContent>
      {!active && !nextActive && <Separator />}
      {(!active || isDragging) && <BottomShadow />}
    </RelativeContainer>
  );
}

type NewTabItemProps = {
  tooltip: string;
  onClick(): void;
};

export function NewTabItem(props: NewTabItemProps) {
  return (
    <RelativeContainer>
      <TabContent active={false}>
        <ButtonIcon
          ml="1"
          mr="2"
          size={0}
          title={props.tooltip}
          onClick={props.onClick}
        >
          <Icons.Add size="small" />
        </ButtonIcon>
      </TabContent>
      <BottomShadow />
    </RelativeContainer>
  );
}

const RelativeContainer = styled.div`
  position: relative;
  display: flex;
  flex-basis: 0;
  align-items: center;
  height: 100%;
`;

const TabContent = styled.div<{
  dragging?: boolean;
  active?: boolean;
  // TODO(bl-nero): is this really used? Perhaps remove it.
  canDrag?: boolean;
}>`
  display: flex;
  z-index: 1; // covers shadow from the top
  align-items: center;
  min-width: 0;
  width: 100%;
  height: 100%;
  border-radius: 8px 8px 0 0;
  position: relative;
  opacity: ${props => (props.dragging ? 0 : 1)};
  color: ${props =>
    props.active
      ? props.theme.colors.text.main
      : props.theme.colors.text.slightlyMuted};
  background: ${props =>
    props.active
      ? props.theme.colors.levels.sunken
      : props.theme.colors.levels.surface};
  box-shadow: ${props =>
    props.active ? 'inset 0px 2px 1.5px -1px rgba(0, 0, 0, 0.12)' : undefined};

  &:hover,
  &:focus {
    color: ${props => props.theme.colors.text.main};
    transition: color 0.3s;
  }
`;

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

const BottomShadow = styled.div`
  box-shadow:
    0 2px 1px -1px rgba(0, 0, 0, 0.2),
    0 1px 1.5px rgba(0, 0, 0, 0.13),
    0 1px 4px rgba(0, 0, 0, 0.12);
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: inherit;
`;

const Separator = styled.div`
  height: 23px;
  width: 1px;
  position: absolute;
  z-index: 1;
  right: 0;
  background: ${props => props.theme.colors.spotBackground[2]};
`;
