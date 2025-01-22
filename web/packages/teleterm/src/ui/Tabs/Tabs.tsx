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

import { Box } from 'design';
import { typography } from 'design/system';
import { TypographyProps } from 'design/system/typography';

import {
  Document,
  getStaticNameAndIcon,
} from 'teleterm/ui/services/workspacesService';

import { NewTabItem, TabItem } from './TabItem';

export function Tabs(props: Props) {
  const {
    items,
    activeTab,
    onSelect,
    onClose,
    onNew,
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
          Icon={getStaticNameAndIcon(item)?.Icon}
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
    <StyledTabs as="nav" {...styledProps}>
      {$items}
      <NewTabItem tooltip={newTabTooltip} onClick={onNew} />
    </StyledTabs>
  );
}

function getIsLoading(doc: Document): boolean {
  return 'status' in doc && doc.status === 'connecting';
}

type Props = {
  items: Document[];
  activeTab: string;
  newTabTooltip: string;
  closeTabTooltip: string;
  onNew: () => void;
  onSelect: (doc: Document) => void;
  onContextMenu: (doc: Document) => void;
  onMoved: (oldIndex: number, newIndex: number) => void;
  [index: string]: any;
};

// TODO(bl-nero): Typography should have a more restrictive type.
const StyledTabs = styled(Box)<TypographyProps>`
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
