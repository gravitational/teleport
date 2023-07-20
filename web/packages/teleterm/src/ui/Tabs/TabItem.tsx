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
            <Icons.Close fontSize="16px" />
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
          <Icons.Add fontSize="16px" />
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

const TabContent = styled.div`
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
