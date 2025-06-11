/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { useState } from 'react';

import Flex from 'design/Flex';
import { H2 } from 'design/Text';

import { ButtonSelect } from './ButtonSelect';

export default {
  title: 'Design/ButtonSelect',
};

const twoOptions = [
  { key: '1', label: 'Option 1' },
  { key: '2', label: 'Option 2' },
];

const fourOptions = [
  { key: '1', label: 'Option 1' },
  { key: '2', label: 'Option 2' },
  { key: '3', label: 'Option 3' },
  { key: '4', label: 'Option 4' },
];

export const Toggle = () => {
  const [twoOptionsIndex, setTwoOptionsIndex] = useState(0);
  const [fourOptionsIndex, setFourOptionsIndex] = useState(0);

  return (
    <Flex flexDirection="column" gap={4}>
      <Flex alignItems="center" gap={3}>
        <ButtonSelect
          options={twoOptions}
          activeIndex={twoOptionsIndex}
          onChange={selectedIndex => {
            setTwoOptionsIndex(selectedIndex);
          }}
        />
        <H2>{twoOptionsIndex + 1}</H2>
      </Flex>
      <Flex alignItems="center" gap={3}>
        <ButtonSelect
          options={fourOptions}
          activeIndex={fourOptionsIndex}
          onChange={selectedIndex => {
            setFourOptionsIndex(selectedIndex);
          }}
        />
        <H2>{fourOptionsIndex + 1}</H2>
      </Flex>
    </Flex>
  );
};
