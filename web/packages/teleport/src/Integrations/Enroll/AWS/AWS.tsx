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

import React, { useCallback, useEffect, useState } from 'react';
import styled from 'styled-components';

import { SwitchTransition, Transition } from 'react-transition-group';

import { Header, HeaderSubtitle } from 'teleport/Discover/Shared';
import { Browser } from 'teleport/Integrations/Enroll/AWS/browser/Browser';
import { IAMHomeScreen } from 'teleport/Integrations/Enroll/AWS/IAM/IAMHomeScreen';
import { Cursor } from 'teleport/Integrations/Enroll/AWS/browser/Cursor';
import { IAMIdentityProvidersScreen } from 'teleport/Integrations/Enroll/AWS/IAM/IAMIdentityProvidersScreen';
import { IAMNewProviderScreen } from 'teleport/Integrations/Enroll/AWS/IAM/IAMNewProviderScreen';
import { FirstStageInstructions } from 'teleport/Integrations/Enroll/AWS/instructions/FirstStageInstructions';
import { SecondStageInstructions } from 'teleport/Integrations/Enroll/AWS/instructions/SecondStageInstructions';

import { ThirdStageInstructions } from 'teleport/Integrations/Enroll/AWS/instructions/ThirdStageInstructions';
import { IAMProvider } from 'teleport/Integrations/Enroll/AWS/IAM/IAMProvider';

import { IAMCreateNewRole } from 'teleport/Integrations/Enroll/AWS/IAM/IAMCreateNewRole';
import { FourthStageInstructions } from 'teleport/Integrations/Enroll/AWS/instructions/FourthStageInstructions';
import { IAMCreateNewRolePermissions } from 'teleport/Integrations/Enroll/AWS/IAM/IAMCreateNewRolePermissions';
import { FifthStageInstructions } from 'teleport/Integrations/Enroll/AWS/instructions/FifthStageInstructions';
import { IAMCreateNewPolicy } from 'teleport/Integrations/Enroll/AWS/IAM/IAMCreateNewPolicy';
import { SixthStageInstructions } from 'teleport/Integrations/Enroll/AWS/instructions/SixthStageInstructions';

import { SeventhStageInstructions } from 'teleport/Integrations/Enroll/AWS/instructions/SeventhStageInstructions';
import { IAMRoles } from 'teleport/Integrations/Enroll/AWS/IAM/IAMRoles';

import { Stage, STAGES } from './stages';

const Container = styled.div`
  padding-left: 40px;
  padding-right: 40px;
  padding-top: 30px;
`;

const InstructionsContainer = styled.div`
  display: flex;
  margin-top: 50px;
`;

const BrowserContainer = styled.div`
  position: relative;
`;

const RestartAnimation = styled.div`
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

  &:hover {
    box-shadow: 0 0 15px rgba(0, 0, 0, 0.5);
  }
`;

const defaultStyle = {
  transition: 'opacity 250ms, transform 250ms',
  opacity: 0,
  width: '100%',
};

const horizontalTransitionStyles = {
  entering: { opacity: 0, transform: 'translateX(50px)' },
  entered: { opacity: 1, transform: 'translateX(0%)' },
  exiting: { opacity: 0, transform: 'translateX(-50px)' },
  exited: { opacity: 0, transform: 'translateX(-50px)' },
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

export function AWS() {
  const [stage, setStage] = useState(Stage.Initial);
  const [showRestartAnimation, setShowRestartAnimation] = useState(false);

  const currentStageIndex = STAGES.findIndex(s => s.kind === stage);
  const currentStage = STAGES[currentStageIndex];
  const currentStageConfig = getStageConfig(stage);

  const restartAnimation = useCallback(() => {
    setStage(currentStageConfig.restartStage);
    setShowRestartAnimation(false);
  }, [currentStageConfig]);

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

  return (
    <Container>
      <Header>Set up your first AWS account</Header>

      <HeaderSubtitle>
        Instead of storing long-lived static credentials, Teleport will become a
        trusted OIDC provider with AWS to be able to request short lived
        credentials when performing operations automatically.
      </HeaderSubtitle>

      <InstructionsContainer>
        <SwitchTransition mode="out-in">
          <Transition
            key={currentStageConfig.instructionStep}
            timeout={250}
            mountOnEnter
            unmountOnExit
          >
            {state => (
              <div
                style={{
                  ...defaultStyle,
                  ...horizontalTransitionStyles[state],
                }}
              >
                {currentStageConfig.instructionStep ===
                  InstructionStep.First && (
                  <FirstStageInstructions
                    stage={stage}
                    onNext={() => {
                      setStage(Stage.NewProviderFullScreen);
                    }}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Second && (
                  <SecondStageInstructions
                    onNext={() => {
                      setStage(Stage.AddProvider);
                    }}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Third && (
                  <ThirdStageInstructions
                    onNext={() => {
                      setStage(Stage.CreateNewRole);
                    }}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Fourth && (
                  <FourthStageInstructions
                    onNext={() => {
                      setStage(Stage.CreatePolicy);
                    }}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Fifth && (
                  <FifthStageInstructions
                    onNext={() => {
                      setStage(Stage.AssignPolicyToRole);
                    }}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Sixth && (
                  <SixthStageInstructions
                    onNext={() => {
                      setStage(Stage.ListRoles);
                    }}
                  />
                )}
                {currentStageConfig.instructionStep ===
                  InstructionStep.Seventh && (
                  <SeventhStageInstructions
                    onNext={() => {
                      setStage(Stage.ListRoles);
                    }}
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
            {getStageComponent(stage)}
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

function getStageComponent(stage: Stage) {
  if (stage >= Stage.Initial && stage <= Stage.ClickIdentityProviders) {
    return <IAMHomeScreen />;
  }

  if (stage >= Stage.IdentityProviders && stage <= Stage.ClickAddProvider) {
    return <IAMIdentityProvidersScreen stage={stage} />;
  }

  if (stage >= Stage.NewProvider && stage <= Stage.AddProvider) {
    return <IAMNewProviderScreen stage={stage} />;
  }

  if (stage >= Stage.ProviderAdded && stage <= Stage.SelectProvider) {
    return <IAMIdentityProvidersScreen stage={stage} />;
  }

  if (stage >= Stage.ProviderView && stage <= Stage.ClickCreateNewRole) {
    return <IAMProvider stage={stage} />;
  }

  if (stage >= Stage.CreateNewRole && stage <= Stage.ClickNextPermissions) {
    return <IAMCreateNewRole stage={stage} />;
  }

  if (
    stage >= Stage.ConfigureRolePermissions &&
    stage <= Stage.ClickCreatePolicy
  ) {
    return <IAMCreateNewRolePermissions stage={stage} />;
  }

  if (stage >= Stage.CreatePolicy && stage <= Stage.ClickCreatePolicyButton) {
    return <IAMCreateNewPolicy stage={stage} />;
  }

  if (
    stage >= Stage.AssignPolicyToRole &&
    stage <= Stage.ClickCreateRoleButton
  ) {
    return <IAMCreateNewRolePermissions stage={stage} />;
  }

  if (stage >= Stage.ListRoles) {
    return <IAMRoles stage={stage} />;
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
