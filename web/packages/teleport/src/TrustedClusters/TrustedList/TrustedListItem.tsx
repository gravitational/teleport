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

import { ButtonPrimary, Flex, Text } from 'design';
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
        <Icons.Lan
          my="4"
          style={{ textAlign: 'center' }}
          size={48}
          color="text.main"
        />
        <Text
          typography="h3"
          mb="1"
          textAlign="center"
          title={name}
          style={{ width: '200px' }}
        >
          {name}
        </Text>
      </Flex>
      <ButtonPrimary mt="auto" px="1" size="medium" block onClick={onClickEdit}>
        Edit Trusted Cluster
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
