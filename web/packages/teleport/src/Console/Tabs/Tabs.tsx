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

import styled from 'styled-components';

import { Box, ButtonIcon } from 'design';
import * as Icons from 'design/Icon';
import { typography } from 'design/system';
import { TypographyProps } from 'design/system/typography';
import { useStore } from 'shared/libs/stores';

import { useConsoleContext } from 'teleport/Console/consoleContextProvider';
import * as stores from 'teleport/Console/stores';

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
            flexGrow: '0',
          }}
        />
      );
    });

  return (
    <StyledTabs as="nav" color="text.slightlyMuted" bold {...styledProps}>
      {$items}
      {$items.length > 0 && (
        <ButtonIcon
          ml="2"
          size={0}
          disabled={disableNew}
          title="New Tab"
          onClick={onNew}
        >
          <Icons.Add size="small" />
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

const StyledTabs = styled(Box)<TypographyProps>`
  background: ${props => props.theme.colors.levels.surface};
  min-height: 32px;
  border-radius: 4px;
  display: flex;
  flex-wrap: no-wrap;
  align-items: center;
  flex-shrink: 0;
  overflow: hidden;
  ${typography}
`;
