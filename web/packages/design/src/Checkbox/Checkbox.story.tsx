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

import { Box } from 'design';

import { CheckboxWrapper, CheckboxInput } from './Checkbox';

export default {
  title: 'Design/Checkbox',
};

export const Checkbox = () => (
  <Box>
    <CheckboxWrapper key={1}>
      <CheckboxInput type="checkbox" name="input1" id={'input1'} />
      Input 1
    </CheckboxWrapper>
    <CheckboxWrapper key={2}>
      <CheckboxInput type="checkbox" name="input2" id={'input2'} />
      Input 2
    </CheckboxWrapper>
    <CheckboxWrapper key={3}>
      <CheckboxInput type="checkbox" name="input3" id={'input3'} />
      Input 3
    </CheckboxWrapper>
  </Box>
);
