/**
 * Copyright 2023 Gravitational, Inc.
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

import { StepNavigation } from './StepNavigation';

export default {
  title: 'Teleport/StepNavigation',
};

const steps = [
  { title: 'first title' },
  { title: 'second title' },
  { title: 'third title' },
  { title: 'fourth title' },
  { title: 'fifth title' },
  { title: 'sixth title' },
  { title: 'seventh title' },
  { title: 'eighth title' },
];

export const Examples = () => {
  return (
    <>
      <Box mb={5}>
        <StepNavigation currentStep={0} steps={steps.slice(0, 2)} />
      </Box>
      <Box mb={5}>
        <StepNavigation currentStep={1} steps={steps.slice(0, 2)} />
      </Box>
      <Box mb={5}>
        <StepNavigation currentStep={2} steps={steps.slice(0, 2)} />
      </Box>
      <Box>
        <StepNavigation currentStep={3} steps={steps} />
      </Box>
    </>
  );
};
