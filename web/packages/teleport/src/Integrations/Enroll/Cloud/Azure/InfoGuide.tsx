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

import { Box } from 'design';
import {
  InfoParagraph,
  InfoTitle,
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
            <strong>
              Deploy Azure managed identity with discovery permissions.
            </strong>
            <br />
            Using Terraform, create a managed identity and custom role that
            grants Teleport read-only access to your Azure resources.
          </li>
          <li>
            <strong>Configure what to discover.</strong> <br />
            Specify regions, resource types (VMs), and tag filters to control
            which resources are discovered.
          </li>
          <li>
            <strong>Automatic discovery begins.</strong> <br />
            Teleport scans your Azure environment every 30 minutes to find
            resources matching your configuration.
          </li>
          <li>
            <strong>Resources appear in your cluster.</strong>
            <br /> Discovered resources are automatically enrolled in Teleport
            and ready for secure access.
          </li>
        </ol>
      </Box>
      <ReferenceLinks links={referenceLinks} />
    </Box>
  );
}
