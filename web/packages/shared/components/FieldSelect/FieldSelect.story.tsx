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
import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';
import { wait } from 'shared/utils/wait';

import { requiredField } from '../Validation/rules';
import { FieldSelect, FieldSelectAsync } from './FieldSelect';
import {
  FieldSelectCreatable,
  FieldSelectCreatableAsync,
} from './FieldSelectCreatable';

export default {
  title: 'Shared/FieldSelect',
};

function noPenguinsAllowed(opt: Option) {
  return () =>
    opt.value !== 'linux'
      ? { valid: true }
      : { valid: false, message: 'No penguins allowed' };
}

function noPenguinsAllowedInArray(opt: Option[]) {
  return () =>
    opt.every(o => o.value !== 'linux')
      ? { valid: true }
      : { valid: false, message: 'No penguins allowed' };
}

export function Default() {
  const [selectedOption, setSelectedOption] = useState<Option>(OPTIONS[0]);
  const [selectedOptions, setSelectedOptions] = useState<readonly Option[]>([]);
  return (
    <Validation>
      {({ validator }) => {
        // Prevent rendering loop.
        if (!validator.state.validating) {
          validator.validate();
        }
        return (
          <Flex flexDirection="column">
            <FieldSelect
              label="FieldSelect with search"
              onChange={option => setSelectedOption(option)}
              value={selectedOption}
              isSearchable
              options={OPTIONS}
              helperText="And a helper text"
            />
            <FieldSelect
              label="FieldSelect with validation rule"
              onChange={option => setSelectedOption(option)}
              rule={noPenguinsAllowed}
              value={selectedOption}
              options={OPTIONS}
            />
            <FieldSelect
              label="FieldSelect, multi-select"
              isMulti
              options={OPTIONS}
              value={selectedOptions}
              onChange={setSelectedOptions}
            />
            <FieldSelect
              label="FieldSelect, multi-select, required, with tooltip"
              isMulti
              options={OPTIONS}
              value={selectedOptions}
              onChange={setSelectedOptions}
              rule={requiredField('Field is required')}
              required
              toolTipContent="I'm a tooltip."
            />
            <FieldSelectAsync
              label="FieldSelectAsync with search"
              onChange={option => setSelectedOption(option)}
              value={selectedOption}
              isSearchable
              loadOptions={async input => {
                await wait(400);
                return OPTIONS.filter(o => o.label.includes(input));
              }}
              noOptionsMessage={() => 'No options'}
            />
            <FieldSelectAsync
              label="FieldSelectAsync with search and validation rule"
              onChange={option => setSelectedOption(option)}
              rule={noPenguinsAllowed}
              value={selectedOption}
              isSearchable
              loadOptions={async input => {
                await wait(400);
                return OPTIONS.filter(o => o.label.includes(input));
              }}
              noOptionsMessage={() => 'No options'}
            />
            <FieldSelectAsync
              label="FieldSelectAsync with error"
              onChange={undefined}
              value={undefined}
              isSearchable
              loadOptions={async () => {
                await wait(400);
                throw new Error('Network error');
              }}
              noOptionsMessage={() => 'No options'}
            />
            <FieldSelectAsync
              label="Empty FieldSelectAsync"
              onChange={undefined}
              value={undefined}
              isSearchable
              loadOptions={async () => {
                await wait(400);
                return [];
              }}
              noOptionsMessage={() => 'No options'}
            />
            <FieldSelectCreatable
              label="FieldSelectCreatable, multi-select"
              isMulti
              onChange={setSelectedOptions}
              value={selectedOptions}
              isSearchable
              options={OPTIONS}
            />
            <FieldSelectCreatable
              label="FieldSelectCreatable, multi-select, with validation rule"
              isMulti
              rule={noPenguinsAllowedInArray}
              onChange={setSelectedOptions}
              value={selectedOptions}
              isSearchable
              options={OPTIONS}
            />
            <FieldSelectCreatableAsync
              label="FieldSelectCreatableAsync, multi-select"
              isMulti
              onChange={setSelectedOptions}
              value={selectedOptions}
              isSearchable
              defaultOptions={true}
              loadOptions={async input => {
                await wait(400);
                return OPTIONS.filter(o => o.label.includes(input));
              }}
              noOptionsMessage={() => 'No options'}
            />
          </Flex>
        );
      }}
    </Validation>
  );
}

Default.storyName = 'FieldSelect';

const OPTIONS = [
  { value: 'mac', label: 'Mac' },
  {
    value: 'windows',
    label: 'Windows',
  },
  { value: 'linux', label: 'Linux' },
];
