/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';

import * as Icons from 'design/Icon';

import { ButtonPrimary } from 'design/Button';

import {
  StepContent,
  StepInstructions,
  StepTitle,
  StepTitleIcon,
} from 'teleport/Discover/Desktop/ConnectTeleport/Step';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import { generateCommand } from 'teleport/Discover/Shared/generateCommand';

import cfg from 'teleport/config';
import { useJoinTokenSuspender } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { ResourceKind } from 'teleport/Discover/Shared';

import loading from './run-configure-script-loading.svg';

interface RunConfigureScriptProps {
  onNext: () => void;
}

export function RunConfigureScript(
  props: React.PropsWithChildren<RunConfigureScriptProps>
) {
  const { joinToken } = useJoinTokenSuspender(ResourceKind.Desktop);

  const command = generateCommand(cfg.getConfigureADUrl(joinToken.id));

  return (
    <StepContent>
      <StepTitle>
        <StepTitleIcon>
          <Icons.Terminal size="extraLarge" />
        </StepTitleIcon>
        1. Run the configure Active Directory script
      </StepTitle>

      <StepInstructions>
        <TextSelectCopy text={command} mt={2} mb={5} bash allowMultiline />

        <ButtonPrimary onClick={() => props.onNext()}>Next</ButtonPrimary>
      </StepInstructions>
    </StepContent>
  );
}

export function RunConfigureScriptLoading() {
  return (
    <StepContent>
      <StepTitle>
        <StepTitleIcon>
          <Icons.Terminal size="extraLarge" />
        </StepTitleIcon>
        1. Run the configure Active Directory script
      </StepTitle>

      <StepInstructions>
        <LoadingBox />
      </StepInstructions>
    </StepContent>
  );
}

const LoadingBox = styled.div`
  width: 340px;
  height: 84px;
  background: url(${loading}) no-repeat;
`;
