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

import React, { useCallback, useEffect, useState, useRef } from 'react';
import { useLocation } from 'react-router';
import styled from 'styled-components';

import { SwitchTransition, Transition } from 'react-transition-group';

import {
  IntegrationEnrollEvent,
  IntegrationEnrollEventData,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import { Header, HeaderSubtitle } from 'teleport/Discover/Shared';
import { DiscoverUrlLocationState } from 'teleport/Discover/useDiscover';
import { Browser } from 'teleport/Integrations/Enroll/AwsOidc/browser/Browser';
import { IAMHomeScreen } from 'teleport/Integrations/Enroll/AwsOidc/IAM/IAMHomeScreen';
import { Cursor } from 'teleport/Integrations/Enroll/AwsOidc/browser/Cursor';
import { IAMIdentityProvidersScreen } from 'teleport/Integrations/Enroll/AwsOidc/IAM/IAMIdentityProvidersScreen';
import { IAMNewProviderScreen } from 'teleport/Integrations/Enroll/AwsOidc/IAM/IAMNewProviderScreen';
import { FirstStageInstructions } from 'teleport/Integrations/Enroll/AwsOidc/instructions/FirstStageInstructions';
import { SecondStageInstructions } from 'teleport/Integrations/Enroll/AwsOidc/instructions/SecondStageInstructions';

import { ThirdStageInstructions } from 'teleport/Integrations/Enroll/AwsOidc/instructions/ThirdStageInstructions';
import { IAMProvider } from 'teleport/Integrations/Enroll/AwsOidc/IAM/IAMProvider';

import { IAMCreateNewRole } from 'teleport/Integrations/Enroll/AwsOidc/IAM/IAMCreateNewRole';
import { FourthStageInstructions } from 'teleport/Integrations/Enroll/AwsOidc/instructions/FourthStageInstructions';
import { IAMCreateNewRolePermissions } from 'teleport/Integrations/Enroll/AwsOidc/IAM/IAMCreateNewRolePermissions';
import { FifthStageInstructions } from 'teleport/Integrations/Enroll/AwsOidc/instructions/FifthStageInstructions';
import { IAMCreateNewPolicy } from 'teleport/Integrations/Enroll/AwsOidc/IAM/IAMCreateNewPolicy';
import { SixthStageInstructions } from 'teleport/Integrations/Enroll/AwsOidc/instructions/SixthStageInstructions';

import { SeventhStageInstructions } from 'teleport/Integrations/Enroll/AwsOidc/instructions/SeventhStageInstructions';
import { IAMRoles } from 'teleport/Integrations/Enroll/AwsOidc/IAM/IAMRoles';
import useTeleport from 'teleport/useTeleport';

import { Stage, STAGES } from './stages';

const Container = styled.div`
  padding-right: 40px;
  padding-top: 16px;
`;

const InstructionsContainer = styled.div`
  display: flex;
  margin-top: 50px;
`;

const BrowserContainer = styled.div`
  position: relative;
`;

const RestartAnimation = styled.div`
  z-index: 100;
  display: flex;
  align-items: center;
  opacity: ${p => (p.visible ? 1 : 0)};
  transition: 0.2s ease-in-out opacity;
  justify-content: center;
  position: absolute;
  bottom: 10px;
  background: rgba(0, 0, 0, 0.8);
  border-radius: 5px;
  padding: 5px 10px;
  cursor: pointer;
  left: 50%;
  transform: translate(-50%, 0);
  box-shadow: 0 0 15px rgba(0, 0, 0, 0.3);
  color: ${props => props.theme.colors.light};

  &:hover {
    box-shadow: 0 0 15px rgba(0, 0, 0, 0.5);
  }
`;

const defaultStyle = {
  transition: 'opacity 250ms, transform 250ms',
  opacity: 0,
  width: '100%',
};

const horizontalNextTransitionStyles = {
  entering: { opacity: 0, transform: 'translateX(50px)' },
  entered: { opacity: 1, transform: 'translateX(0%)' },
  exited: { opacity: 0, transform: 'translateX(-50px)' },
};

const horizontalPrevTransitionStyles = {
  entering: { opacity: 0, transform: 'translateX(-50px)' },
  entered: { opacity: 1, transform: 'translateX(0%)' },
  exited: { opacity: 0, transform: 'translateX(50px)' },
};

enum InstructionStep {
  First,
  Second,
  Third,
  Fourth,
  Fifth,
  Sixth,
  Seventh,
}

export type AwsOidc = {
  thumbprint: string;
  roleArn: string;
  integrationName: string;
};

export function AwsOidc() {
  const ctx = useTeleport();
  let clusterPublicUri = getClusterPublicUri(
    ctx.storeUser.state.cluster.publicURL
  );

  const transitionRef = useRef<'prev' | 'next'>('next');

  const location = useLocation<DiscoverUrlLocationState>();

  const [stage, setStage] = useState(Stage.Initial);
  const [eventData] = useState<IntegrationEnrollEventData>({
    id: crypto.randomUUID(),
    kind: IntegrationEnrollKind.AwsOidc,
  });
  const [showRestartAnimation, setShowRestartAnimation] = useState(false);
  const [awsOidc, setAwsOidc] = useState<AwsOidc>({
    thumbprint: '',
    roleArn: '',
    integrationName: '',
  });

  const currentStageIndex = STAGES.findIndex(s => s.kind === stage);
  const currentStage = STAGES[currentStageIndex];
  const currentStageConfig = getStageConfig(stage);

  const restartAnimation = useCallback(() => {
    setStage(currentStageConfig.restartStage);
    setShowRestartAnimation(false);
  }, [currentStageConfig]);

  useEffect(() => {
    // If a user came from the discover wizard,
    // discover wizard will send of appropriate events.
    if (location.state?.discover) {
      return;
    }

    emitEvent(IntegrationEnrollEvent.Started);
    // Only send event once on init.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (currentStage.end) {
      setShowRestartAnimation(true);

      return;
    }

    if (showRestartAnimation) {
      setShowRestartAnimation(false);
    }

    if (currentStage.duration && STAGES[currentStageIndex + 1]) {
      const id = window.setTimeout(
        () => setStage(STAGES[currentStageIndex + 1].kind),
        currentStage.duration
      );

      return () => window.clearTimeout(id);
    }
  }, [currentStage, currentStageIndex, showRestartAnimation]);

  function emitEvent(event: IntegrationEnrollEvent) {
    userEventService.captureIntegrationEnrollEvent({
      event,
      eventData,
    });
  }

  function updateState(nextStage: Stage, awsOidc?: AwsOidc) {
    if (nextStage > stage) {
      transitionRef.current = 'next';
    } else {
      transitionRef.current = 'prev';
    }
    if (awsOidc) {
      setAwsOidc(awsOidc);
    }
    setStage(nextStage);
  }

  const transitionStyle =
    transitionRef.current === 'next'
      ? horizontalNextTransitionStyles
      : horizontalPrevTransitionStyles;

  return (
    <Container>
      <Header>Set up your AWS account</Header>

      <HeaderSubtitle>
        Instead of storing long-lived static credentials, Teleport will become a
        trusted OIDC provider with AWS to be able to request short lived
        credentials when performing operations automatically.
      </HeaderSubtitle>

      <InstructionsContainer>
        <SwitchTransition mode="out-in">
          <Transition<undefined>
            key={currentStageConfig.instructionStep}
            timeout={250}
            mountOnEnter
            unmountOnExit
            onExiting={(n: HTMLElement) => {
              n.style.transform = `translateX(${
                transitionRef.current === 'prev' ? '50px' : '-50px'
              })`;
            }}
          >
            {state => (
              <div
                style={{
                  ...defaultStyle,
                  ...transitionStyle[state],
                }}
              >
                {currentStageConfig.instructionStep ===
                  InstructionStep.First && (
                  <FirstStageInstructions
                    onNext={() => updateState(Stage.NewProviderFullScreen)}
                    onPrev={null}
                    clusterPublicUri={clusterPublicUri}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Second && (
                  <SecondStageInstructions
                    awsOidc={awsOidc}
                    onNext={updatedAwsOidc => {
                      updateState(Stage.AddProvider, updatedAwsOidc);
                    }}
                    onPrev={updatedAwsOidc => {
                      updateState(Stage.Initial, updatedAwsOidc);
                    }}
                    clusterPublicUri={clusterPublicUri}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Third && (
                  <ThirdStageInstructions
                    onNext={() => updateState(Stage.CreateNewRole)}
                    onPrev={() => updateState(Stage.NewProviderFullScreen)}
                    clusterPublicUri={clusterPublicUri}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Fourth && (
                  <FourthStageInstructions
                    onNext={() => updateState(Stage.CreatePolicy)}
                    onPrev={() => updateState(Stage.AddProvider)}
                    clusterPublicUri={clusterPublicUri}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Fifth && (
                  <FifthStageInstructions
                    onNext={() => updateState(Stage.AssignPolicyToRole)}
                    onPrev={() => updateState(Stage.CreateNewRole)}
                    clusterPublicUri={clusterPublicUri}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Sixth && (
                  <SixthStageInstructions
                    onNext={() => updateState(Stage.ListRoles)}
                    onPrev={() => updateState(Stage.CreatePolicy)}
                    clusterPublicUri={clusterPublicUri}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Seventh && (
                  <SeventhStageInstructions
                    emitEvent={emitEvent}
                    awsOidc={awsOidc}
                    onPrev={updatedAwsOidc =>
                      updateState(Stage.AssignPolicyToRole, updatedAwsOidc)
                    }
                  />
                )}
              </div>
            )}
          </Transition>
        </SwitchTransition>

        <BrowserContainer>
          <Browser stage={stage}>
            <Cursor
              top={currentStage.cursor.top}
              left={currentStage.cursor.left}
              click={currentStage.cursor.click}
            />
            {getStageComponent(stage, clusterPublicUri)}
          </Browser>

          <RestartAnimation
            visible={showRestartAnimation}
            onClick={() => restartAnimation()}
          >
            Restart animation
          </RestartAnimation>
        </BrowserContainer>
      </InstructionsContainer>
    </Container>
  );
}

function getStageComponent(stage: Stage, uri: string) {
  let clusterPublicUri = uri;
  // Truncate long URI's so it doesn't mess up the animation screens.
  if (clusterPublicUri.length > 30) {
    clusterPublicUri = `${clusterPublicUri.substring(0, 30)}...`;
  }
  const props = { stage, clusterPublicUri };

  if (stage >= Stage.Initial && stage <= Stage.ClickIdentityProviders) {
    return <IAMHomeScreen />;
  }

  if (stage >= Stage.IdentityProviders && stage <= Stage.ClickAddProvider) {
    return <IAMIdentityProvidersScreen {...props} />;
  }

  if (stage >= Stage.NewProvider && stage <= Stage.AddProvider) {
    return <IAMNewProviderScreen {...props} />;
  }

  if (stage >= Stage.ProviderAdded && stage <= Stage.SelectProvider) {
    return <IAMIdentityProvidersScreen {...props} />;
  }

  if (stage >= Stage.ProviderView && stage <= Stage.ClickCreateNewRole) {
    return <IAMProvider {...props} />;
  }

  if (stage >= Stage.CreateNewRole && stage <= Stage.ClickNextPermissions) {
    return <IAMCreateNewRole {...props} />;
  }

  if (
    stage >= Stage.ConfigureRolePermissions &&
    stage <= Stage.ClickCreatePolicy
  ) {
    return <IAMCreateNewRolePermissions {...props} />;
  }

  if (stage >= Stage.CreatePolicy && stage <= Stage.ClickCreatePolicyButton) {
    return <IAMCreateNewPolicy {...props} />;
  }

  if (
    stage >= Stage.AssignPolicyToRole &&
    stage <= Stage.ClickCreateRoleButton
  ) {
    return <IAMCreateNewRolePermissions {...props} />;
  }

  if (stage >= Stage.ListRoles) {
    return <IAMRoles {...props} />;
  }
}

function getStageConfig(stage: Stage) {
  if (stage >= Stage.Initial && stage <= Stage.NewProvider) {
    return {
      instructionStep: InstructionStep.First,
      restartStage: Stage.Initial,
    };
  }

  if (
    stage >= Stage.NewProviderFullScreen &&
    stage <= Stage.ThumbprintSelected
  ) {
    return {
      instructionStep: InstructionStep.Second,
      restartStage: Stage.NewProviderFullScreen,
    };
  }

  if (stage >= Stage.AddProvider && stage <= Stage.ClickCreateNewRole) {
    return {
      instructionStep: InstructionStep.Third,
      restartStage: Stage.AddProvider,
    };
  }

  if (stage >= Stage.CreateNewRole && stage <= Stage.ClickCreatePolicy) {
    return {
      instructionStep: InstructionStep.Fourth,
      restartStage: Stage.CreateNewRole,
    };
  }

  if (stage >= Stage.CreatePolicy && stage <= Stage.ClickCreatePolicyButton) {
    return {
      instructionStep: InstructionStep.Fifth,
      restartStage: Stage.CreatePolicy,
    };
  }

  if (
    stage >= Stage.AssignPolicyToRole &&
    stage <= Stage.ClickCreateRoleButton
  ) {
    return {
      instructionStep: InstructionStep.Sixth,
      restartStage: Stage.AssignPolicyToRole,
    };
  }

  if (stage >= Stage.ListRoles) {
    return {
      instructionStep: InstructionStep.Seventh,
      restartStage: Stage.ListRoles,
    };
  }
}

function getClusterPublicUri(uri: string) {
  const uriParts = uri.split(':');
  const port = uriParts.length > 1 ? uriParts[1] : '';

  // Strip 443 ports from uri.
  if (port === '443') {
    return uriParts[0];
  }

  return uri;
}
