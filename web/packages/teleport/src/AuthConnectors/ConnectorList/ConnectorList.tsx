/*
Copyright 2020 Gravitational, Inc.

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
import { Text, Flex, ButtonPrimary } from 'design';
import * as Icons from 'design/Icon';
import { MenuIcon, MenuItem } from 'shared/components/MenuAction';

import { State as ResourceState } from 'teleport/components/useResources';

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
    <Flex flexWrap="wrap" alignItems="center" flex={1}>
      {$items}
    </Flex>
  );
}

function ConnectorListItem({ name, id, onEdit, onDelete }) {
  const onClickEdit = () => onEdit(id);
  const onClickDelete = () => onDelete(id);

  return (
    <Flex
      style={{
        position: 'relative',
        boxShadow: '0 8px 32px rgba(0, 0, 0, 0.24)',
      }}
      width="240px"
      height="240px"
      borderRadius="3"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      bg="levels.surface"
      px="5"
      pt="2"
      pb="5"
      mb={4}
      mr={5}
    >
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
        <Icons.Github
          style={{ textAlign: 'center' }}
          fontSize="50px"
          color="text.primary"
          mb={3}
          mt={3}
        />
        <Text style={{ width: '100%' }} typography="body2" bold caps>
          {name}
        </Text>
      </Flex>
      <ButtonPrimary mt="auto" size="medium" block onClick={onClickEdit}>
        EDIT CONNECTOR
      </ButtonPrimary>
    </Flex>
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
