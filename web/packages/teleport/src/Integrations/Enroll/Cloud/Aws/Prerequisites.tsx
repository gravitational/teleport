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

import styled from 'styled-components';
import { useToggle } from 'usehooks-ts';

import { Box, Link as ExternalLink, Flex, Text } from 'design';
import { ArrowSquareOut, ChevronRight } from 'design/Icon';

import { Divider } from './EnrollAws';

export function Prerequisites() {
  const [showPrerequisites, togglePrerequisites] = useToggle(true);

  return (
    <>
      <Flex mb={1} onClick={togglePrerequisites} css={{ cursor: 'pointer' }}>
        <SectionChevron expanded={showPrerequisites} mr={2} />
        <Text typography="h2" fontSize={4} fontWeight="medium">
          Prerequisites
        </Text>
      </Flex>
      {showPrerequisites && (
        <Box pl={5}>
          <Text mb={2}>
            Before you begin, configure the required Terraform providers:
          </Text>
          <Text typography="h3" mb={1}>
            Required Configuration
          </Text>
          <ul
            css={`
              margin: 0;
              padding-left: ${p => p.theme.space[3]}px;
            `}
          >
            <li>
              <Text fontWeight="medium">Teleport Terraform Provider:</Text>
              <Text>Authenticate to your Teleport cluster</Text>
              <ExternalLink
                href="https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/terraform-provider/"
                target="_blank"
              >
                Teleport Provider Configuration
                <ArrowSquareOut size="small" />
              </ExternalLink>
            </li>
            <li>
              <Text fontWeight="medium">AWS Terraform Provider:</Text>
              <Text>Configure AWS credentials for IAM management</Text>
              <ExternalLink
                href="https://registry.terraform.io/providers/hashicorp/aws/latest/docs"
                target="_blank"
              >
                AWS Provider Configuration <ArrowSquareOut size="small" />
              </ExternalLink>
            </li>
          </ul>
          <Divider />
          <Text typography="h3" mb={2}>
            AWS Permissions
          </Text>
          <Text typography="h3" fontSize="small">
            For Single AWS Account Discovery:
          </Text>
          <ul
            css={`
              margin: 0;
              padding-left: ${p => p.theme.space[3]}px;
            `}
          >
            <li>
              <Text>
                AWS IAM permissions required for AWS Terraform provider
              </Text>
              <ExternalLink
                href="https://goteleport.com/docs/enroll-resources/auto-discovery/servers/ec2-discovery/ec2-discovery-terraform#step-15-configure-aws-terraform-provider"
                target="_blank"
              >
                Terraform EC2 Auto-Discovery Configuration{' '}
                <ArrowSquareOut size="small" />
              </ExternalLink>
            </li>
          </ul>
          <Text typography="h3" fontSize="small">
            For EC2 resources:
          </Text>
          <ul
            css={`
              margin: 0;
              padding-left: ${p => p.theme.space[3]}px;
            `}
          >
            <li>
              <Text>
                AmazonSSMManagedInstanceCore IAM policy attached to EC2
                instance's role.
              </Text>
              <ExternalLink
                href="https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonSSMManagedInstanceCore.html"
                target="_blank"
              >
                AmazonSSMManagedInstanceCore Policy{' '}
                <ArrowSquareOut size="small" />
              </ExternalLink>
            </li>
            <li>
              <Text>SSM agent running on EC2 instances to be discovered.</Text>
              <ExternalLink
                href="https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent.html"
                target="_blank"
              >
                Working with SSM Agent
                <ArrowSquareOut size="small" />
              </ExternalLink>
            </li>
          </ul>
        </Box>
      )}
    </>
  );
}

const SectionChevron = styled(ChevronRight)<{ expanded: boolean }>`
  transition: transform 0.2s ease-in-out;
  transform: ${props => (props.expanded ? 'rotate(90deg)' : 'none')};
  stroke: ${props => props.theme.colors.text.slightlyMuted};
`;
