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

import React, { useState } from 'react';

import { Text, Card, ButtonPrimary, ButtonLink, Box } from 'design';

import { StepSlider, StepComponentProps, NewFlow } from './StepSlider';

export default {
  title: 'Design/StepSlider',
};

const singleFlow = { default: [Body1, Body2] };
export const SingleStaticFlow = () => {
  return (
    <Card bg="levels.surface" my="5" mx="auto" width={464}>
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
type ViewProps = StepComponentProps & {
  changeFlow(f: NewFlow<MultiFlow>): void;
};
const multiflows = {
  primary: [MainStep1, MainStep2, FinalStep],
  secondary: [OtherStep1, FinalStep],
};
export const MultiCardFlow = () => {
  const [flow, setFlow] = useState<MultiFlow>('primary');
  const [newFlow, setNewFlow] = useState<NewFlow<MultiFlow>>();

  function onSwitchFlow(flow: MultiFlow) {
    setFlow(flow);
  }

  function onNewFlow(newFlow: NewFlow<MultiFlow>) {
    setNewFlow(newFlow);
  }

  return (
    <Card as="form" bg="levels.surface" mx="auto" width={464}>
      <StepSlider<typeof multiflows>
        flows={multiflows}
        currFlow={flow}
        onSwitchFlow={onSwitchFlow}
        newFlow={newFlow}
        changeFlow={onNewFlow}
      />
    </Card>
  );
};

function MainStep1({ next, refCallback, changeFlow }: ViewProps) {
  return (
    <Box p="6" ref={refCallback} data-testid="multi-primary1">
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
            changeFlow({ flow: 'secondary' });
          }}
        >
          Switch Secondary Flow
        </ButtonLink>
      </Box>
    </Box>
  );
}

function MainStep2({ next, prev, refCallback }: ViewProps) {
  return (
    <Box p="6" ref={refCallback} data-testid="multi-primary2">
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

function OtherStep1({ changeFlow, next: onNext, refCallback }: ViewProps) {
  return (
    <Box p="6" ref={refCallback} data-testid="multi-secondary1">
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
            changeFlow({ flow: 'primary', applyNextAnimation: true });
          }}
        >
          Switch Primary Flow
        </ButtonLink>
      </Box>
    </Box>
  );
}

function FinalStep({ prev, refCallback }: ViewProps) {
  return (
    <Box p="6" ref={refCallback} data-testid="multi-final">
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
            prev();
          }}
        >
          Go Back
        </ButtonLink>
      </Box>
    </Box>
  );
}

function Body1({
  next,
  prev,
  refCallback,
  testProp,
}: StepComponentProps & { testProp: string }) {
  return (
    <Box p="6" ref={refCallback} data-testid="single-body1">
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
    <Box p="6" ref={refCallback} data-testid="single-body2">
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
