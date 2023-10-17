/*
Copyright 2023 Gravitational, Inc.

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
import { Flex, Box } from 'design';

import Select, { DarkStyledSelect, SelectCreatable, Option } from '../Select';

export default {
  title: 'Shared/SelectCreatable',
};

function SelectCreatableDefault() {
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
      <Box>
        <DarkStyledSelect>
          Single input (dark)
          <SelectCreatable
            placeholder="Example"
            inputValue={input}
            value={selected}
            onInputChange={v => setInput(v)}
            onChange={v => setSelected((v as Option[] | null) || [])}
          />
        </DarkStyledSelect>
      </Box>
    </Flex>
  );
}

export const Selects = () => {
  return (
    <Flex>
      <SelectCreatableDefault />
    </Flex>
  );
};
