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

import { Meta } from '@storybook/react-vite';

import { ButtonPrimary, Text } from 'design';

import Validation from '../../components/Validation';
import { requiredField } from '../Validation/rules';
import { FieldTextArea as Component } from './FieldTextArea';

type StoryProps = {
  readonly?: boolean;
  disabled?: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Shared',
  component: FieldTextArea,
  args: {
    readonly: false,
    disabled: false,
  },
};
export default meta;

export function FieldTextArea(props: StoryProps) {
  return (
    <Validation>
      {({ validator }) => (
        <>
          <Component
            label="Label"
            helperText="Optional helper text"
            name="optional name"
            onChange={() => {}}
            value={'some value lorem ipsum dolores'}
            size="large"
            disabled={props.disabled}
            readonly={props.readonly}
          />
          <Component
            label="Label with placeholder"
            name="optional name"
            onChange={() => {}}
            placeholder="placeholder"
            value={''}
            rule={requiredField('So required. Much mandatory.')}
            required
            disabled={props.disabled}
            readonly={props.readonly}
          />
          <Component
            label="Label with tooltip"
            name="optional name"
            onChange={() => {}}
            placeholder="placeholder"
            value={''}
            tooltipContent={<Text>Hello world</Text>}
            disabled={props.disabled}
            readonly={props.readonly}
          />
          <Component
            label="Label with helper text and tooltip"
            helperText="Helper text"
            tooltipContent={<Text>Hello world</Text>}
            name="optional name"
            onChange={() => {}}
            placeholder="placeholder"
            value={''}
            disabled={props.disabled}
            readonly={props.readonly}
          />
          <Component
            placeholder="without label"
            onChange={() => {}}
            disabled={props.disabled}
            readonly={props.readonly}
          />
          <Component
            label="Required"
            required
            rule={requiredField('So required. Much mandatory.')}
            onChange={() => {}}
            value=""
            disabled={props.disabled}
            readonly={props.readonly}
          />
          <ButtonPrimary onClick={() => validator.validate()}>
            Validate
          </ButtonPrimary>
        </>
      )}
    </Validation>
  );
}
