/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Box, Flex, Link as ExternalLink, Text } from 'design';
import { NewTab } from 'design/Icon';
import {
  InfoParagraph,
  InfoTitle,
  InfoUl,
  ReferenceLinks,
  type ReferenceLink,
} from 'shared/components/SlidingSidePanel/InfoGuide';

const referenceLinks: ReferenceLink[] = [
  {
    title: 'Teleport AWS Discovery Documentation',
    href: 'https://goteleport.com/docs/enroll-resources/auto-discovery/servers/ec2-discovery/',
  },
  {
    title: 'AWS IAM Roles',
    href: 'https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html',
  },
  {
    title: 'AWS Organizations',
    href: 'https://docs.aws.amazon.com/organizations/',
  },
];

export function InfoGuideContent() {
  return (
    <Box>
      <InfoTitle>How It Works</InfoTitle>
      <Box pl={2}>
        <ol
          css={`
            padding: inherit;
          `}
        >
          <li>
            <strong>Configure what to discover.</strong>
            <br />
            <Text as="span" color="text.slightlyMuted">
              Specify resource types, regions, and tag filters to control which
              resources are discovered.
            </Text>
          </li>
          <li>
            <strong>Use the generated Terraform module.</strong>
            <br />
            <Text as="span" color="text.slightlyMuted">
              The Terraform module will set up an OIDC connection in AWS and
              configure Teleport discovery to scan for your resources.
            </Text>
          </li>
          <li>
            <strong>
              Your cloud resources automatically appear in your Teleport
              cluster.
            </strong>
            <br />
            <Text as="span" color="text.slightlyMuted">
              Teleport scans every 30 minutes to find matching resources.
              Resources are enrolled in Teleport and ready for secure access.
            </Text>
          </li>
        </ol>
      </Box>

      <InfoTitle>Prerequisites</InfoTitle>
      <InfoParagraph>Before you begin, ensure you have:</InfoParagraph>
      <InfoUl>
        <InfoLinkLi>
          <ExternalLink
            href="https://goteleport.com/docs/enroll-resources/auto-discovery/servers/ec2-discovery/ec2-discovery-terraform#step-15-configure-aws-terraform-provider"
            target="_blank"
          >
            <Flex>
              AWS IAM permissions for discovery
              <NewTab size="small" ml={1} />
            </Flex>
          </ExternalLink>
        </InfoLinkLi>
        <InfoLinkLi>
          <ExternalLink
            href="https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent.html"
            target="_blank"
          >
            <Flex>
              SSM agent running on EC2 instances
              <NewTab size="small" ml={1} />
            </Flex>
          </ExternalLink>
        </InfoLinkLi>
      </InfoUl>

      <ReferenceLinks links={referenceLinks} />
    </Box>
  );
}

const InfoLinkLi = styled.li`
  color: ${({ theme }) => theme.colors.interactive.solid.accent.default};
`;
