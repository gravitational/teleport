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
import { EmailSolid } from 'design/Icon';

import Validation from '../../components/Validation';
import { requiredEmailLike, requiredField } from '../Validation/rules';
import FieldInputComponent from './FieldInput';

type StoryProps = {
  readOnly?: boolean;
  disabled?: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Shared',
  component: FieldInput,
  args: {
    readOnly: false,
    disabled: false,
  },
};
export default meta;

export function FieldInput(props: StoryProps) {
  return (
    <Validation>
      {({ validator }) => (
        <>
          <FieldInputComponent
            label="Label"
            helperText="Optional bottom helper text"
            name="optional name"
            onChange={() => {}}
            value={'value'}
            icon={EmailSolid}
            size="large"
            rule={requiredEmailLike}
            disabled={props.disabled}
            readonly={props.readOnly}
          />
          <FieldInputComponent
            label="Label with placeholder"
            name="optional name"
            onChange={() => {}}
            placeholder="placeholder"
            value={''}
            disabled={props.disabled}
            readonly={props.readOnly}
          />
          <FieldInputComponent
            label="Label with tooltip"
            name="optional name"
            onChange={() => {}}
            placeholder="placeholder"
            value={''}
            toolTipContent={<Text>Hello world</Text>}
            disabled={props.disabled}
            readonly={props.readOnly}
          />
          <FieldInputComponent
            label="Label with helper text and tooltip"
            helperText="Bottom helper text"
            toolTipContent={<Text>Hello world</Text>}
            name="optional name"
            onChange={() => {}}
            placeholder="placeholder"
            value={''}
            disabled={props.disabled}
            readonly={props.readOnly}
          />
          <FieldInputComponent
            placeholder="without label"
            onChange={() => {}}
            disabled={props.disabled}
            readonly={props.readOnly}
          />
          <FieldInputComponent
            label="Required"
            rule={requiredField('So required. Much mandatory.')}
            required
            onChange={() => {}}
            value=""
            disabled={props.disabled}
            readonly={props.readOnly}
          />
          <ButtonPrimary onClick={() => validator.validate()}>
            Validate
          </ButtonPrimary>
        </>
      )}
    </Validation>
  );
}
