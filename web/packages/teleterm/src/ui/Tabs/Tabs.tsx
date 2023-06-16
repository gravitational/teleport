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

import React from 'react';
import styled from 'styled-components';
import { typography } from 'design/system';
import { Box, ButtonIcon } from 'design';
import * as Icons from 'design/Icon';

import { Document } from 'teleterm/ui/services/workspacesService';

import { TabItem } from './TabItem';

export function Tabs(props: Props) {
  const {
    items,
    activeTab,
    onSelect,
    onClose,
    onNew,
    disableNew,
    onMoved,
    onContextMenu,
    newTabTooltip,
    closeTabTooltip,
    ...styledProps
  } = props;

  const $items = items.length ? (
    items.map((item, index) => {
      const active = item.uri === activeTab;
      const nextActive = items[index + 1]?.uri === activeTab;
      return (
        <TabItem
          key={item.uri}
          index={index}
          name={item.title}
          active={active}
          nextActive={nextActive}
          onClick={() => onSelect(item)}
          onClose={() => onClose(item)}
          onContextMenu={() => onContextMenu(item)}
          onMoved={onMoved}
          isLoading={getIsLoading(item)}
          closeTabTooltip={closeTabTooltip}
        />
      );
    })
  ) : (
    <TabItem active={true} />
  );

  return (
    <StyledTabs as="nav" typography="h5" bold {...styledProps}>
      {$items}
      <ButtonIcon
        ml="1"
        mr="2"
        size={0}
        disabled={disableNew}
        title={newTabTooltip}
        onClick={onNew}
      >
        <Icons.Add fontSize="16px" />
      </ButtonIcon>
    </StyledTabs>
  );
}

function getIsLoading(doc: Document): boolean {
  return 'status' in doc && doc.status === 'connecting';
}

type Props = {
  items: Document[];
  activeTab: string;
  disableNew: boolean;
  newTabTooltip: string;
  closeTabTooltip: string;
  onNew: () => void;
  onSelect: (doc: Document) => void;
  onContextMenu: (doc: Document) => void;
  onMoved: (oldIndex: number, newIndex: number) => void;
  [index: string]: any;
};

const StyledTabs = styled(Box)`
  background-color: ${props => props.theme.colors.levels.surface};
  min-height: 32px;
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  flex-shrink: 0;
  max-width: 100%;
  position: relative;
  z-index: 1;
  ${typography}
`;
