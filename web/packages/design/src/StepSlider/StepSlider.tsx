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
import React, { useState, useRef, useEffect, useCallback } from 'react';
import { TransitionGroup, CSSTransition } from 'react-transition-group';
import styled from 'styled-components';

import { Box } from 'design';

export function StepSlider<T>(props: Props<T>) {
  const {
    flows,
    currFlow,
    onSwitchFlow,
    newFlow,
    tDuration = 500,
    // stepProps are the props required by our step components defined in our flows.
    ...stepProps
  } = props;

  const [hasTransitionEnded, setHasTransitionEnded] = useState<boolean>(false);

  // step defines the current step we are in the current flow.
  const [step, setStep] = useState(0);
  // animationDirectionPrefix defines the prefix of the class name that contains
  // the animations to apply when transitioning.
  const [animationDirectionPrefix, setAnimationDirectionPrefix] = useState<
    'next' | 'prev' | ''
  >('');

  const startTransitionInDirection = useCallback(
    (direction: 'next' | 'prev') => {
      setAnimationDirectionPrefix(direction);
      setHasTransitionEnded(false);
    },
    [setAnimationDirectionPrefix, setHasTransitionEnded]
  );

  const [height, setHeight] = useState(0);

  // preMount is used to invisibly render the next view so we
  // can get its height. This height is needed in advance
  // so the height can be animated along with the transform transitions.
  const [preMount, setPreMount] = useState(false);

  // rootRef is used to set the height on initial render.
  // Needed to animate the height on initial transition.
  const rootRef = useRef<HTMLElement>();

  // preMountState is used to hold the latest pre mount data.
  // useState's could not be used b/c they became stale for
  // our func 'setHeightOnPreMount'.
  const preMountState = useRef<{ step: number; flow: keyof T }>({} as any);

  // Sets the initial height.
  useEffect(() => {
    const { height } = rootRef.current.getBoundingClientRect();
    setHeight(height);
  }, []);

  // Triggered as the first step to changing the current flow.
  // It preps data required for pre mounting and sets the
  // next animation direction.
  useEffect(() => {
    // only true on initial render
    if (!newFlow) {
      setHasTransitionEnded(true);
      return;
    }

    preMountState.current.step = 0; // reset step to 0 to start at beginning
    preMountState.current.flow = newFlow.flow;
    rootRef.current.style.height = `${height}px`;

    setPreMount(true);
    if (newFlow.applyNextAnimation) {
      startTransitionInDirection('next');
      return;
    }
    startTransitionInDirection('prev');
  }, [newFlow]);

  // After pre mount, we can calculate the exact height of the next step.
  // After calculating height, we increment the step to trigger the
  // animations.
  const setHeightOnPreMount = (node: HTMLElement) => {
    if (node !== null) {
      setHeight(node.getBoundingClientRect().height);
      setStep(preMountState.current.step);
      setPreMount(false);

      if (preMountState.current.flow) {
        onSwitchFlow(preMountState.current.flow);
      }
    }
  };

  function generateCurrentStep(View: StepComponent, requirePreMount = false) {
    return (
      <View
        key={step}
        refCallback={requirePreMount ? setHeightOnPreMount : null}
        next={() => {
          preMountState.current.step = step + 1;
          setPreMount(true);
          startTransitionInDirection('next');
          rootRef.current.style.height = `${height}px`;
        }}
        prev={() => {
          preMountState.current.step = step - 1;
          setPreMount(true);
          startTransitionInDirection('prev');
          rootRef.current.style.height = `${height}px`;
        }}
        hasTransitionEnded={hasTransitionEnded}
        {...stepProps}
      />
    );
  }

  let $content;
  const Step = flows[currFlow][step];
  if (Step) {
    $content = generateCurrentStep(Step);
  }

  let $preContent;
  if (preMount) {
    let flow = currFlow;
    if (preMountState?.current?.flow) {
      flow = preMountState.current.flow;
    }
    const PreStep = flows[flow][preMountState.current.step];
    if (PreStep) {
      $preContent = generateCurrentStep(PreStep, true /* pass ref callback */);
    }
  }

  const rootStyle = {
    // During the *-enter transition state, children are positioned absolutely
    // to keep views "stacked" on top of each other. Position relative is needed
    // so these children's position themselves relative to parent.
    position: 'relative',
    // Height 'auto' is only ever used on the initial render to let it
    // take up as much space it needs. Afterwards, it sets the starting
    // height.
    height: rootRef?.current?.style.height || 'auto',
    transition: `height ${tDuration}ms ease`,
  };

  return (
    <Box ref={rootRef} style={rootStyle}>
      {preMount && <HiddenBox>{$preContent}</HiddenBox>}
      <Wrap className={animationDirectionPrefix} tDuration={tDuration}>
        <TransitionGroup component={null}>
          <CSSTransition
            // timeout needs to match the css transition duration for smoothness
            timeout={tDuration}
            key={`${step}${String(currFlow)}`}
            classNames={`${animationDirectionPrefix}-slide`}
            onEnter={() => {
              // When steps are translating (sliding), hides overflow content
              rootRef.current.style.overflow = 'hidden';
              // The next height to transition into.
              rootRef.current.style.height = `${height}px`;
            }}
            onExited={() => {
              // Set it back to auto because the parent component might contain elements
              // that may want it to be overflowed e.g. long drop down menu in a small card.
              rootRef.current.style.overflow = 'auto';
              // Set height back to auto to allow the parent component to grow as needed
              // e.g. rendering of an error banner
              rootRef.current.style.height = 'auto';
              setHasTransitionEnded(true);
            }}
          >
            {$content}
          </CSSTransition>
        </TransitionGroup>
      </Wrap>
    </Box>
  );
}

const HiddenBox = styled.div`
  visibility: hidden;
  position: absolute;
`;

const Wrap = styled.div(
  ({ tDuration }) => `
 
 .prev-slide-enter {
   transform: translateX(-100%);
   opacity: 0;
   position: absolute;
   top: 0;
   left: 0;
   right: 0;
   bottom: 0;
 }
 
 .prev-slide-enter-active {
   transform: translateX(0);
   opacity: 1;
   transition: transform ${tDuration}ms ease;
 }
 
 .prev-slide-exit {
   transform: translateX(100%);
   opacity: 1;
   transition: transform ${tDuration}ms ease;
 }
 
 .next-slide-enter {
   transform: translateX(100%);
   opacity: 0;
   position: absolute;
   top: 0;
   left: 0;
   right: 0;
   bottom: 0;
 }
 
 .next-slide-enter-active {
   transform: translateX(0);
   opacity: 1;
   transition: transform ${tDuration}ms ease;
 }
 
 .next-slide-exit {
   transform: translateX(-100%);
   opacity: 1;
   transition: transform ${tDuration}ms ease;
 }
 `
);

type ComponentProps = StepComponentProps & {
  [remainingProps: string]: any;
};

type StepComponent = (props: ComponentProps) => JSX.Element;

type Props<T> = {
  // flows contains the different flows and its accompanying steps.
  flows: Record<keyof T, StepComponent[]>;
  // currFlow refers to the current set of steps.
  // E.g. we have a flow named "passwordless", flow "passwordless"
  // will refer to all the steps related to "passwordless".
  currFlow: keyof T;
  // tDuration is the length of time a transition
  // animation should take to complete.
  tDuration?: number;
  // newFlow is step 1 of 2 of changing the current flow to a new flow.
  // When supplied, it sets the premount data and the next animation class
  // which will kick of the next step `onSwitchFlow` that does the actual
  // switching to the new flow.
  // Optional if there is only one flow.
  newFlow?: NewFlow<keyof T>;
  // onSwitchFlow is the final step that switches the current flow to the new flow.
  // E.g, toggling between "passwordless" or "local" login flow.
  // Optional if there is only one flow.
  onSwitchFlow?(flow: keyof T): void;
  // remainingProps are the rest of the props that needs to be passed
  // down to the flows StepComponent's.
  [remainingProps: string]: any;
};

export type StepComponentProps = {
  // refCallback is a func that is called after component mounts.
  // Required to calculate dimensions of the component for height animations.
  refCallback(node: HTMLElement): void;
  // next goes to the next step in the flow.
  next(): void;
  // prev goes back a step in the flow.
  prev(): void;
  hasTransitionEnded: boolean;
};

// NewFlow defines fields for a new flow.
// The applyNextAnimation flag when true applies the next-slide-* transition,
// otherwise prev-slide-* transitions are applied.
export type NewFlow<T> = {
  flow: T;
  applyNextAnimation?: boolean;
};
