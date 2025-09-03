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

import { Meta } from '@storybook/react-vite';
import { useState } from 'react';

import { Flex } from 'design';
import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';
import { wait } from 'shared/utils/wait';

import { requiredField } from '../Validation/rules';
import {
  FieldSelectAsync as FieldSelectAsyncComp,
  FieldSelect as FieldSelectComp,
} from './FieldSelect';
import {
  FieldSelectCreatable,
  FieldSelectCreatableAsync,
} from './FieldSelectCreatable';

type StoryProps = {
  readOnly?: boolean;
  isDisabled?: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Shared',
  component: FieldSelect,
  args: {
    readOnly: false,
    isDisabled: false,
  },
};
export default meta;

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

export function FieldSelect(props: StoryProps) {
  const [selectedOption, setSelectedOption] = useState<Option>(OPTIONS[0]);
  const [selectedOptions, setSelectedOptions] = useState<readonly Option[]>([
    OPTIONS[0],
    OPTIONS[1],
  ]);
  return (
    <Validation>
      {({ validator }) => {
        // Prevent rendering loop.
        if (!validator.state.validating) {
          validator.validate();
        }
        return (
          <Flex flexDirection="column">
            <FieldSelectComp
              label="FieldSelect with search"
              onChange={option => setSelectedOption(option)}
              value={selectedOption}
              isSearchable
              options={OPTIONS}
              helperText="And a helper text"
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
            />
            <FieldSelectComp
              label="FieldSelect with validation rule"
              onChange={option => setSelectedOption(option)}
              rule={noPenguinsAllowed}
              value={selectedOption}
              options={OPTIONS}
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
            />
            <FieldSelectComp
              label="FieldSelect, multi-select"
              isMulti
              options={OPTIONS}
              value={selectedOptions}
              onChange={setSelectedOptions}
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
            />
            <FieldSelectComp
              label="FieldSelect, multi-select, required, with tooltip"
              isMulti
              options={OPTIONS}
              value={selectedOptions}
              onChange={setSelectedOptions}
              rule={requiredField('Field is required')}
              required
              toolTipContent="I'm a tooltip."
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
            />
            <FieldSelectAsyncComp
              label="FieldSelectAsync with search"
              onChange={option => setSelectedOption(option)}
              value={selectedOption}
              isSearchable
              loadOptions={async input => {
                await wait(400);
                return OPTIONS.filter(o => o.label.includes(input));
              }}
              noOptionsMessage={() => 'No options'}
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
            />
            <FieldSelectAsyncComp
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
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
            />
            <FieldSelectAsyncComp
              label="FieldSelectAsync with error"
              onChange={undefined}
              value={undefined}
              isSearchable
              loadOptions={async () => {
                await wait(400);
                throw new Error('Network error');
              }}
              noOptionsMessage={() => 'No options'}
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
            />
            <FieldSelectAsyncComp
              label="Empty FieldSelectAsync"
              onChange={undefined}
              value={undefined}
              isSearchable
              loadOptions={async () => {
                await wait(400);
                return [];
              }}
              noOptionsMessage={() => 'No options'}
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
            />
            <FieldSelectCreatable
              label="FieldSelectCreatable, multi-select"
              isMulti
              onChange={setSelectedOptions}
              value={selectedOptions}
              isSearchable
              options={OPTIONS}
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
            />
            <FieldSelectCreatable
              label="FieldSelectCreatable, multi-select, with validation rule"
              isMulti
              rule={noPenguinsAllowedInArray}
              onChange={setSelectedOptions}
              value={selectedOptions}
              isSearchable
              options={OPTIONS}
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
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
              isDisabled={props.isDisabled}
              readOnly={props.readOnly}
            />
          </Flex>
        );
      }}
    </Validation>
  );
}

const OPTIONS = [
  { value: 'mac', label: 'Mac' },
  {
    value: 'windows',
    label: 'Windows',
  },
  { value: 'linux', label: 'Linux' },
  { value: 'mobile', label: 'Mobile' },
];
