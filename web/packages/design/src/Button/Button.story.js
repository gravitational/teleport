/*
Copyright 2019 Gravitational, Inc.

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
import Button, {
  ButtonPrimary,
  ButtonSecondary,
  ButtonWarning,
} from './Button';

export default {
  title: 'Design/Button',
};

export const Basics = () => (
  <>
    <div>
      <ButtonPrimary mr={3}>Primary</ButtonPrimary>
      <ButtonSecondary mr={3}>Secondary</ButtonSecondary>
      <ButtonWarning>Warning</ButtonWarning>
    </div>
    <div>
      <Button size="large" mr={3} mt={5}>
        Large
      </Button>
      <Button size="medium" mr={3}>
        Medium
      </Button>
      <Button size="small">Small</Button>
    </div>
  </>
);

export const Block = () => (
  <>
    <Button block mb={3}>
      Primary Block Button
    </Button>
    <ButtonSecondary block mb={3}>
      Secondary Block Button
    </ButtonSecondary>
    <ButtonWarning block>Warning Block Button</ButtonWarning>
  </>
);

export const States = () => (
  <>
    <Button mr={3} disabled>
      Disabled
    </Button>
    <Button autoFocus>Focused</Button>
  </>
);
