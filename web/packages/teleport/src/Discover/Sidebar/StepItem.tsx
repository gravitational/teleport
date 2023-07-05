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

import { Box, Text } from 'design';
import styled from 'styled-components';

import { hasActiveChildren } from 'teleport/Discover/flow';

import { StepList } from './StepList';

import type { View } from 'teleport/Discover/flow';

interface StepItemProps {
  view: View;
  currentStep: number;
}

export function StepItem(props: StepItemProps) {
  if (props.view.hide) {
    return null;
  }

  let list;
  let isActive = props.currentStep === props.view.index;
  if (props.view.views) {
    list = (
      <Box ml={2}>
        <StepList views={props.view.views} currentStep={props.currentStep} />
      </Box>
    );

    if (!isActive) {
      isActive = hasActiveChildren(props.view.views, props.currentStep);
    }
  }

  const isDone = props.currentStep > props.view.index;

  return (
    <StepsContainer active={isDone || isActive}>
      <StepTitle>
        {getBulletIcon(isDone, isActive)}

        {props.view.title}
      </StepTitle>

      {list}
    </StepsContainer>
  );
}

function getBulletIcon(isDone: boolean, isActive: boolean) {
  if (isActive) {
    return <ActiveBullet />;
  }

  if (isDone) {
    return <CheckedBullet />;
  }

  return <Bullet />;
}

const StepTitle = styled.div`
  display: flex;
  align-items: center;
`;

const Bullet = styled.span`
  height: 14px;
  width: 14px;
  border: 1px solid #9b9b9b;
  border-radius: 50%;
  margin-right: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
`;

const ActiveBullet = styled(Bullet)`
  border-color: ${props => props.theme.colors.brand.main};
  background: ${props => props.theme.colors.brand.main};

  :before {
    content: '';
    height: 8px;
    width: 8px;
    border-radius: 50%;
    border: 2px solid ${props => props.theme.colors.levels.surfaceSecondary};
  }
`;

const CheckedBullet = styled(Bullet)`
  border-color: ${props => props.theme.colors.brand.main};
  background: ${props => props.theme.colors.brand.main};

  :before {
    content: '✓';
  }
`;

const StepsContainer = styled<{ active: boolean }>(Text)`
  display: flex;
  flex-direction: column;
  color: ${p => (p.active ? 'inherit' : p.theme.colors.text.secondary)};
  margin-bottom: 8px;
`;
