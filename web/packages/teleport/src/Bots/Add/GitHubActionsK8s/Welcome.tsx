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

import { useHistory, useLocation } from 'react-router';
import styled, { useTheme } from 'styled-components';

import { Info } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button/Button';
import Flex from 'design/Flex/Flex';
import Image from 'design/Image/Image';
import Link from 'design/Link/Link';
import { H2, P2 } from 'design/Text/Text';
import { Theme } from 'design/theme/themes/types';

import cfg from 'teleport/config';
import {
  IntegrationEnrollStatusCode,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent';

import { FlowStepProps } from '../Shared/GuidedFlow';
import { useTracking } from '../Shared/useTracking';
import { CodePanel } from './CodePanel';
import welcomeDark from './gha-k8s-welcome-dark.svg';
import welcomeLight from './gha-k8s-welcome-light.svg';

export function Welcome(props: FlowStepProps) {
  const { nextStep } = props;

  const theme = useTheme();
  const tracking = useTracking();
  const location = useLocation();
  const history = useHistory();

  const welcomeImage: ImageSpec = {
    light: welcomeLight,
    dark: welcomeDark,
  };

  const handleNext = () => {
    tracking.step(
      IntegrationEnrollStep.MWIGHAK8SWelcome,
      IntegrationEnrollStatusCode.Success
    );

    nextStep?.();
  };

  const handlePrevious = () => {
    // If location.key is unset, or 'default', this is the first history entry in-app in the session.
    if (!location.key || location.key === 'default') {
      history.push(cfg.getIntegrationsEnrollRoute());
    } else {
      history.goBack();
    }
  };

  return (
    <Container>
      <FormContainer>
        <Box>
          <H2 mb={3} mt={3}>
            GitHub Actions + Kubernetes
          </H2>

          <P2>
            Use <code>kubectl</code> and other tools from your GitHub Actions
            workflows.
          </P2>

          <Image py={4} width={420} src={welcomeImage[theme.type]} />

          <P2>You’ll need:</P2>
          <ul>
            <li>A GitHub repository</li>
            <li>An enrolled Kubernetes cluster</li>
          </ul>

          <Info
            alignItems="flex-start"
            mt={4}
            details={
              <>
                This guide uses Infrastructure as Code (IaC) to create the
                resources required to complete the setup. See the{' '}
                <Link
                  target="_blank"
                  href={IAC_LINK}
                  onClick={() => {
                    tracking.link(
                      IntegrationEnrollStep.MWIGHAK8SWelcome,
                      IAC_LINK
                    );
                  }}
                >
                  Infrastructure as Code
                </Link>{' '}
                docs for information about setting up and using IaC with
                Teleport.
              </>
            }
          >
            Infrastructure as Code required
          </Info>

          <Flex gap={2} pt={1}>
            <ButtonPrimary onClick={handleNext}>Start</ButtonPrimary>
            <ButtonSecondary onClick={handlePrevious}>Back</ButtonSecondary>
          </Flex>
        </Box>
      </FormContainer>

      <CodeContainer>
        <CodePanel
          trackingStep={IntegrationEnrollStep.MWIGHAK8SWelcome}
          inProgress
        />
      </CodeContainer>
    </Container>
  );
}

const Container = styled(Flex)`
  flex: 1;
  overflow: auto;
  gap: ${({ theme }) => theme.space[1]}px;
`;

const FormContainer = styled(Flex)`
  flex: 4;
  flex-direction: column;
  overflow: auto;
  padding-right: ${({ theme }) => theme.space[5]}px;
`;

const CodeContainer = styled(Flex)`
  flex: 6;
  flex-direction: column;
  overflow: auto;
`;

type ImageSpec = {
  [K in Theme['type']]: string;
};

const IAC_LINK =
  'https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/';
