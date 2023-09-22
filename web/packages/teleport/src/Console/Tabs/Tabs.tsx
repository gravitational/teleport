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

import * as Icons from 'design/Icon';
import { Box, ButtonIcon } from 'design';

import { useStore } from 'shared/libs/stores';

import * as stores from 'teleport/Console/stores';
import { useConsoleContext } from 'teleport/Console/consoleContextProvider';

import { colors } from '../colors';

import TabItem from './TabItem';

export default function TabsContainer(props: Props) {
  const ctx = useConsoleContext();
  // subscribe to store changes
  useStore(ctx.storeParties);
  return <Tabs {...props} parties={ctx.storeParties.state} />;
}

export function Tabs(props: Props & { parties: stores.Parties }) {
  const {
    items,
    parties,
    activeTab,
    onSelect,
    onClose,
    onNew,
    disableNew,
    ...styledProps
  } = props;

  const $items = items
    .filter(i => i.kind !== 'blank')
    .map(i => {
      const active = i.id === activeTab;
      let users: { user: string }[] = [];
      if (i.kind === 'terminal') {
        users = parties[i.sid] || [];
      }

      return (
        <TabItem
          name={i.title}
          key={i.id}
          users={users}
          active={active}
          onClick={() => onSelect(i)}
          onClose={() => onClose(i)}
          style={{
            flex: '1',
            flexBasis: '0',
            flexGrow: '1',
          }}
        />
      );
    });

  return (
    <StyledTabs
      as="nav"
      typography="h5"
      color="text.slightlyMuted"
      bold
      {...styledProps}
    >
      {$items}
      {$items.length > 0 && (
        <ButtonIcon
          ml="2"
          size={0}
          disabled={disableNew}
          title="New Tab"
          onClick={onNew}
        >
          <Icons.Add fontSize="16px" />
        </ButtonIcon>
      )}
    </StyledTabs>
  );
}

type Props = {
  items: stores.Document[];
  activeTab: number;
  disableNew: boolean;
  onNew: () => void;
  onSelect: (doc: stores.Document) => void;
  [index: string]: any;
};

const StyledTabs = styled(Box)`
  background: ${colors.terminalDark};
  min-height: 32px;
  border-radius: 4px;
  display: flex;
  flex-wrap: no-wrap;
  align-items: center;
  flex-shrink: 0;
  overflow: hidden;
  ${typography}
`;
