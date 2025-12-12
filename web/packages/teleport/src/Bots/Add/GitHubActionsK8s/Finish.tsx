/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useState } from 'react';
import { useHistory } from 'react-router';
import { styled } from 'styled-components';

import Box from 'design/Box/Box';
import {
  ButtonPrimary,
  ButtonSecondary,
  ButtonWarning,
} from 'design/Button/Button';
import { Dialog } from 'design/Dialog/Dialog';
import DialogContent from 'design/Dialog/DialogContent';
import DialogHeader from 'design/Dialog/DialogHeader';
import DialogTitle from 'design/Dialog/DialogTitle';
import Flex from 'design/Flex/Flex';
import Link from 'design/Link/Link';
import Text, { H2, P2 } from 'design/Text/Text';

import cfg from 'teleport/config';
import {
  IntegrationEnrollStatusCode,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent';

import { FlowStepProps } from '../Shared/GuidedFlow';
import { useTracking } from '../Shared/useTracking';
import { CodePanel } from './CodePanel';

export function Finish(props: FlowStepProps) {
  const { prevStep } = props;

  const [showDoneCheck, setShowDoneCheck] = useState(false);

  const history = useHistory();
  const tracking = useTracking();

  const handleDone = () => {
    tracking.step(
      IntegrationEnrollStep.MWIGHAK8SSetupWorkflow,
      IntegrationEnrollStatusCode.Success
    );

    history.replace(cfg.getBotsRoute());
  };

  return (
    <Container>
      <Box flex={1}>
        <H2 mb={3} mt={3}>
          Setup Workflow
        </H2>

        <Text as="p" mt={3}>
          <strong>To complete the setup</strong>;
        </Text>
        <ul>
          <li>Use the Infrastructure as Code templates to create resources</li>
          <li>Copy the workflow template and add it to your repository</li>
        </ul>

        <Text as="p">
          See the{' '}
          <Link
            target="_blank"
            href={IAC_LINK}
            onClick={() => {
              tracking.link(
                IntegrationEnrollStep.MWIGHAK8SSetupWorkflow,
                IAC_LINK
              );
            }}
          >
            Infrastructure as Code
          </Link>{' '}
          docs for information about setting up and using IaC with Teleport.
        </Text>

        <Text as="p" mt={3}>
          See the{' '}
          <Link
            target="_blank"
            href={TBOT_GHA_LINK}
            onClick={() => {
              tracking.link(
                IntegrationEnrollStep.MWIGHAK8SSetupWorkflow,
                TBOT_GHA_LINK
              );
            }}
          >
            Deploying tbot on GitHub Actions
          </Link>{' '}
          docs for information about running tbot in a GitHub Actions workflow.
        </Text>

        <Flex gap={2} pt={5}>
          <ButtonPrimary onClick={() => setShowDoneCheck(true)}>
            Done
          </ButtonPrimary>
          <ButtonSecondary onClick={prevStep}>Back</ButtonSecondary>
        </Flex>
      </Box>

      <CodeContainer>
        <CodePanel />
      </CodeContainer>

      <Dialog open={showDoneCheck} onClose={() => setShowDoneCheck(false)}>
        <DialogHeader mb={4}>
          <DialogTitle>
            Are you sure you would like to complete the guide?
          </DialogTitle>
        </DialogHeader>
        <DialogContent mb={3} maxWidth={480}>
          <P2>
            Once the guide is completed, you will not longer have access to the
            Infrastructure as Code and GitHub workflow templates.
          </P2>
        </DialogContent>
        <Flex gap={3}>
          <ButtonWarning block size="large" onClick={handleDone}>
            Confirm
          </ButtonWarning>
          <ButtonSecondary
            block
            size="large"
            autoFocus
            onClick={() => setShowDoneCheck(false)}
          >
            Cancel
          </ButtonSecondary>
        </Flex>
      </Dialog>
    </Container>
  );
}

const Container = styled(Flex)`
  flex: 1;
  overflow: auto;
  gap: ${({ theme }) => theme.space[3]}px;
`;

const CodeContainer = styled(Flex)`
  flex: 1;
  flex-direction: column;
  overflow: auto;
`;

const IAC_LINK =
  'https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/';
const TBOT_GHA_LINK =
  '"https://goteleport.com/docs/machine-workload-identity/deployment/github-actions/"';
