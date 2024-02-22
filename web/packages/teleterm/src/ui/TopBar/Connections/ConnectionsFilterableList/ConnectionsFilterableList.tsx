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

import { Box, Text, ButtonIcon, Flex } from 'design';
import { Unlink } from 'design/Icon';

import { FilterableList } from 'teleterm/ui/components/FilterableList';
import { ExtendedTrackedConnection } from 'teleterm/ui/services/connectionTracker';
import { useKeyboardArrowsNavigationStateUpdate } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import { ConnectionItem } from './ConnectionItem';
import { ConnectionStatusIndicator } from './ConnectionStatusIndicator';

export function ConnectionsFilterableList(props: {
  items: ExtendedTrackedConnection[];
  onActivateItem(id: string): void;
  onRemoveItem(id: string): void;
  onDisconnectItem(id: string): void;
  closePopover(): void;
}) {
  const { setActiveIndex } = useKeyboardArrowsNavigationStateUpdate();

  return (
    <Box width="300px">
      {/*
        TODO(ravicious): Render "No Connections" if props.items.length is zero and VNet isn't
        supported.
      */}
      <FilterableList<ExtendedTrackedConnection>
        items={props.items}
        filterBy="title"
        placeholder="Search Connections"
        onFilterChange={value =>
          value.length ? setActiveIndex(0) : setActiveIndex(-1)
        }
        Node={({ item, index }) => (
          <ConnectionItem
            item={item}
            index={index}
            onActivate={() => props.onActivateItem(item.id)}
            onRemove={() => props.onRemoveItem(item.id)}
            onDisconnect={() => props.onDisconnectItem(item.id)}
          />
        )}
      >
        {/*
            TODO(ravicious): Change the type of FilterableList above to something like
            FilterableList<ExtendedTrackedConnection | Vnet> and render a different component in Node
            depending on the item type.

            We don't want to put VNet into ExtendedTrackedConnection because these are two
            fundamentally different things.
          */}
        <VnetConnection closePopover={props.closePopover} />
      </FilterableList>
    </Box>
  );
}

const VnetConnection = (props: { closePopover: () => void }) => {
  const { workspacesService } = useAppContext();
  const documentsService =
    workspacesService.getActiveWorkspaceDocumentService();
  const rootClusterUri = workspacesService.getRootClusterUri();

  return (
    <ListItem
      css={`
        padding: 6px 8px;
        height: unset;
      `}
      onClick={() => {
        documentsService.openVnetDocument({ rootClusterUri });
        props.closePopover();
      }}
    >
      <ConnectionStatusIndicator
        mr={3}
        css={`
          flex-shrink: 0;
        `}
        connected={true}
      />
      <Flex
        alignItems="center"
        justifyContent="space-between"
        flex="1"
        minWidth="0"
      >
        <div
          css={`
            min-width: 0;
          `}
        >
          <Text
            typography="body1"
            bold
            color="text.main"
            css={`
              line-height: 16px;
            `}
          >
            <span
              css={`
                vertical-align: middle;
              `}
            >
              VNet
            </span>
          </Text>
        </div>
        <ButtonIcon mr="-3px" title="Disconnect">
          <Unlink size={18} />
        </ButtonIcon>
      </Flex>
    </ListItem>
  );
};
