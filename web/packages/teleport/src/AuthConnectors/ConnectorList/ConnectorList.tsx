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
import { Box, ButtonPrimary, Flex, Text } from 'design';
import { MenuIcon, MenuItem } from 'shared/components/MenuAction';
import { GitHubIcon } from 'design/SVGIcon';

import { State as ResourceState } from 'teleport/components/useResources';

import { ResponsiveConnector } from 'teleport/AuthConnectors/styles/ConnectorBox.styles';

import { State as AuthConnectorState } from '../useAuthConnectors';

export default function ConnectorList({ items, onEdit, onDelete }: Props) {
  items = items || [];
  const $items = items.map(item => {
    const { id, name } = item;
    return (
      <ConnectorListItem
        key={id}
        id={id}
        onEdit={onEdit}
        onDelete={onDelete}
        name={name}
      />
    );
  });

  return (
    <Flex flexWrap="wrap" alignItems="center" flex={1} gap={5}>
      {$items}
    </Flex>
  );
}

function ConnectorListItem({ name, id, onEdit, onDelete }) {
  const onClickEdit = () => onEdit(id);
  const onClickDelete = () => onDelete(id);

  return (
    <ResponsiveConnector>
      <Flex width="100%" justifyContent="center">
        <MenuIcon buttonIconProps={menuActionProps}>
          <MenuItem onClick={onClickDelete}>Delete...</MenuItem>
        </MenuIcon>
      </Flex>
      <Flex
        flex="1"
        alignItems="center"
        justifyContent="center"
        flexDirection="column"
        width="200px"
        style={{ textAlign: 'center' }}
      >
        <Box mb={3} mt={3}>
          <GitHubIcon style={{ textAlign: 'center' }} size={50} />
        </Box>
        <Text style={{ width: '100%' }} typography="body2" bold caps>
          {name}
        </Text>
      </Flex>
      <ButtonPrimary mt="auto" size="medium" block onClick={onClickEdit}>
        Edit Connector
      </ButtonPrimary>
    </ResponsiveConnector>
  );
}

const menuActionProps = {
  style: {
    right: '10px',
    position: 'absolute',
    top: '10px',
  },
};

type Props = {
  items: AuthConnectorState['items'];
  onEdit: ResourceState['edit'];
  onDelete: ResourceState['remove'];
};
