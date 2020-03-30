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
import styled from 'styled-components';
import { Text, Box } from 'design';

export default function CardEmpty(props) {
  const { title, children, ...styles } = props;
  return (
    <Container
      p="8"
      m="0 auto"
      width="100%"
      maxWidth="600px"
      textAlign="center"
      color="text.primary"
      bg="primary.light"
      {...styles}
    >
      <Text typography="h2" mb="3">
        {title}
      </Text>
      {children}
    </Container>
  );
}

const Container = styled(Box)`
  border-radius: 12px;
`;
