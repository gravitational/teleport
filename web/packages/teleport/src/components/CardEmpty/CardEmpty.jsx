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
import { Text, Box } from 'design';

export default function CardEmpty(props) {
  const { title, children, ...styles } = props;
  return (
    <Box
      bg="levels.surface"
      p="4"
      width="100%"
      m="0 auto"
      maxWidth="600px"
      textAlign="center"
      color="text.primary"
      style={{ borderRadius: '6px' }}
      {...styles}
    >
      <Text typography="h4">{title}</Text>
      {children}
    </Box>
  );
}
