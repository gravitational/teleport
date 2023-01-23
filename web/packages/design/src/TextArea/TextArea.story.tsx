/**
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

import { TextArea } from './TextArea';

export default {
  title: 'Design/TextArea',
};

export const TextAreas = () => (
  <>
    <TextArea mb={4} placeholder="Enter Some long text" />
    <TextArea mb={4} hasError={true} defaultValue="This field has an error" />
    <TextArea
      mb={4}
      resizable={true}
      defaultValue="This field is resizable vertically"
    />
  </>
);
