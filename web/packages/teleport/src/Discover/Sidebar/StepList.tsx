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

import { Box } from 'design';

import { StepItem } from './StepItem';

import type { View } from 'teleport/Discover/flow';

interface StepListProps {
  views: View[];
  currentStep: number;
}

export function StepList(props: StepListProps) {
  const items = props.views.map((view, index) => (
    <StepItem key={index} view={view} currentStep={props.currentStep} />
  ));

  return (
    <Box style={{ marginLeft: 7 }} mt={2}>
      {items}
    </Box>
  );
}
