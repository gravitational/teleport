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

import { Box, Text, Flex, Link as ExternalLink } from 'design';
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
    title: 'Teleport Azure Discovery Documentation',
    href: 'https://goteleport.com/docs/enroll-resources/auto-discovery/servers/azure-discovery/',
  },
];

export function InfoGuideContent() {
  return (
    <Box>
      <InfoTitle>Overview</InfoTitle>
      <InfoParagraph>
        Connect your Azure account to Teleport to automatically discover and
        enroll resources in your cluster.
      </InfoParagraph>

      <InfoTitle>How It Works</InfoTitle>
      <Box pl={2}>
        <ol
          css={`
            padding: inherit;
          `}
        >
          <li>
            <strong>Configure what to discover.</strong> <br />
            Specify resource types, subscriptions, regions, and tag filters to
            control which resources are discovered.
          </li>
          <li>
            <strong>Use the generated Terraform module.</strong> <br />
            The generated Terraform module configuration will create an Azure
            managed identity that grants Teleport read-only access and
            configures Teleport discovery service to scan for your Azure
            resources.
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
            href="https://goteleport.com/docs/enroll-resources/auto-discovery/servers/azure-discovery/#step-35-set-up-an-identity-for-discovered-nodes"
            target="_blank"
          >
            <Flex>
              Azure identity for VMs to be discovered
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
