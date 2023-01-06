/*
Copyright 2022 Gravitational, Inc.

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
