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

import { ButtonToggle } from './ButtonToggle';

export default {
  title: 'Design/Button',
};

export const Toggle = () => {
  // first toggle: left button is "true"
  const [leftIsTrueValue, setLeftIsTrueValue] = useState(true);
  // second toggle: right button is "true"
  const [rightIsTrueValue, setRightIsTrueValue] = useState(true);

  return (
    <Flex flexDirection="column" gap={4}>
      <Flex flexDirection="row" gap={4}>
        <ButtonToggle
          leftLabel="ON"
          rightLabel="OFF"
          initialValue={leftIsTrueValue}
          onChange={setLeftIsTrueValue}
        />
        <H2>{`is "ON": ${leftIsTrueValue}`}</H2>
      </Flex>
      <Flex flexDirection="row" gap={4}>
        <ButtonToggle
          leftLabel="OFF"
          rightLabel="ON"
          initialValue={rightIsTrueValue}
          rightIsTrue
          onChange={setRightIsTrueValue}
        />
        <H2>{`is "ON": ${rightIsTrueValue}`}</H2>
      </Flex>
    </Flex>
  );
};
