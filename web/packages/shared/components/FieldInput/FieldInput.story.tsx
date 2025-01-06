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

import { ButtonPrimary, Text } from 'design';
import { EmailSolid } from 'design/Icon';

import Validation from '../../components/Validation';
import { requiredEmailLike, requiredField } from '../Validation/rules';
import FieldInput from './FieldInput';

export default {
  title: 'Shared',
};

export const Fields = () => (
  <Validation>
    {({ validator }) => (
      <>
        <FieldInput
          label="Label"
          helperText="Optional helper text"
          name="optional name"
          onChange={() => {}}
          value={'value'}
          icon={EmailSolid}
          size="large"
          rule={requiredEmailLike}
        />
        <FieldInput
          label="Label with placeholder"
          name="optional name"
          onChange={() => {}}
          placeholder="placeholder"
          value={''}
        />
        <FieldInput
          label="Label with tooltip"
          name="optional name"
          onChange={() => {}}
          placeholder="placeholder"
          value={''}
          toolTipContent={<Text>Hello world</Text>}
        />
        <FieldInput
          label="Label with helper text and tooltip"
          helperText="Helper text"
          toolTipContent={<Text>Hello world</Text>}
          name="optional name"
          onChange={() => {}}
          placeholder="placeholder"
          value={''}
        />
        <FieldInput placeholder="without label" onChange={() => {}} />
        <FieldInput
          label="Required"
          rule={requiredField('So required. Much mandatory.')}
          onChange={() => {}}
          value=""
        />
        <ButtonPrimary onClick={() => validator.validate()}>
          Validate
        </ButtonPrimary>
      </>
    )}
  </Validation>
);

Fields.storyName = 'FieldInput';
