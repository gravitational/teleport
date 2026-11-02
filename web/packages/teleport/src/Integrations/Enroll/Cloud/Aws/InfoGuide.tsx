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

import { Box, Text, Stack } from 'design';
import {
  InfoTitle,
  Links,
  ReferenceLinks,
  type ReferenceLink,
} from 'shared/components/SlidingSidePanel/InfoGuide';

const dbRefLinks: ReferenceLink[] = [
  {
    title: 'Database Labels Reference',
    href: 'https://goteleport.com/docs/enroll-resources/database-access/reference/labels/',
  },
  {
    title: 'Dynamic Database Registration',
    href: 'https://goteleport.com/docs/enroll-resources/database-access/guides/dynamic-registration/',
  },
];

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
      <Stack gap={3}>
        <Box>
          <Text fontWeight={'medium'}>Configure what to discover.</Text>
          <Text>
            Specify resource types, regions, and tag filters to control which
            resources are discovered.
          </Text>
        </Box>
        <Box>
          <Text fontWeight={'medium'}>Use the generated Terraform module.</Text>
          <Text>
            The Terraform module will set up an OIDC connection in AWS and
            configure Teleport discovery to scan for your resources.
          </Text>
        </Box>
        <Box>
          <Text fontWeight={'medium'}>
            Your cloud resources automatically appear in your Teleport cluster.
          </Text>
          <Text>
            Teleport scans every 30 minutes to find matching resources.
            Resources are enrolled in Teleport and ready for secure access.
          </Text>
        </Box>
      </Stack>

      <InfoTitle>Database Service</InfoTitle>
      <Stack gap={3}>
        <Text>
          Database discovery enrolls databases by creating db resources in the
          Teleport cluster. It assigns labels from AWS tags and cloud metadata,
          such as the database's VPC ID. Database Service instances use label
          selectors to determine which enrolled databases to proxy.
        </Text>
        <Text>
          For IAM-authenticated RDS databases, the Database Service must be able
          to reach both the Teleport cluster and RDS endpoints and use AWS
          credentials with rds-db:connect permission.
        </Text>
        <Text>
          To provide this network connectivity, we recommend deploying a
          Database Service instance in each AWS VPC containing databases that it
          will proxy.
        </Text>
      </Stack>

      <Box mt={2}>
        <Links links={dbRefLinks} />
      </Box>

      <ReferenceLinks links={referenceLinks} />
    </Box>
  );
}
