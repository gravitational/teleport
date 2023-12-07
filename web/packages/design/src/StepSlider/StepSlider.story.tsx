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

import React, { useState } from 'react';

import { Box, ButtonLink, ButtonPrimary, Text, Card } from 'design';

import { OnboardCard } from 'design/Onboard/OnboardCard';

import { NewFlow, StepComponentProps, StepSlider } from './StepSlider';

export default {
  title: 'Design/StepSlider',
};

const singleFlow = { default: [Body1, Body2] };
export const SingleFlowInPlaceSlider = () => {
  return (
    <Card my="5" mx="auto" width={464}>
      <Text typography="h3" pt={5} textAlign="center" color="text.main">
        Static Title
      </Text>
      <StepSlider<typeof singleFlow>
        flows={singleFlow}
        currFlow={'default'}
        testProp="I'm that test prop"
      />
    </Card>
  );
};

type MultiFlow = 'primary' | 'secondary';
type ViewProps = StepComponentProps & {
  changeFlow(f: NewFlow<MultiFlow>): void;
};
const multiflows = {
  primary: [MainStep1, MainStep2, FinalStep],
  secondary: [OtherStep1, FinalStep],
};
export const MultiFlowWheelSlider = () => {
  const [flow, setFlow] = useState<MultiFlow>('primary');
  const [newFlow, setNewFlow] = useState<NewFlow<MultiFlow>>();

  function onSwitchFlow(flow: MultiFlow) {
    setFlow(flow);
  }

  function onNewFlow(newFlow: NewFlow<MultiFlow>) {
    setNewFlow(newFlow);
  }

  return (
    <StepSlider<typeof multiflows>
      flows={multiflows}
      currFlow={flow}
      onSwitchFlow={onSwitchFlow}
      newFlow={newFlow}
      changeFlow={onNewFlow}
    />
  );
};

function MainStep1({ next, refCallback, changeFlow }: ViewProps) {
  return (
    <OnboardCard ref={refCallback} data-testid="multi-primary1">
      <Text typography="h2" mb={3} textAlign="center" color="text.main" bold>
        First Step
      </Text>
      <Text mb={3}>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod
        tempor incididunt ut labore et dolore magna aliqua.
      </Text>
      <ButtonPrimary
        width="100%"
        mt={3}
        size="large"
        onClick={e => {
          e.preventDefault();
          next();
        }}
      >
        Next
      </ButtonPrimary>
      <Box mt={5}>
        <ButtonLink
          onClick={e => {
            e.preventDefault();
            changeFlow({ flow: 'secondary' });
          }}
        >
          Switch Secondary Flow
        </ButtonLink>
      </Box>
    </OnboardCard>
  );
}

function MainStep2({ next, prev, refCallback }: ViewProps) {
  return (
    <OnboardCard ref={refCallback} data-testid="multi-primary2">
      <Text typography="h2" mb={3} textAlign="center" color="text.main" bold>
        Second Step
      </Text>
      <Text mb={3}>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod
        tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim
        veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea
        commodo consequat.
      </Text>
      <Text mb={3}>
        Duis aute irure dolor in reprehenderit in voluptate velit esse cillum
        dolore eu fugiat nulla pariatur.
      </Text>
      <Text mb={3}>
        Duis aute irure dolor in reprehenderit in voluptate velit esse cillum
        dolore eu fugiat nulla pariatur.
      </Text>
      <Text mb={3}>
        Duis aute irure dolor in reprehenderit in voluptate velit esse cillum
        dolore eu fugiat nulla pariatur.
      </Text>
      <Text mb={3}>
        Duis aute irure dolor in reprehenderit in voluptate velit esse cillum
        dolore eu fugiat nulla pariatur.
      </Text>
      <ButtonPrimary
        width="100%"
        mt={3}
        size="large"
        onClick={e => {
          e.preventDefault();
          next();
        }}
      >
        Next
      </ButtonPrimary>
      <Box mt={5}>
        <ButtonLink
          onClick={e => {
            e.preventDefault();
            prev();
          }}
        >
          Go Back
        </ButtonLink>
      </Box>
    </OnboardCard>
  );
}

function OtherStep1({ changeFlow, next: onNext, refCallback }: ViewProps) {
  return (
    <OnboardCard ref={refCallback} data-testid="multi-secondary1">
      <Text typography="h2" mb={3} textAlign="center" color="text.main" bold>
        Some Other Flow Title
      </Text>
      <Text mb={3}>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod
        tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim
        veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea
        commodo consequat.
      </Text>
      <ButtonPrimary
        width="100%"
        mt={3}
        size="large"
        onClick={e => {
          e.preventDefault();
          onNext();
        }}
      >
        Next
      </ButtonPrimary>
      <Box mt={5}>
        <ButtonLink
          onClick={e => {
            e.preventDefault();
            changeFlow({ flow: 'primary', applyNextAnimation: true });
          }}
        >
          Switch Primary Flow
        </ButtonLink>
      </Box>
    </OnboardCard>
  );
}

function FinalStep({ prev, refCallback }: ViewProps) {
  return (
    <OnboardCard ref={refCallback} data-testid="multi-final">
      <Text typography="h2" mb={3} textAlign="center" color="text.main" bold>
        Done Step
      </Text>
      <Text mb={3}>
        Duis aute irure dolor in reprehenderit in voluptate velit esse cillum
        dolore eu fugiat nulla pariatur.
      </Text>
      <Text mb={3}>
        Duis aute irure dolor in reprehenderit in voluptate velit esse cillum
        dolore eu fugiat nulla pariatur.
      </Text>
      <Box mt={5}>
        <ButtonLink
          onClick={e => {
            e.preventDefault();
            prev();
          }}
        >
          Go Back
        </ButtonLink>
      </Box>
    </OnboardCard>
  );
}

function Body1({
  next,
  prev,
  refCallback,
  testProp,
}: StepComponentProps & { testProp: string }) {
  return (
    <Box p={6} ref={refCallback} data-testid="single-body1">
      <Text mb={3}>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod
        tempor incididunt ut labore et dolore magna aliqua.
      </Text>
      <Text mb={6}>{testProp}</Text>
      <ButtonPrimary
        width="100%"
        size="large"
        onClick={e => {
          e.preventDefault();
          next();
        }}
      >
        Next1
      </ButtonPrimary>
      <Box mt={5}>
        <ButtonLink
          onClick={e => {
            e.preventDefault();
            prev();
          }}
        >
          Back1
        </ButtonLink>
      </Box>
    </Box>
  );
}

function Body2({
  prev: onPrev,
  next: onNext,
  refCallback,
  testProp,
}: StepComponentProps & { testProp: string }) {
  return (
    <Box p={6} ref={refCallback} data-testid="single-body2">
      <Text mb={3}>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod
        tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim
        veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea
        commodo consequat.
      </Text>
      <Text mb={3}>
        Duis aute irure dolor in reprehenderit in voluptate velit esse cillum
        dolore eu fugiat nulla pariatur.
      </Text>
      <Text mb={6}>{testProp}</Text>
      <ButtonPrimary
        width="100%"
        size="large"
        onClick={e => {
          e.preventDefault();
          onPrev();
        }}
      >
        Back2
      </ButtonPrimary>
      <Box mt={5}>
        <ButtonLink
          onClick={e => {
            e.preventDefault();
            onNext();
          }}
        >
          Next2
        </ButtonLink>
      </Box>
    </Box>
  );
}
