/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { components, OptionProps } from 'react-select';

import { Flex, Text } from 'design';
import { Option as BaseOption } from 'shared/components/Select';

export type Option = BaseOption & {
  isAdded?: boolean;
  kind: 'app' | 'user_group' | 'namespace' | 'aws_ic_account_assignment';
};

export const CheckableOptionComponent = (
  props: OptionProps<Option> & { data: Option }
) => {
  const { data } = props;
  return (
    <components.Option {...props}>
      <Flex alignItems="center" py="8px" px="12px">
        <input
          type="checkbox"
          checked={data.isAdded || props.isSelected}
          readOnly
          name={data.value}
          id={data.value}
        />{' '}
        <Text ml={1}>{data.label}</Text>
      </Flex>
    </components.Option>
  );
};
