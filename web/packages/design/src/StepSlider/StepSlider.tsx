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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { CSSTransition, TransitionGroup } from 'react-transition-group';
import styled from 'styled-components';

import Box from 'design/Box';

/**
 * StepSlider
 *
 * There are two transition style this StepSlider can
 * take on depending on how it is used.
 *
 * 1) In place slider (look at FormLogin.tsx) where components
 *    slides in the same container as parent.
 * 2) Wheel like slider (look at Welcome.tsx) where the whole
 *    component slides out of screen.
 *
 * Noted Caveats with Wheel like slider:
 *
 * Parent of slider must have a margin-top AND a margin-bottom
 * (no auto, and number > 0) for the dynamic height transition
 * to work without glitching. Transition may work fine without
 * both top/bottom margins, but top/bottom box shadows will be
 * cutt off and is noticeable with lighter themes.
 *
 * Parent of slider having a sibling where vertical margins are
 * collapsed will also make it glitch by "uncollapsing" the
 * margins and ending up with more space than the original.
 *
 */
export function StepSlider<Flows>(props: Props<Flows>) {
  const {
    flows,
    currFlow,
    onSwitchFlow,
    newFlow,
    defaultStepIndex = 0,
    tDuration = 500,
    wrapping = false,
    // extraProps are the props required by our step components defined in our flows.
    ...extraProps
  } = props;

  const [hasTransitionEnded, setHasTransitionEnded] = useState<boolean>(false);

  // step defines the current step we are in the current flow.
  const [step, setStep] = useState(defaultStepIndex);
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
  const rootRef = useRef<HTMLDivElement>();

  // preMountState is used to hold the latest pre mount data.
  // useState's could not be used b/c they became stale for
  // our func 'setHeightOnPreMount'.
  const preMountState = useRef<{ step: number; flow: keyof Flows }>({} as any);

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
      setHeight(node.getBoundingClientRect().height + getMarginY(node));
      setStep(preMountState.current.step);
      setPreMount(false);

      if (preMountState.current.flow) {
        onSwitchFlow(preMountState.current.flow);
      }
    }
  };

  const setHeightOnInitialMount = (node: HTMLElement) => {
    if (node !== null) {
      setHeight(node.getBoundingClientRect().height + getMarginY(node));
    }
  };

  function generateCurrentStep(
    View: React.ComponentType<StepComponentProps & Record<string, any>>,
    requirePreMount = false
  ) {
    // refCallbackFn is called with the DOM element ("View") that
    // has been mounted. This way we can get the true height of
    // the "View" container with the margins.
    let refCallbackFn: (node: HTMLElement) => void;
    if (requirePreMount) {
      refCallbackFn = setHeightOnPreMount;
    } else if (!rootRef?.current) {
      refCallbackFn = setHeightOnInitialMount;
    }
    return (
      <View
        key={step}
        refCallback={refCallbackFn}
        next={() => {
          const flow = preMountState.current.flow ?? currFlow;
          if (wrapping && step === flows[flow].length - 1) {
            preMountState.current.step = 0;
          } else {
            preMountState.current.step = step + 1;
          }
          setPreMount(true);
          startTransitionInDirection('next');
          rootRef.current.style.height = `${height}px`;
        }}
        prev={() => {
          if (wrapping && step === 0) {
            const flow = preMountState.current.flow ?? currFlow;
            preMountState.current.step = flows[flow].length - 1;
          } else {
            preMountState.current.step = step - 1;
          }
          setPreMount(true);
          startTransitionInDirection('prev');
          rootRef.current.style.height = `${height}px`;
        }}
        hasTransitionEnded={hasTransitionEnded}
        stepIndex={step}
        flowLength={flows[currFlow].length}
        {...extraProps}
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

  // Sets the height of the outer container (root container).
  // Initial render will always be 'auto' since rootRef current
  // will be undefined.
  let heightWithMargins = 'auto';
  if (rootRef?.current) {
    heightWithMargins = rootRef.current.style.height;
  }

  const rootStyle: React.CSSProperties = {
    // During the *-enter transition state, children are positioned absolutely
    // to keep views "stacked" on top of each other. Position relative is needed
    // so these children's position themselves relative to parent.
    position: 'relative',
    height: heightWithMargins,
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
            <Box>{$content}</Box>
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
  ({ tDuration }: { tDuration: number }) => `
 
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

interface AnyFlows<ExtraProps> {
  [key: string]: React.ComponentType<StepComponentProps & ExtraProps>[];
}

type Props<Flows> =
  Flows extends AnyFlows<infer ExtraProps>
    ? {
        /** flows contains the different flows and its accompanying steps. */
        flows: Flows;
        /**
         * currFlow refers to the current set of steps.
         * E.g. we have a flow named "passwordless", flow "passwordless"
         * will refer to all the steps related to "passwordless".
         */
        currFlow: keyof Flows;
        /**
         * tDuration is the length of time a transition
         * animation should take to complete.
         */
        tDuration?: number;
        /**
         * newFlow is step 1 of 2 of changing the current flow to a new flow.
         * When supplied, it sets the premount data and the next animation class
         * which will kick of the next step `onSwitchFlow` that does the actual
         * switching to the new flow.
         * Optional if there is only one flow.
         */
        newFlow?: NewFlow<keyof Flows>;
        /**
         * onSwitchFlow is the final step that switches the current flow to the new flow.
         * E.g, toggling between "passwordless" or "local" login flow.
         * Optional if there is only one flow.
         */
        onSwitchFlow?(flow: keyof Flows): void;
        /**
         * defaultStepIndex is the step that will be shown on the first render, similar to
         * defaultValue passed to the input tag.
         *
         * Since this value is used only for the initial render, it won't be persisted when
         * switching flows – this will result in the current step index being reset to 0.
         */
        defaultStepIndex?: number;
        /**
         * If set to `true`, allows going forwards the last slide to the first
         * one and backwards from the first one to the last one.
         */
        wrapping?: boolean;
      } & ExtraProps // Extra props that are passed to each step component. Each step of each flow needs to accept the same set of extra props.
    : never;

export type StepComponentProps = {
  /**
   * refCallback is a func that is called after component mounts.
   * Required to calculate dimensions of the component for height animations.
   */
  refCallback(node: HTMLElement): void;
  /**
   * next goes to the next step in the flow.
   */
  next(): void;
  /**
   * prev goes back a step in the flow.
   */
  prev(): void;
  hasTransitionEnded: boolean;
  stepIndex: number;
  flowLength: number;
};

/**
 * NewFlow defines fields for a new flow.
 * The applyNextAnimation flag when true applies the next-slide-* transition,
 * otherwise prev-slide-* transitions are applied.
 */
export type NewFlow<T> = {
  flow: T;
  applyNextAnimation?: boolean;
};

function getMarginY(node: HTMLElement) {
  const style = getComputedStyle(node);
  const marginTopNum = parseInt(style.marginTop);
  const marginBotNum = parseInt(style.marginBottom);

  if (isNaN(marginTopNum) || isNaN(marginBotNum)) {
    return 0;
  }

  return marginTopNum + marginBotNum;
}
