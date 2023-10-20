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
import { Flex } from 'design';

const Document: React.FC<{ visible: boolean; [x: string]: any }> = ({
  visible,
  children,
  ...styles
}) => (
  <Flex
    flex="1"
    style={{
      overflow: 'auto',
      display: visible ? 'flex' : 'none',
      position: 'relative',
    }}
    {...styles}
  >
    {children}
  </Flex>
);

export default Document;
