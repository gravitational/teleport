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
import { Text } from 'design';

export default function StepList({ options, value }){
  const $steps = options.map( (option, index) => (
    <StepListItem active={value === option.value}
       title={`${index+1}. ${option.title}`} key={option.value}
    />
  ))

  return (
    <StyledStepList bold children={$steps} />
  )
}

const StepListItem = ({ title, active }) => (
  <StyledTabItem color="text.primary" active={active} typography="h3" mr={5} py="2" >
    {title}
  </StyledTabItem>
)

const StyledTabItem = styled(Text)`
  position: relative;
  &:last-child{
    margin-right: 0;
  }

  ${ ({ active, theme }) => {
    if( active ){
      return `
        &:after {
          background-color: ${theme.colors.accent};
          content: "";
          position: absolute;
          bottom: 0;
          left: 0;
          width: 100%;
          height: 4px;
      }
    `
    }
  }}
`

const StyledStepList = styled.div`
  min-width: 500px;
  display: flex;
  align-items: center;
  flex-shrink: 0;
  flex-wrap: wrap;
  flex: 1;
`