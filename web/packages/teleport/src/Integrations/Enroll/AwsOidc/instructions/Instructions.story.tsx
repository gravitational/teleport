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
import { MemoryRouter } from 'react-router';

import {
  Integration,
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { FirstStageInstructions } from './FirstStageInstructions';
import { SecondStageInstructions } from './SecondStageInstructions';
import { ThirdStageInstructions } from './ThirdStageInstructions';
import { FourthStageInstructions } from './FourthStageInstructions';
import { FifthStageInstructions } from './FifthStageInstructions';
import { SixthStageInstructions } from './SixthStageInstructions';
import {
  SeventhStageInstructions,
  SuccessfullyAddedIntegrationDialog,
} from './SeventhStageInstructions';

import type { DiscoverUrlLocationState } from 'teleport/Discover/useDiscover';
import type { CommonInstructionsProps } from './common';

export default {
  title: 'Teleport/Integrations/Enroll/AwsOidc/Instructions',
};

export const Step1 = () => <FirstStageInstructions {...props} />;
export const Step2 = () => <SecondStageInstructions {...props} />;
export const Step3 = () => <ThirdStageInstructions {...props} />;
export const Step4 = () => <FourthStageInstructions {...props} />;
export const Step5 = () => <FifthStageInstructions {...props} />;
export const Step6 = () => <SixthStageInstructions {...props} />;
export const Step7 = () => (
  <MemoryRouter>
    <SeventhStageInstructions {...props} emitEvent={() => null} />
  </MemoryRouter>
);

export const ConfirmDialog = () => (
  <MemoryRouter>
    <SuccessfullyAddedIntegrationDialog
      integration={mockIntegration}
      emitEvent={() => null}
    />
  </MemoryRouter>
);

export const ConfirmDialogFromDiscover = () => (
  <MemoryRouter
    initialEntries={[{ state: { discover: {} } as DiscoverUrlLocationState }]}
  >
    <SuccessfullyAddedIntegrationDialog
      integration={mockIntegration}
      emitEvent={() => null}
    />
  </MemoryRouter>
);

const props: CommonInstructionsProps = {
  onNext: () => null,
  onPrev: () => null,
  awsOidc: {
    thumbprint: 'thumbprint',
    roleArn: 'arn',
    integrationName: 'name',
  },
  clusterPublicUri: 'gravitationalwashington.cloud.gravitional.io:4444',
};

const mockIntegration: Integration = {
  kind: IntegrationKind.AwsOidc,
  name: 'aws-oidc-integration',
  resourceType: 'integration',
  spec: {
    roleArn: 'arn-123',
  },
  statusCode: IntegrationStatusCode.Running,
};
