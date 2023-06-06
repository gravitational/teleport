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

import React, { Fragment } from 'react';
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

  const $emptyTab = (
    <>
      <TabItem active={true} />
      <Separator />
    </>
  );

  const $items = items.length
    ? items.map((item, index) => {
        const active = item.uri === activeTab;
        return (
          <Fragment key={item.uri}>
            <TabItem
              index={index}
              name={item.title}
              active={active}
              onClick={() => onSelect(item)}
              onClose={() => onClose(item)}
              onContextMenu={() => onContextMenu(item)}
              onMoved={onMoved}
              isLoading={getIsLoading(item)}
              closeTabTooltip={closeTabTooltip}
            />
            <Separator />
          </Fragment>
        );
      })
    : $emptyTab;

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

const Separator = styled.div`
  height: 23px;
  width: 1px;
  margin: 0 1px;
  background: ${props => props.theme.colors.spotBackground[2]};
`;

const StyledTabs = styled(Box)`
  background-color: ${props => props.theme.colors.levels.surface};
  min-height: 32px;
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  flex-shrink: 0;
  overflow: hidden;
  position: relative;
  z-index: 1;
  box-shadow: 0px 1px 10px 0px rgba(0, 0, 0, 0.12),
    0px 4px 5px 0px rgba(0, 0, 0, 0.14), 0px 2px 4px -1px rgba(0, 0, 0, 0.2);
  ${typography}
`;
