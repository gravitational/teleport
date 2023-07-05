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

export default function TrustedListItem(props: Props) {
  const { name, id, onEdit, onDelete, ...rest } = props;
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
      pt="4"
      pb="5"
      {...rest}
    >
      <Flex width="100%" justifyContent="center">
        <MenuIcon buttonIconProps={menuActionProps}>
          <MenuItem onClick={onClickDelete}>Delete...</MenuItem>
        </MenuIcon>
      </Flex>
      <Flex
        flex="1"
        mb="3"
        alignItems="center"
        justifyContent="center"
        flexDirection="column"
      >
        <Icons.LanAlt
          my="4"
          style={{ textAlign: 'center' }}
          fontSize="48px"
          color="text.primary"
        />
        <Text
          typography="p"
          bold
          caps
          mb="1"
          textAlign="center"
          title={name}
          style={{ width: '200px' }}
        >
          {name}
        </Text>
      </Flex>
      <ButtonPrimary mt="auto" px="1" size="medium" block onClick={onClickEdit}>
        EDIT TRUSTED CLUSTER
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
  name: string;
  id: string;
  onEdit: (id: string) => void;
  onDelete: (id: string) => void;
  [index: string]: any;
};
