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
import { Flex, Text } from 'design'

export const AppLayout = styled(Flex)`
  position: absolute;
  margin: 0 auto;
  width: 100%;
  height: 100%;
  color: ${({theme}) => theme.colors.primary.contrastText};
`

export function StepLayout({ title, children, ...styles}){
  return (
    <Flex flexDirection="column" {...styles}>
      { title && ( <Text mb="4" typography="h1" style={{flexShrink: "0"}}> {title} </Text> )}
      {children}
    </Flex>
  )
}