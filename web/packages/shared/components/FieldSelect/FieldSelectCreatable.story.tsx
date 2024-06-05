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

import { useState } from 'react';
import { Flex } from 'design';

import { wait } from 'shared/utils/wait';
import Validation from 'shared/components/Validation';
import { Option } from 'shared/components/Select';

import {
  FieldSelectCreatable,
  FieldSelectCreatableAsync,
} from './FieldSelectCreatable';

export default {
  title: 'Shared/FieldSelectCreatable',
};

export function Default() {
  const [selectedOptions, setSelectedOption] = useState<Option[]>([OPTIONS[0]]);
  return (
    <Validation>
      {() => (
        <Flex flexDirection="column">
          <FieldSelectCreatable
            inputId="test1"
            label="FieldSelectCreatable multi"
            onChange={option => setSelectedOption(option as Option[])}
            value={selectedOptions}
            isSearchable
            options={OPTIONS}
            isMulti
          />
          <FieldSelectCreatableAsync
            inputId="test2"
            label="FieldSelectCreatableAsync multi"
            onChange={option => setSelectedOption(option as Option[])}
            value={selectedOptions}
            isSearchable
            isMulti
            loadOptions={async input => {
              await wait(400);
              return OPTIONS.filter(o => o.label.toLowerCase().includes(input));
            }}
            defaultOptions={true}
            noOptionsMessage={() => 'No options'}
          />
          <FieldSelectCreatableAsync
            inputId="test3"
            label="FieldSelectCreatableAsync with error"
            onChange={undefined}
            value={undefined}
            isSearchable
            isMulti
            defaultOptions={true}
            loadOptions={async () => {
              await wait(400);
              return Promise.reject('Network error');
            }}
            noOptionsMessage={() => 'No options'}
          />
        </Flex>
      )}
    </Validation>
  );
}

Default.storyName = 'FieldSelectCreatable';

const OPTIONS = [
  { value: 'mac', label: 'Mac' },
  {
    value: 'windows',
    label: 'Windows',
  },
  { value: 'linux', label: 'Linux' },
];
