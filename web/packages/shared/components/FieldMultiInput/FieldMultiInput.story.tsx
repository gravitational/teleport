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

import React, { useState } from 'react';

import Box from 'design/Box';

import { FieldMultiInput } from './FieldMultiInput';

export default {
  title: 'Shared',
};

export function Story() {
  const [items, setItems] = useState([]);
  return (
    <Box width="500px">
      <FieldMultiInput label="Some items" value={items} onChange={setItems} />
    </Box>
  );
}
Story.storyName = 'FieldMultiInput';
