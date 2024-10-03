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

import { Text } from 'design';

import Validation from '../../components/Validation';

import FieldInput from './FieldInput';

export default {
  title: 'Shared',
};

export const Fields = () => (
  <Validation>
    {() => (
      <>
        <FieldInput
          mb="6"
          label="Label"
          labelTip="Optional tabel tip"
          name="optional name"
          onChange={() => {}}
          value={'value'}
        />
        <FieldInput
          mb="6"
          label="Label with placeholder"
          name="optional name"
          onChange={() => {}}
          placeholder="placeholder"
          value={''}
        />
        <FieldInput
          mb="6"
          label="Label with tooltip"
          name="optional name"
          onChange={() => {}}
          placeholder="placeholder"
          value={''}
          toolTipContent={<Text>Hello world</Text>}
        />
        <FieldInput
          mb="6"
          label="Label with labeltip and tooltip"
          labelTip="the label tip"
          toolTipContent={<Text>Hello world</Text>}
          name="optional name"
          onChange={() => {}}
          placeholder="placeholder"
          value={''}
        />
        <FieldInput
          mb="6"
          placeholder="without label"
          validator={() => false}
          onChange={() => {}}
        />
      </>
    )}
  </Validation>
);

Fields.storyName = 'FieldInput';
