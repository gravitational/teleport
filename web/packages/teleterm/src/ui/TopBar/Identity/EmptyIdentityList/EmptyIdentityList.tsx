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

import { ButtonPrimary, Flex, ResourceIcon, Text } from 'design';

interface EmptyIdentityListProps {
  onConnect(): void;
}

export function EmptyIdentityList(props: EmptyIdentityListProps) {
  return (
    <Flex
      m="auto"
      flexDirection="column"
      alignItems="center"
      width="200px"
      p={3}
    >
      <ResourceIcon width="60px" name="server" />
      <Text typography="body3" bold mb={2}>
        No cluster connected
      </Text>
      <ButtonPrimary size="small" onClick={props.onConnect}>
        Connect
      </ButtonPrimary>
    </Flex>
  );
}
