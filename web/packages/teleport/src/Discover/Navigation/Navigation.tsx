/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
