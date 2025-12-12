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

import styled, { useTheme } from 'styled-components';

import Box from 'design/Box/Box';
import { ButtonPrimary } from 'design/Button/Button';
import Flex from 'design/Flex/Flex';
import Image from 'design/Image/Image';
import Link from 'design/Link/Link';
import Text, { H2 } from 'design/Text/Text';
import { Theme } from 'design/theme/themes/types';

import {
  IntegrationEnrollStatusCode,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent';

import { FlowStepProps } from '../Shared/GuidedFlow';
import { useTracking } from '../Shared/useTracking';
import welcomeDark from './gha-k8s-welcome-dark.svg';
import welcomeLight from './gha-k8s-welcome-light.svg';

export function Welcome(props: FlowStepProps) {
  const { nextStep } = props;

  const theme = useTheme();
  const tracking = useTracking();

  const welcomeImage: IconSpec = {
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

  return (
    <Container>
      <Box>
        <H2 mb={3} mt={3}>
          GitHub Actions + Kubernetes
        </H2>

        <Text as="p">
          Use <code>kubectl</code> and other tools from your GitHub Actions
          workflows.
        </Text>

        <Text as="p" mt={3}>
          You’ll need:
        </Text>
        <ul>
          <li>A GitHub repository</li>
          <li>Access rules for your workflow</li>
        </ul>

        <Text as="p">
          See the{' '}
          <Link
            target="_blank"
            href={IAC_LINK}
            onClick={() => {
              tracking.link(IntegrationEnrollStep.MWIGHAK8SWelcome, IAC_LINK);
            }}
          >
            Infrastructure as Code
          </Link>{' '}
          docs for information about setting up and using IaC with Teleport.
        </Text>

        <Flex gap={2} pt={5}>
          <ButtonPrimary onClick={handleNext}>Start</ButtonPrimary>
        </Flex>
      </Box>

      <Image p={5} width={420} src={welcomeImage[theme.type]} />
    </Container>
  );
}

const Container = styled(Flex)`
  align-items: flex-start;
  flex: 1;
  overflow: auto;
  gap: ${({ theme }) => theme.space[1]}px;
`;

type IconSpec = {
  [K in Theme['type']]: string;
};

const IAC_LINK =
  'https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/';
