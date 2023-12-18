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
import { Flex, Box } from 'design';

import { SelectCreatable, Option } from '../Select';

export default {
  title: 'Shared/SelectCreatable',
};

export const Selects = () => {
  const [input, setInput] = React.useState('');
  const [selected, setSelected] = React.useState<Option[]>();

  return (
    // Note that these examples don't provide for great UX. Implementations
    // should consider adding an `onKeyDown` handler to split entries on
    // keypress (tab, comma, enter, etc) rather than relying on react-select's
    // "Create [foo]" dropdown.
    <Flex flexDirection="column" width="330px" mr={5}>
      <Box mb="200px">
        Multiple input
        <SelectCreatable
          placeholder="Example"
          isMulti
          isClearable
          isSearchable
          inputValue={input}
          value={selected}
          onInputChange={v => setInput(v)}
          onChange={v => setSelected((v as Option[] | null) || [])}
        />
        Note: accept new candidate with Enter or mouse click
      </Box>
      <Box mb="200px">
        Single input
        <SelectCreatable
          placeholder="Example"
          inputValue={input}
          value={selected}
          onInputChange={v => setInput(v)}
          onChange={v => setSelected((v as Option[] | null) || [])}
        />
      </Box>
    </Flex>
  );
};
