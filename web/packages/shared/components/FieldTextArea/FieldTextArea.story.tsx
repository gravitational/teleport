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

import Validation from '../../components/Validation';
import { requiredField } from '../Validation/rules';
import { FieldTextArea } from './FieldTextArea';

export default {
  title: 'Shared/FieldTextArea',
};

export const Fields = () => (
  <Validation>
    {({ validator }) => (
      <>
        <FieldTextArea
          label="Label"
          helperText="Optional helper text"
          name="optional name"
          onChange={() => {}}
          value={'value'}
          size="large"
        />
        <FieldTextArea
          label="Label with placeholder"
          name="optional name"
          onChange={() => {}}
          placeholder="placeholder"
          value={''}
          rule={requiredField('So required. Much mandatory.')}
        />
        <FieldTextArea
          label="Label with tooltip"
          name="optional name"
          onChange={() => {}}
          placeholder="placeholder"
          value={''}
          toolTipContent={<Text>Hello world</Text>}
        />
        <FieldTextArea
          label="Label with helper text and tooltip"
          helperText="Helper text"
          toolTipContent={<Text>Hello world</Text>}
          name="optional name"
          onChange={() => {}}
          placeholder="placeholder"
          value={''}
        />
        <FieldTextArea placeholder="without label" onChange={() => {}} />
        <FieldTextArea
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
