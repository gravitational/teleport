/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';

import { Flex } from 'design';

import { View } from '../flow';

import { StepList } from './StepList';
import { StepItem } from './StepItem';

import type { ResourceSpec } from '../SelectResource';

interface NavigationProps {
  currentStep: number;
  selectedResource: ResourceSpec;
  views: View[];
}

const StyledNav = styled.div`
  display: flex;
`;

export function Navigation(props: NavigationProps) {
  let content;
  if (props.views) {
    content = (
      <Flex mt="10px" mb="45px">
        {/*
          This initial StepItem is to render the first "bullet"
          in this nav, which is the selected resource's icon
          and name.
        */}
        <StepItem selectedResource={props.selectedResource} />
        <StepList views={props.views} currentStep={props.currentStep} />
      </Flex>
    );
  }

  return <StyledNav>{content}</StyledNav>;
}
