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

import Text from '../Text';

export default {
  title: 'Design/Text',
};

export const FontSizes = () => (
  <>
    <Text as="h1" typography="h1">
      h1
    </Text>
    <Text as="h2" typography="h2">
      h2
    </Text>
    <Text as="h3" typography="h3">
      h3
    </Text>
    <Text as="h4" typography="h4">
      h4
    </Text>
    <Text as="h5" typography="h5">
      h5
    </Text>
    <Text as="h6" typography="h6">
      h6
    </Text>
  </>
);

export const FontAttributes = () => (
  <>
    <Text regular mb={3}>
      Hello Regular
    </Text>
    <Text bold mb={3}>
      Hello Bold
    </Text>
    <Text caps mb={3}>
      Hello Caps
    </Text>
    <Text italic>Hello italic</Text>
  </>
);

export const FontColor = () => (
  <>
    <Text color="blue">Hello Blue</Text>
    <Text color="green">Hello Green</Text>
  </>
);

export const Alignments = () => (
  <>
    <Text textAlign="left">Hello Left</Text>
    <Text textAlign="center">Hello Center</Text>
    <Text textAlign="right">Hello Right</Text>
  </>
);
