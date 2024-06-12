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

import { Box } from 'design';

import { CheckboxWrapper, CheckboxInput } from './Checkbox';

export default {
  title: 'Design/Checkbox',
};

export const Checkbox = () => (
  <Box>
    <CheckboxWrapper key={1}>
      <CheckboxInput type="checkbox" name="input1" id={'input1'} />
      Input 1
    </CheckboxWrapper>
    <CheckboxWrapper key={2}>
      <CheckboxInput type="checkbox" name="input2" id={'input2'} />
      Input 2
    </CheckboxWrapper>
    <CheckboxWrapper key={3}>
      <CheckboxInput type="checkbox" name="input3" id={'input3'} />
      Input 3
    </CheckboxWrapper>
  </Box>
);
