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
import StepSlider, { SliderProps } from './StepSlider';
import { Text, Card, ButtonPrimary, ButtonLink, Box } from 'design';

export default {
  title: 'Teleport/StepSlider',
};

const singleFlow = { default: [Body1, Body2] };
export const SingleStaticFlow = () => {
  return (
    <Card bg="primary.light" my="5" mx="auto" width={464}>
      <Text typography="h3" pt={5} textAlign="center" color="light">
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
const multiflows = {
  primary: [MainStep1, MainStep2, FinalStep],
  secondary: [OtherStep1, FinalStep],
};
export const MultiCardFlow = () => {
  const [flow, setFlow] = React.useState<keyof typeof multiflows>('primary');

  function onSwitchFlow(flow: keyof typeof multiflows) {
    setFlow(flow);
  }

  return (
    <Card as="form" bg="primary.light" my={6} mx="auto" width={464}>
      <StepSlider<typeof multiflows>
        flows={multiflows}
        currFlow={flow}
        onSwitchFlow={onSwitchFlow}
      />
    </Card>
  );
};

function MainStep1({ next, switchFlow, refCallback }: SliderProps<MultiFlow>) {
  return (
    <Box flex="3" p="6" ref={refCallback} data-testid="multi-primary1">
      <Text typography="h2" mb={3} textAlign="center" color="light" bold>
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
            switchFlow('secondary');
          }}
        >
          Switch Secondary Flow
        </ButtonLink>
      </Box>
    </Box>
  );
}

function MainStep2({ next, prev, refCallback }: SliderProps<MultiFlow>) {
  return (
    <Box flex="3" p="6" ref={refCallback} data-testid="multi-primary2">
      <Text typography="h2" mb={3} textAlign="center" color="light" bold>
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
    </Box>
  );
}

function OtherStep1({
  switchFlow: onSwitchFlow,
  next: onNext,
  refCallback,
}: SliderProps<MultiFlow>) {
  return (
    <Box flex="3" p="6" ref={refCallback} data-testid="multi-secondary1">
      <Text typography="h2" mb={3} textAlign="center" color="light" bold>
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
            onSwitchFlow('primary', true /* go back to primary flow */);
          }}
        >
          Switch Primary Flow
        </ButtonLink>
      </Box>
    </Box>
  );
}

function FinalStep({ prev: onPrev, refCallback }: SliderProps<MultiFlow>) {
  return (
    <Box flex="3" p="6" ref={refCallback} data-testid="multi-final">
      <Text typography="h2" mb={3} textAlign="center" color="light" bold>
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
            onPrev();
          }}
        >
          Go Back
        </ButtonLink>
      </Box>
    </Box>
  );
}

function Body1({
  next: onNext,
  prev: onPrev,
  refCallback,
  testProp,
}: SliderProps<any> & { testProp: string }) {
  return (
    <Box flex="3" p="6" ref={refCallback} data-testid="single-body1">
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
          onNext();
        }}
      >
        Next1
      </ButtonPrimary>
      <Box mt={5}>
        <ButtonLink
          onClick={e => {
            e.preventDefault();
            onPrev();
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
}: SliderProps<any> & { testProp: string }) {
  return (
    <Box flex="3" p="6" ref={refCallback} data-testid="single-body2">
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
